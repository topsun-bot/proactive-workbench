#import "NotificationGate.h"
#import <stdio.h>

static int g_fails;

static void expect(const char *name, int cond) {
  if (!cond) {
    fprintf(stderr, "FAIL %s\n", name);
    g_fails++;
    return;
  }
  printf("PASS %s\n", name);
}

int main(void) {
  SuggestionProbe *missing = [NotificationGate probeFromJSONObject:@{@"Title" : @"Walk", @"Body" : @"hi"}];
  expect("JSON missing propose → present=NO", missing.proposePresent == NO);
  expect("JSON missing propose → propose=NO", missing.propose == NO);
  expect("missing propose + Focus off → hide",
         [NotificationGate shouldShowBannerProposePresent:missing.proposePresent
                                                 propose:missing.propose
                                                 focusOn:NO] == NO);

  SuggestionProbe *no = [NotificationGate probeFromJSONObject:@{@"propose" : @NO, @"Title" : @"x"}];
  expect("JSON propose=false → hide",
         [NotificationGate shouldShowBannerProposePresent:no.proposePresent propose:no.propose focusOn:NO] == NO);

  SuggestionProbe *yes = [NotificationGate probeFromJSONObject:@{@"Propose" : @YES, @"Title" : @"x"}];
  expect("JSON Propose=true + Focus on → hide",
         [NotificationGate shouldShowBannerProposePresent:yes.proposePresent propose:yes.propose focusOn:YES] == NO);
  expect("JSON Propose=true + Focus off → show",
         [NotificationGate shouldShowBannerProposePresent:yes.proposePresent propose:yes.propose focusOn:NO] == YES);

  if (g_fails != 0) {
    fprintf(stderr, "%d JSON propose parse failure(s)\n", g_fails);
    return 1;
  }
  printf("all propose JSON parse tests passed\n");
  return 0;
}
