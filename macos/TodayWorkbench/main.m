#import <Cocoa/Cocoa.h>
#import <WebKit/WebKit.h>

#import "CalendarSource.h"
#import "NotificationGate.h"

@interface AppDelegate : NSObject <NSApplicationDelegate, WKNavigationDelegate>
@property(nonatomic, strong) NSWindow *window;
@property(nonatomic, strong) WKWebView *webView;
@property(nonatomic, strong) NSTask *server;
@property(nonatomic, strong) CalendarSource *calendar;
@property(nonatomic, strong) NotificationGate *gate;
@property(nonatomic, strong) NSURL *serverURL;
@property(nonatomic, strong) CalendarSourceSnapshot *calendarSnap;
@end

@implementation AppDelegate

- (NSString *)bundledWorkbench {
  NSString *dir = [[NSBundle mainBundle] bundlePath];
  NSString *macOS = [dir stringByAppendingPathComponent:@"Contents/MacOS"];
  return [macOS stringByAppendingPathComponent:@"workbench"];
}

- (NSString *)portFilePath {
  NSString *home = NSHomeDirectory();
  return [home stringByAppendingPathComponent:@"Library/Application Support/Today Workbench/port"];
}

- (NSString *)urlStringFromPortFile {
  NSString *body = [NSString stringWithContentsOfFile:[self portFilePath]
                                            encoding:NSUTF8StringEncoding
                                               error:nil];
  NSString *addr = [body
      stringByTrimmingCharactersInSet:[NSCharacterSet whitespaceAndNewlineCharacterSet]];
  if (addr.length == 0) {
    return nil;
  }
  if ([addr hasPrefix:@"http://"] || [addr hasPrefix:@"https://"]) {
    return addr;
  }
  return [NSString stringWithFormat:@"http://%@/", addr];
}

- (BOOL)debugFixtureEnabled {
  NSString *raw = [[[NSProcessInfo processInfo] environment] objectForKey:@"PW_DEBUG_FIXTURE"];
  if (raw.length == 0) {
    return NO;
  }
  NSString *v = [raw lowercaseString];
  return !([v isEqualToString:@"0"] || [v isEqualToString:@"false"] ||
           [v isEqualToString:@"off"] || [v isEqualToString:@"no"]);
}

- (NSArray<NSString *> *)serverArguments {
  NSMutableArray<NSString *> *args = [NSMutableArray arrayWithObjects:@"serve", @"--addr=127.0.0.1:8741", nil];
  if (![self debugFixtureEnabled]) {
    return args;
  }
  [args addObject:@"--debug-fixture"];
  NSDictionary *env = [[NSProcessInfo processInfo] environment];
  NSString *now = env[@"PW_DEBUG_NOW"];
  NSString *wx = env[@"PW_DEBUG_WEATHER"];
  if (now.length > 0) {
    [args addObject:[NSString stringWithFormat:@"--now=%@", now]];
  }
  if (wx.length > 0) {
    [args addObject:[NSString stringWithFormat:@"--weather=%@", wx]];
  }
  return args;
}

- (NSString *)waitForServerURL:(NSPipe *)outPipe {
  NSFileHandle *fh = [outPipe fileHandleForReading];
  NSMutableData *buf = [NSMutableData data];
  NSDate *deadline = [NSDate dateWithTimeIntervalSinceNow:4.0];
  while ([deadline timeIntervalSinceNow] > 0) {
    NSData *chunk = [fh availableData];
    if (chunk.length > 0) {
      [buf appendData:chunk];
      NSString *s = [[NSString alloc] initWithData:buf encoding:NSUTF8StringEncoding];
      NSRange nl = [s rangeOfString:@"\n"];
      if (nl.location != NSNotFound) {
        NSString *line = [[s substringToIndex:nl.location]
            stringByTrimmingCharactersInSet:[NSCharacterSet whitespaceAndNewlineCharacterSet]];
        if (line.length > 0) {
          return line;
        }
      }
    }
    NSString *fromFile = [self urlStringFromPortFile];
    if (fromFile.length > 0) {
      return fromFile;
    }
    [NSThread sleepForTimeInterval:0.05];
  }
  return [self urlStringFromPortFile];
}

- (NSURL *)startServer {
  NSString *bin = [self bundledWorkbench];
  if (![[NSFileManager defaultManager] isExecutableFileAtPath:bin]) {
    NSLog(@"workbench missing at %@", bin);
    return nil;
  }
  NSPipe *out = [NSPipe pipe];
  NSTask *task = [[NSTask alloc] init];
  task.launchPath = bin;
  task.arguments = [self serverArguments];
  task.standardOutput = out;
  task.standardError = [NSPipe pipe];
  @try {
    [task launch];
  } @catch (NSException *ex) {
    NSLog(@"failed to launch workbench: %@", ex);
    return nil;
  }
  self.server = task;
  NSString *line = [self waitForServerURL:out];
  if (line.length == 0) {
    return nil;
  }
  if (![line hasSuffix:@"/"]) {
    line = [line stringByAppendingString:@"/"];
  }
  return [NSURL URLWithString:line];
}

- (NSString *)jsString:(NSString *)raw {
  if (raw == nil) {
    return @"''";
  }
  NSError *err = nil;
  NSData *data = [NSJSONSerialization dataWithJSONObject:raw options:NSJSONWritingFragmentsAllowed error:&err];
  if (data == nil) {
    return @"''";
  }
  return [[NSString alloc] initWithData:data encoding:NSUTF8StringEncoding];
}

- (void)injectCalendarIntoWebView {
  if (self.webView == nil || self.calendarSnap == nil) {
    return;
  }
  CalendarSourceSnapshot *snap = self.calendarSnap;
  NSMutableString *js = [NSMutableString string];
  [js appendString:@"(function(){"];
  [js appendString:@"var tasks=document.getElementById('tasks');"];
  [js appendString:@"if(!tasks) return;"];
  [js appendString:@"var old=document.getElementById('eventkit-note');"];
  [js appendString:@"if(old) old.remove();"];
  [js appendString:@"var note=document.createElement('p');"];
  [js appendString:@"note.id='eventkit-note';"];
  [js appendString:@"note.className='date';"];
  if ([snap.auth isEqualToString:@"granted"]) {
    [js appendFormat:@"note.textContent='Calendar: EventKit (live) · %lu event(s). Location stays MOCK — no CoreLocation.';",
                     (unsigned long)snap.events.count];
    [js appendString:@"tasks.parentNode.insertBefore(note, tasks);"];
    for (CalendarSourceEvent *e in snap.events) {
      [js appendString:@"{"];
      [js appendString:@"var art=document.createElement('article'); art.className='task';"];
      [js appendFormat:@"var time=document.createElement('time'); time.textContent=%@; art.appendChild(time);",
                       [self jsString:e.start]];
      [js appendFormat:@"var h=document.createElement('h3'); h.textContent=%@; art.appendChild(h);",
                       [self jsString:e.title]];
      [js appendString:@"var k=document.createElement('div'); k.className='kind'; k.textContent='eventkit'; art.appendChild(k);"];
      [js appendString:@"tasks.appendChild(art);"];
      [js appendString:@"}"];
    }
  } else {
    [js appendString:@"note.textContent='Calendar: EventKit permission denied — MOCK schedule from the Go core.';"];
    [js appendString:@"tasks.parentNode.insertBefore(note, tasks);"];
  }
  [js appendString:@"})();"];
  [self.webView evaluateJavaScript:js completionHandler:nil];
}

- (void)webView:(WKWebView *)webView didFinishNavigation:(WKNavigation *)navigation {
  (void)webView;
  (void)navigation;
  [self injectCalendarIntoWebView];
  if (self.serverURL != nil) {
    [self.gate evaluateTodayURL:self.serverURL
                     completion:^(NSUInteger shown, NSString *log) {
                       NSLog(@"gate2 shown=%lu %@", (unsigned long)shown, log);
                     }];
  }
}

- (void)applicationDidFinishLaunching:(NSNotification *)notification {
  (void)notification;
  self.calendar = [[CalendarSource alloc] init];
  self.gate = [[NotificationGate alloc] init];

  NSRect frame = NSMakeRect(120, 80, 1040, 780);
  self.window = [[NSWindow alloc]
      initWithContentRect:frame
                styleMask:(NSWindowStyleMaskTitled | NSWindowStyleMaskClosable |
                           NSWindowStyleMaskMiniaturizable | NSWindowStyleMaskResizable)
                  backing:NSBackingStoreBuffered
                    defer:NO];
  self.window.title = @"Today";
  WKWebViewConfiguration *cfg = [[WKWebViewConfiguration alloc] init];
  self.webView = [[WKWebView alloc] initWithFrame:self.window.contentView.bounds configuration:cfg];
  self.webView.autoresizingMask = NSViewWidthSizable | NSViewHeightSizable;
  self.webView.navigationDelegate = self;
  [self.window.contentView addSubview:self.webView];

  NSURL *url = [self startServer];
  self.serverURL = url;
  if (url != nil) {
    [self.webView loadRequest:[NSURLRequest requestWithURL:url]];
  } else {
    NSString *html = @"<html><body style='font:20px Palatino;padding:40px'>Could not start the bundled Go core.</body></html>";
    [self.webView loadHTMLString:html baseURL:nil];
  }

  /* EventKit permission is requested here. Denied → empty calendar.Source + MOCK UI. */
  [self.calendar requestAccessAndLoad:^(CalendarSourceSnapshot *snapshot) {
    self.calendarSnap = snapshot;
    [self injectCalendarIntoWebView];
  }];

  [self.window makeKeyAndOrderFront:nil];
  [NSApp activateIgnoringOtherApps:YES];
}

- (void)applicationWillTerminate:(NSNotification *)notification {
  (void)notification;
  if (self.server.running) {
    [self.server terminate];
  }
}

- (BOOL)applicationShouldTerminateAfterLastWindowClosed:(NSApplication *)sender {
  (void)sender;
  return YES;
}

@end

int main(int argc, const char *argv[]) {
  (void)argc;
  (void)argv;
  @autoreleasepool {
    NSApplication *app = [NSApplication sharedApplication];
    AppDelegate *delegate = [[AppDelegate alloc] init];
    app.delegate = delegate;
    [app setActivationPolicy:NSApplicationActivationPolicyRegular];
    [app run];
  }
  return 0;
}
