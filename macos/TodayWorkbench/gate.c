#include "gate.h"

#include <string.h>

int should_show_banner(BannerInput in) {
  if (!in.propose_present) {
    return 0;
  }
  if (!in.propose) {
    return 0;
  }
  if (in.focus_on) {
    return 0;
  }
  return 1;
}

void calendar_source_after_auth(int auth, const CalendarEvent *live, int n, CalendarSnapshot *out) {
  if (out == NULL) {
    return;
  }
  memset(out, 0, sizeof(*out));
  out->auth = auth;
  if (auth != kCalAuthGranted) {
    strncpy(out->fallback, kCalFallbackDenied, sizeof(out->fallback) - 1);
    out->event_count = 0;
    return;
  }
  if (live == NULL || n <= 0) {
    out->event_count = 0;
    return;
  }
  if (n > kCalMaxEvents) {
    n = kCalMaxEvents;
  }
  for (int i = 0; i < n; i++) {
    out->events[i] = live[i];
    if (out->events[i].source[0] == '\0') {
      strncpy(out->events[i].source, kCalSourceEventKit, sizeof(out->events[i].source) - 1);
    }
    out->events[i].is_mock = 0;
  }
  out->event_count = n;
}
