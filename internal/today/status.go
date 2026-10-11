package today

import (
	dscal "github.com/topsun-bot/proactive-workbench/internal/datasources/calendar"
)

// Explicit calendar status on the Snapshot. The client reads this field and
// never infers availability from an empty event or task list.
const (
	CalendarStatusAvailable        = "available"
	CalendarStatusUnconfigured     = "unconfigured"
	CalendarStatusPermissionDenied = "permission_denied"
	CalendarStatusUnavailable      = "unavailable"
)

// CalendarUserPermissionDenied is shown when EventKit is denied and there is no ICS.
const CalendarUserPermissionDenied = "日历权限被拒绝"

// CalendarStatusFromPerception maps A4 ICS perception onto CalendarStatus.
func CalendarStatusFromPerception(calErr, calUser string) string {
	if calErr == "" {
		return CalendarStatusAvailable
	}
	if calUser == dscal.UserFacingUnconfigured {
		return CalendarStatusUnconfigured
	}
	return CalendarStatusUnavailable
}

// ApplyEventKitStatus overlays Mac EventKit auth onto a Snapshot.
// Granted hides 日历未配置 even when the event list is empty (a real empty day).
// Denied + no ICS becomes permission_denied, not unconfigured.
// eventCount is ignored — status is never inferred from an empty list.
func ApplyEventKitStatus(s Snapshot, auth string, eventCount int) Snapshot {
	_ = eventCount
	switch auth {
	case "granted":
		s.CalendarStatus = CalendarStatusAvailable
		s.CalendarAvailable = true
		s.CalendarUserMessage = ""
		s.CalendarError = ""
	case "denied", "unknown":
		if s.CalendarStatus == CalendarStatusUnconfigured ||
			s.CalendarStatus == "" ||
			!s.CalendarAvailable {
			s.CalendarStatus = CalendarStatusPermissionDenied
			s.CalendarAvailable = false
			s.CalendarUserMessage = CalendarUserPermissionDenied
			s.CalendarError = "calendar_permission_denied"
		}
	default:
	}
	s.Briefing = briefingLine(s, LongTerm{SourceLabel: s.MemorySource, SleepNote: s.SleepLine, SleepHours: 0, People: s.People}, s.WeatherMock, s.WeatherAvailable)
	s.Signals = rewriteCalendarSignal(s)
	return s
}

func rewriteCalendarSignal(s Snapshot) []Signal {
	calValue := s.CalendarUserMessage
	if calValue == "" {
		calValue = s.CalendarStatus
	}
	out := make([]Signal, 0, len(s.Signals))
	replaced := false
	for _, sig := range s.Signals {
		if sig.Label == "Calendar" {
			sig.Value = calValue
			replaced = true
		}
		out = append(out, sig)
	}
	if !replaced {
		out = append(out, Signal{Label: "Calendar", Value: calValue})
	}
	return out
}
