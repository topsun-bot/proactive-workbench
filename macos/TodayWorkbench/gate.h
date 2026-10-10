#ifndef TODAY_WORKBENCH_GATE_H
#define TODAY_WORKBENCH_GATE_H

/* Pure logic for Mac gate 2 and EventKit → calendar.Source fallback.
 * No Cocoa. Compiled on Linux CI and macOS CI.
 */

#ifdef __cplusplus
extern "C" {
#endif

enum {
  kProposeMissing = 0,
  kProposePresent = 1
};

typedef struct {
  int propose_present; /* 0 = field missing → treat as false */
  int propose;         /* 0/1 when present */
  int focus_on;        /* 1 = Focus / Do Not Disturb is on */
} BannerInput;

/* Banner only when propose == true AND Focus is off. */
int should_show_banner(BannerInput in);

enum {
  kCalAuthUnknown = 0,
  kCalAuthGranted = 1,
  kCalAuthDenied = 2
};

#define kCalFallbackDenied "calendar_permission_denied"
#define kCalSourceEventKit "eventkit"
#define kCalMaxEvents 32
#define kCalFieldLen 256

typedef struct {
  char uid[kCalFieldLen];
  char title[kCalFieldLen];
  char start[64]; /* RFC3339, matches calendar.Event.Start */
  char end[64];
  char notes[kCalFieldLen];
  char location[kCalFieldLen]; /* event venue, NOT device GPS */
  int is_mock;                 /* 0 for live EventKit */
  char source[64];             /* "eventkit" */
} CalendarEvent;

typedef struct {
  int auth; /* kCalAuth* */
  char fallback[64];
  CalendarEvent events[kCalMaxEvents];
  int event_count;
} CalendarSnapshot;

/* Feed calendar.Source: denied/unknown → empty list + fallback; granted → copy live. */
void calendar_source_after_auth(int auth, const CalendarEvent *live, int n, CalendarSnapshot *out);

#ifdef __cplusplus
}
#endif

#endif
