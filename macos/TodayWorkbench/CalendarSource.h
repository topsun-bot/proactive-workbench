#import <Foundation/Foundation.h>

NS_ASSUME_NONNULL_BEGIN

/* EventKit adapter that feeds calendar.Source-shaped records.
 * Linux keeps using local ICS in the Go package. There is no EventKit stub there.
 * Device location stays MOCK — this adapter never imports CoreLocation.
 */

@interface CalendarSourceEvent : NSObject
@property(nonatomic, copy) NSString *uid;
@property(nonatomic, copy) NSString *title;
@property(nonatomic, copy) NSString *start;
@property(nonatomic, copy) NSString *end;
@property(nonatomic, copy) NSString *notes;
@property(nonatomic, copy) NSString *location;
@property(nonatomic, copy) NSString *source;
@property(nonatomic, assign) BOOL isMock;
@end

@interface CalendarSourceSnapshot : NSObject
@property(nonatomic, copy) NSString *auth;      /* granted | denied | unknown */
@property(nonatomic, copy) NSString *fallback;  /* calendar_permission_denied or empty */
@property(nonatomic, copy) NSArray<CalendarSourceEvent *> *events;
@property(nonatomic, copy) NSArray<NSDictionary *> *reminders;
- (NSDictionary *)JSONObject;
@end

typedef void (^CalendarSourceHandler)(CalendarSourceSnapshot *snapshot);

@interface CalendarSource : NSObject
- (void)requestAccessAndLoad:(CalendarSourceHandler)done;
+ (NSURL *)snapshotFileURL;
+ (BOOL)writeSnapshot:(CalendarSourceSnapshot *)snap error:(NSError **)error;
@end

NS_ASSUME_NONNULL_END
