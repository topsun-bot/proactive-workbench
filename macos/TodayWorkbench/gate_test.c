#include "gate.h"

#include <stdio.h>
#include <string.h>

static int g_fails;

static void expect_int(const char *name, int got, int want) {
  if (got != want) {
    fprintf(stderr, "FAIL %s: got %d want %d\n", name, got, want);
    g_fails++;
    return;
  }
  printf("PASS %s => %d\n", name, got);
}

static void expect_str(const char *name, const char *got, const char *want) {
  if (strcmp(got ? got : "", want) != 0) {
    fprintf(stderr, "FAIL %s: got %s want %s\n", name, got ? got : "(null)", want);
    g_fails++;
    return;
  }
  printf("PASS %s => %s\n", name, got);
}

int main(void) {
  BannerInput missing = {.propose_present = kProposeMissing, .propose = 1, .focus_on = 0};
  expect_int("gate: missing propose, Focus off → hide", should_show_banner(missing), 0);

  BannerInput no = {.propose_present = kProposePresent, .propose = 0, .focus_on = 0};
  expect_int("gate: propose=false, Focus off → hide", should_show_banner(no), 0);

  BannerInput focus = {.propose_present = kProposePresent, .propose = 1, .focus_on = 1};
  expect_int("gate: propose=true, Focus on → hide", should_show_banner(focus), 0);

  BannerInput yes = {.propose_present = kProposePresent, .propose = 1, .focus_on = 0};
  expect_int("gate: propose=true, Focus off → show", should_show_banner(yes), 1);

  CalendarEvent live;
  memset(&live, 0, sizeof(live));
  strncpy(live.uid, "ek-1", sizeof(live.uid) - 1);
  strncpy(live.title, "Standup", sizeof(live.title) - 1);
  strncpy(live.start, "2026-10-10T10:00:00+08:00", sizeof(live.start) - 1);
  strncpy(live.end, "2026-10-10T10:30:00+08:00", sizeof(live.end) - 1);

  CalendarSnapshot denied;
  calendar_source_after_auth(kCalAuthDenied, &live, 1, &denied);
  expect_int("eventkit denied: event_count", denied.event_count, 0);
  expect_int("eventkit denied: auth", denied.auth, kCalAuthDenied);
  expect_str("eventkit denied: fallback", denied.fallback, kCalFallbackDenied);

  CalendarSnapshot unknown;
  calendar_source_after_auth(kCalAuthUnknown, &live, 1, &unknown);
  expect_int("eventkit unknown: event_count", unknown.event_count, 0);
  expect_str("eventkit unknown: fallback", unknown.fallback, kCalFallbackDenied);

  CalendarSnapshot granted;
  calendar_source_after_auth(kCalAuthGranted, &live, 1, &granted);
  expect_int("eventkit granted: event_count", granted.event_count, 1);
  expect_int("eventkit granted: is_mock", granted.events[0].is_mock, 0);
  expect_str("eventkit granted: source", granted.events[0].source, kCalSourceEventKit);
  expect_str("eventkit granted: title", granted.events[0].title, "Standup");
  expect_str("eventkit granted: fallback empty", granted.fallback, "");

  if (g_fails != 0) {
    fprintf(stderr, "%d failure(s)\n", g_fails);
    return 1;
  }
  printf("all macos gate + EventKit-denied tests passed\n");
  return 0;
}
