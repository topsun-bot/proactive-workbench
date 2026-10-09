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

- (NSURL *)startServer {
  NSString *bin = [self bundledWorkbench];
  if (![[NSFileManager defaultManager] isExecutableFileAtPath:bin]) {
    NSLog(@"workbench missing at %@", bin);
    return nil;
  }
  NSPipe *out = [NSPipe pipe];
  NSTask *task = [[NSTask alloc] init];
  task.launchPath = bin;
  task.arguments = @[
    @"serve",
    @"--addr=127.0.0.1:0",
    @"--tz=Asia/Shanghai",
    @"--now=2026-10-10T07:15:00+08:00",
    @"--weather=clear"
  ];
  task.standardOutput = out;
  task.standardError = [NSPipe pipe];
  @try {
    [task launch];
  } @catch (NSException *ex) {
    NSLog(@"failed to launch workbench: %@", ex);
    return nil;
  }
  self.server = task;
  NSData *data = [[out fileHandleForReading] availableData];
  NSString *line = [[[NSString alloc] initWithData:data encoding:NSUTF8StringEncoding]
      stringByTrimmingCharactersInSet:[NSCharacterSet whitespaceAndNewlineCharacterSet]];
  if (line.length == 0) {
    return nil;
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
