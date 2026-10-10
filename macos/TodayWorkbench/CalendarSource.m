#import "CalendarSource.h"
#import "gate.h"

#import <EventKit/EventKit.h>

@implementation CalendarSourceEvent
@end

@implementation CalendarSourceSnapshot
- (NSDictionary *)JSONObject {
  NSMutableArray *events = [NSMutableArray array];
  for (CalendarSourceEvent *e in self.events) {
    [events addObject:@{
      @"UID" : e.uid ?: @"",
      @"Title" : e.title ?: @"",
      @"Start" : e.start ?: @"",
      @"End" : e.end ?: @"",
      @"Notes" : e.notes ?: @"",
      @"Location" : e.location ?: @"",
      @"IsMock" : @(e.isMock),
      @"Source" : e.source ?: @"eventkit",
    }];
  }
  return @{
    @"auth" : self.auth ?: @"unknown",
    @"fallback" : self.fallback ?: @"",
    @"source" : @"eventkit",
    @"events" : events,
    @"reminders" : self.reminders ?: @[],
  };
}
@end

@implementation CalendarSource {
  EKEventStore *_store;
}

- (instancetype)init {
  self = [super init];
  if (self) {
    _store = [[EKEventStore alloc] init];
  }
  return self;
}

+ (NSURL *)snapshotFileURL {
  NSArray<NSURL *> *urls = [[NSFileManager defaultManager] URLsForDirectory:NSApplicationSupportDirectory
                                                                  inDomains:NSUserDomainMask];
  NSURL *dir = [urls.firstObject URLByAppendingPathComponent:@"Today Workbench" isDirectory:YES];
  return [dir URLByAppendingPathComponent:@"calendar-source.json"];
}

+ (BOOL)writeSnapshot:(CalendarSourceSnapshot *)snap error:(NSError **)error {
  NSURL *url = [self snapshotFileURL];
  [[NSFileManager defaultManager] createDirectoryAtURL:url.URLByDeletingLastPathComponent
                           withIntermediateDirectories:YES
                                            attributes:nil
                                                 error:nil];
  NSData *data = [NSJSONSerialization dataWithJSONObject:[snap JSONObject] options:NSJSONWritingPrettyPrinted error:error];
  if (data == nil) {
    return NO;
  }
  return [data writeToURL:url options:NSDataWritingAtomic error:error];
}

static NSString *rfc3339(NSDate *date) {
  if (date == nil) {
    return @"";
  }
  static NSISO8601DateFormatter *fmt;
  static dispatch_once_t once;
  dispatch_once(&once, ^{
    fmt = [[NSISO8601DateFormatter alloc] init];
    fmt.formatOptions = NSISO8601DateFormatWithInternetDateTime;
  });
  return [fmt stringFromDate:date] ?: @"";
}

- (CalendarSourceSnapshot *)snapshotFromAuth:(int)auth liveEvents:(NSArray<EKEvent *> *)ekEvents {
  CalendarEvent live[kCalMaxEvents];
  memset(live, 0, sizeof(live));
  int n = 0;
  for (EKEvent *ev in ekEvents) {
    if (n >= kCalMaxEvents) {
      break;
    }
    CalendarEvent *dst = &live[n++];
    NSString *uid = ev.eventIdentifier ?: @"";
    NSString *title = ev.title ?: @"";
    NSString *start = rfc3339(ev.startDate);
    NSString *end = rfc3339(ev.endDate);
    NSString *notes = ev.notes ?: @"";
    NSString *place = ev.location ?: @""; /* event venue, not CoreLocation */
    strncpy(dst->uid, uid.UTF8String, sizeof(dst->uid) - 1);
    strncpy(dst->title, title.UTF8String, sizeof(dst->title) - 1);
    strncpy(dst->start, start.UTF8String, sizeof(dst->start) - 1);
    strncpy(dst->end, end.UTF8String, sizeof(dst->end) - 1);
    strncpy(dst->notes, notes.UTF8String, sizeof(dst->notes) - 1);
    strncpy(dst->location, place.UTF8String, sizeof(dst->location) - 1);
    strncpy(dst->source, kCalSourceEventKit, sizeof(dst->source) - 1);
  }
  CalendarSnapshot raw;
  calendar_source_after_auth(auth, live, n, &raw);

  CalendarSourceSnapshot *snap = [[CalendarSourceSnapshot alloc] init];
  if (raw.auth == kCalAuthGranted) {
    snap.auth = @"granted";
  } else if (raw.auth == kCalAuthDenied) {
    snap.auth = @"denied";
  } else {
    snap.auth = @"unknown";
  }
  snap.fallback = raw.fallback[0] ? @(raw.fallback) : @"";
  NSMutableArray<CalendarSourceEvent *> *out = [NSMutableArray array];
  for (int i = 0; i < raw.event_count; i++) {
    CalendarSourceEvent *e = [[CalendarSourceEvent alloc] init];
    e.uid = @(raw.events[i].uid);
    e.title = @(raw.events[i].title);
    e.start = @(raw.events[i].start);
    e.end = @(raw.events[i].end);
    e.notes = @(raw.events[i].notes);
    e.location = @(raw.events[i].location);
    e.source = @(raw.events[i].source);
    e.isMock = raw.events[i].is_mock ? YES : NO;
    [out addObject:e];
  }
  snap.events = out;
  snap.reminders = @[];
  return snap;
}

- (NSArray<EKEvent *> *)eventsNext24Hours {
  NSDate *from = [NSDate date];
  NSDate *to = [from dateByAddingTimeInterval:24 * 60 * 60];
  NSPredicate *pred = [_store predicateForEventsWithStartDate:from endDate:to calendars:nil];
  return [_store eventsMatchingPredicate:pred] ?: @[];
}

- (void)finishWithGranted:(BOOL)granted error:(NSError *)error handler:(CalendarSourceHandler)done {
  int auth = granted ? kCalAuthGranted : kCalAuthDenied;
  NSArray<EKEvent *> *live = granted ? [self eventsNext24Hours] : @[];
  if (error != nil && !granted) {
    NSLog(@"EventKit permission denied or failed: %@", error);
  }
  CalendarSourceSnapshot *snap = [self snapshotFromAuth:auth liveEvents:live];
  NSError *writeErr = nil;
  if (![CalendarSource writeSnapshot:snap error:&writeErr]) {
    NSLog(@"EventKit calendar-source.json write failed: %@", writeErr);
  } else {
    NSLog(@"EventKit calendar.Source auth=%@ fallback=%@ events=%lu file=%@",
          snap.auth, snap.fallback, (unsigned long)snap.events.count, [CalendarSource snapshotFileURL].path);
  }
  if (done) {
    done(snap);
  }
}

- (void)requestAccessAndLoad:(CalendarSourceHandler)done {
  CalendarSourceHandler cb = done;
#if defined(__MAC_14_0)
  if (@available(macOS 14.0, *)) {
    [_store requestFullAccessToEventsWithCompletion:^(BOOL granted, NSError *error) {
      dispatch_async(dispatch_get_main_queue(), ^{
        [self finishWithGranted:granted error:error handler:cb];
      });
    }];
    return;
  }
#endif
  [_store requestAccessToEntityType:EKEntityTypeEvent
                         completion:^(BOOL granted, NSError *error) {
                           dispatch_async(dispatch_get_main_queue(), ^{
                             [self finishWithGranted:granted error:error handler:cb];
                           });
                         }];
}

@end
