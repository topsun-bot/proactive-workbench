#import <Foundation/Foundation.h>

NS_ASSUME_NONNULL_BEGIN

@interface SuggestionProbe : NSObject
@property(nonatomic, copy) NSString *title;
@property(nonatomic, copy) NSString *body;
@property(nonatomic, assign) BOOL proposePresent;
@property(nonatomic, assign) BOOL propose;
@end

@interface NotificationGate : NSObject
+ (SuggestionProbe *)probeFromJSONObject:(id)obj;
+ (NSArray<SuggestionProbe *> *)probesFromTodayPayload:(NSDictionary *)payload;
+ (BOOL)shouldShowBannerProposePresent:(BOOL)present propose:(BOOL)propose focusOn:(BOOL)focusOn;
- (void)evaluateTodayURL:(NSURL *)todayURL completion:(void (^)(NSUInteger shown, NSString *log))done;
@end

NS_ASSUME_NONNULL_END
