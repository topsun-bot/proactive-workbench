#import "NotificationGate.h"
#import "gate.h"

#import <Intents/Intents.h>
#import <UserNotifications/UserNotifications.h>

@implementation SuggestionProbe
@end

@implementation NotificationGate

+ (SuggestionProbe *)probeFromJSONObject:(id)obj {
  SuggestionProbe *p = [[SuggestionProbe alloc] init];
  if (![obj isKindOfClass:[NSDictionary class]]) {
    return p;
  }
  NSDictionary *d = obj;
  p.title = [d[@"Title"] isKindOfClass:[NSString class]] ? d[@"Title"] : d[@"title"];
  p.body = [d[@"Body"] isKindOfClass:[NSString class]] ? d[@"Body"] : d[@"body"];
  if (p.title == nil) {
    p.title = @"";
  }
  if (p.body == nil) {
    p.body = @"";
  }
  id raw = d[@"propose"];
  if (raw == nil) {
    raw = d[@"Propose"];
  }
  if (raw == nil || raw == [NSNull null]) {
    p.proposePresent = NO;
    p.propose = NO;
  } else {
    p.proposePresent = YES;
    p.propose = [raw respondsToSelector:@selector(boolValue)] ? [raw boolValue] : NO;
  }
  return p;
}

+ (NSArray<SuggestionProbe *> *)probesFromTodayPayload:(NSDictionary *)payload {
  id list = payload[@"Suggestions"];
  if (list == nil) {
    list = payload[@"suggestions"];
  }
  if (![list isKindOfClass:[NSArray class]]) {
    return @[];
  }
  NSMutableArray<SuggestionProbe *> *out = [NSMutableArray array];
  for (id item in list) {
    [out addObject:[self probeFromJSONObject:item]];
  }
  return out;
}

+ (BOOL)shouldShowBannerProposePresent:(BOOL)present propose:(BOOL)propose focusOn:(BOOL)focusOn {
  BannerInput in = {
      .propose_present = present ? kProposePresent : kProposeMissing,
      .propose = propose ? 1 : 0,
      .focus_on = focusOn ? 1 : 0,
  };
  return should_show_banner(in) ? YES : NO;
}

- (BOOL)focusIsOn {
  if (@available(macOS 12.0, *)) {
    INFocusStatus *status = [INFocusStatusCenter defaultCenter].focusStatus;
    if (status == nil) {
      /* Unknown Focus: fail closed so we never banner through DND. */
      return YES;
    }
    return status.isFocused;
  }
  return NO;
}

- (void)requestFocusAccess:(void (^)(void))done {
  if (@available(macOS 12.0, *)) {
    [[INFocusStatusCenter defaultCenter] requestAuthorizationWithCompletionHandler:^(INFocusStatusAuthorizationStatus status) {
      (void)status;
      dispatch_async(dispatch_get_main_queue(), done);
    }];
    return;
  }
  done();
}

- (void)postBanner:(SuggestionProbe *)probe {
  UNUserNotificationCenter *center = [UNUserNotificationCenter currentNotificationCenter];
  [center requestAuthorizationWithOptions:(UNAuthorizationOptionAlert | UNAuthorizationOptionSound)
                        completionHandler:^(BOOL granted, NSError *error) {
                          if (!granted) {
                            NSLog(@"notification authorization denied: %@", error);
                            return;
                          }
                          UNMutableNotificationContent *content = [[UNMutableNotificationContent alloc] init];
                          content.title = probe.title.length ? probe.title : @"Today";
                          content.body = probe.body.length ? probe.body : @"";
                          NSString *ident = [NSString stringWithFormat:@"today-%@", [[NSUUID UUID] UUIDString]];
                          UNNotificationRequest *req = [UNNotificationRequest requestWithIdentifier:ident
                                                                                            content:content
                                                                                            trigger:nil];
                          [center addNotificationRequest:req
                                   withCompletionHandler:^(NSError *addErr) {
                                     if (addErr != nil) {
                                       NSLog(@"banner failed: %@", addErr);
                                     }
                                   }];
                        }];
}

- (void)evaluateTodayURL:(NSURL *)todayURL completion:(void (^)(NSUInteger shown, NSString *log))done {
  NSURL *api = [todayURL URLByAppendingPathComponent:@"api/today"];
  [self requestFocusAccess:^{
    BOOL focusOn = [self focusIsOn];
    NSURLSessionDataTask *task = [[NSURLSession sharedSession]
        dataTaskWithURL:api
      completionHandler:^(NSData *data, NSURLResponse *response, NSError *error) {
        NSMutableString *log = [NSMutableString string];
        [log appendFormat:@"focus_on=%d ", focusOn ? 1 : 0];
        if (error != nil || data == nil) {
          [log appendFormat:@"fetch_error=%@", error];
          NSLog(@"%@", log);
          dispatch_async(dispatch_get_main_queue(), ^{
            if (done) {
              done(0, log);
            }
          });
          return;
        }
        (void)response;
        NSDictionary *payload = [NSJSONSerialization JSONObjectWithData:data options:0 error:nil];
        if (![payload isKindOfClass:[NSDictionary class]]) {
          [log appendString:@"bad_json"];
          dispatch_async(dispatch_get_main_queue(), ^{
            if (done) {
              done(0, log);
            }
          });
          return;
        }
        NSArray<SuggestionProbe *> *probes = [NotificationGate probesFromTodayPayload:payload];
        NSUInteger shown = 0;
        for (SuggestionProbe *p in probes) {
          BOOL show = [NotificationGate shouldShowBannerProposePresent:p.proposePresent
                                                              propose:p.propose
                                                              focusOn:focusOn];
          [log appendFormat:@"[propose_present=%d propose=%d → %@] ",
                            p.proposePresent ? 1 : 0, p.propose ? 1 : 0, show ? @"show" : @"hide"];
          if (show) {
            shown += 1;
            dispatch_async(dispatch_get_main_queue(), ^{
              [self postBanner:p];
            });
          }
        }
        [log appendFormat:@"shown=%lu", (unsigned long)shown];
        NSLog(@"notification gate 2: %@", log);
        dispatch_async(dispatch_get_main_queue(), ^{
          if (done) {
            done(shown, log);
          }
        });
      }];
    [task resume];
  }];
}

@end
