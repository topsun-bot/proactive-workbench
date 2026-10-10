#import <Cocoa/Cocoa.h>
#import <WebKit/WebKit.h>

@interface AppDelegate : NSObject <NSApplicationDelegate>
@property(nonatomic, strong) NSWindow *window;
@property(nonatomic, strong) WKWebView *webView;
@property(nonatomic, strong) NSTask *server;
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
  // Explicit debug switch only — never the default launch path.
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

- (void)applicationDidFinishLaunching:(NSNotification *)notification {
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
  [self.window.contentView addSubview:self.webView];

  NSURL *url = [self startServer];
  if (url != nil) {
    [self.webView loadRequest:[NSURLRequest requestWithURL:url]];
  } else {
    NSString *html = @"<html><body style='font:20px Palatino;padding:40px'>Could not start the bundled Go core.</body></html>";
    [self.webView loadHTMLString:html baseURL:nil];
  }
  [self.window makeKeyAndOrderFront:nil];
  [NSApp activateIgnoringOtherApps:YES];
}

- (void)applicationWillTerminate:(NSNotification *)notification {
  if (self.server.running) {
    [self.server terminate];
  }
}

- (BOOL)applicationShouldTerminateAfterLastWindowClosed:(NSApplication *)sender {
  return YES;
}

@end

int main(int argc, const char *argv[]) {
  @autoreleasepool {
    NSApplication *app = [NSApplication sharedApplication];
    AppDelegate *delegate = [[AppDelegate alloc] init];
    app.delegate = delegate;
    [app setActivationPolicy:NSApplicationActivationPolicyRegular];
    [app run];
  }
  return 0;
}
