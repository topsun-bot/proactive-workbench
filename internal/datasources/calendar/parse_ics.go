package calendar

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

type contentLine struct {
	Name   string
	Params map[string]string
	Value  string
}

// ParseICS reads a minimal RFC 5545 subset: VEVENT + VALARM.
// Unknown properties are ignored. This is for local files / vdirsyncer caches.
func ParseICS(data []byte, source string, isMock bool, fallback *time.Location) ([]Event, []Reminder, error) {
	if fallback == nil {
		fallback = time.UTC
	}
	if source == "" {
		source = "ics"
	}
	lines, err := unfoldICS(string(data))
	if err != nil {
		return nil, nil, err
	}

	var events []Event
	var reminders []Reminder
	var ev *Event
	var alarmAction, alarmDesc, alarmTrigger string
	inEvent, inAlarm := false, false

	flushAlarm := func() {
		if ev == nil || alarmTrigger == "" {
			alarmAction, alarmDesc, alarmTrigger = "", "", ""
			return
		}
		when, err := triggerAt(ev.Start, alarmTrigger)
		if err != nil {
			alarmAction, alarmDesc, alarmTrigger = "", "", ""
			return
		}
		reminders = append(reminders, Reminder{
			EventUID:    ev.UID,
			Description: alarmDesc,
			TriggerAt:   when,
			Action:      alarmAction,
			IsMock:      isMock,
			Source:      source,
		})
		alarmAction, alarmDesc, alarmTrigger = "", "", ""
	}

	for _, raw := range lines {
		cl, err := parseContentLine(raw)
		if err != nil {
			return nil, nil, err
		}
		switch {
		case cl.Name == "BEGIN" && cl.Value == "VEVENT":
			inEvent = true
			ev = &Event{IsMock: isMock, Source: source}
		case cl.Name == "END" && cl.Value == "VEVENT":
			if ev != nil {
				if ev.UID == "" {
					ev.UID = fmt.Sprintf("anon-%d", len(events)+1)
				}
				events = append(events, *ev)
			}
			inEvent = false
			ev = nil
			inAlarm = false
		case cl.Name == "BEGIN" && cl.Value == "VALARM":
			inAlarm = true
			alarmAction, alarmDesc, alarmTrigger = "", "", ""
		case cl.Name == "END" && cl.Value == "VALARM":
			flushAlarm()
			inAlarm = false
		case inAlarm:
			switch cl.Name {
			case "ACTION":
				alarmAction = cl.Value
			case "DESCRIPTION":
				alarmDesc = unescapeICS(cl.Value)
			case "TRIGGER":
				alarmTrigger = cl.Value
			}
		case inEvent && ev != nil:
			switch cl.Name {
			case "UID":
				ev.UID = cl.Value
			case "SUMMARY":
				ev.Title = unescapeICS(cl.Value)
			case "DESCRIPTION":
				ev.Notes = unescapeICS(cl.Value)
			case "LOCATION":
				ev.Location = unescapeICS(cl.Value)
			case "DTSTART":
				t, err := parseICSTime(cl, fallback)
				if err != nil {
					return nil, nil, fmt.Errorf("calendar: DTSTART: %w", err)
				}
				ev.Start = t
			case "DTEND":
				t, err := parseICSTime(cl, fallback)
				if err != nil {
					return nil, nil, fmt.Errorf("calendar: DTEND: %w", err)
				}
				ev.End = t
			}
		}
	}
	return events, reminders, nil
}

func unfoldICS(raw string) ([]string, error) {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	raw = strings.ReplaceAll(raw, "\r", "\n")
	parts := strings.Split(raw, "\n")
	var out []string
	for _, line := range parts {
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			if len(out) == 0 {
				return nil, fmt.Errorf("calendar: folded ICS line with no predecessor")
			}
			out[len(out)-1] += strings.TrimLeft(line, " \t")
			continue
		}
		out = append(out, line)
	}
	return out, nil
}

func parseContentLine(raw string) (contentLine, error) {
	colon := strings.IndexByte(raw, ':')
	if colon < 0 {
		return contentLine{}, fmt.Errorf("calendar: ICS line missing colon: %q", raw)
	}
	head, value := raw[:colon], raw[colon+1:]
	name, params := head, map[string]string{}
	if semi := strings.IndexByte(head, ';'); semi >= 0 {
		name = head[:semi]
		for _, part := range strings.Split(head[semi+1:], ";") {
			if part == "" {
				continue
			}
			eq := strings.IndexByte(part, '=')
			if eq < 0 {
				params[strings.ToUpper(part)] = ""
				continue
			}
			params[strings.ToUpper(part[:eq])] = strings.Trim(part[eq+1:], `"`)
		}
	}
	return contentLine{Name: strings.ToUpper(name), Params: params, Value: value}, nil
}

func parseICSTime(cl contentLine, fallback *time.Location) (time.Time, error) {
	loc := fallback
	if tzid, ok := cl.Params["TZID"]; ok && tzid != "" {
		if loaded, err := time.LoadLocation(tzid); err == nil {
			loc = loaded
		}
	}
	v := cl.Value
	if strings.HasSuffix(v, "Z") {
		t, err := time.Parse("20060102T150405Z", v)
		if err != nil {
			t, err = time.Parse("20060102T1504Z", v)
		}
		return t, err
	}
	if cl.Params["VALUE"] == "DATE" || (len(v) == 8 && !strings.Contains(v, "T")) {
		return time.ParseInLocation("20060102", v, loc)
	}
	if t, err := time.ParseInLocation("20060102T150405", v, loc); err == nil {
		return t, nil
	}
	return time.ParseInLocation("20060102T1504", v, loc)
}

func unescapeICS(s string) string {
	s = strings.ReplaceAll(s, "\\n", "\n")
	s = strings.ReplaceAll(s, "\\N", "\n")
	s = strings.ReplaceAll(s, "\\,", ",")
	s = strings.ReplaceAll(s, "\\;", ";")
	s = strings.ReplaceAll(s, "\\\\", "\\")
	return s
}

func triggerAt(start time.Time, trigger string) (time.Time, error) {
	if start.IsZero() {
		return time.Time{}, fmt.Errorf("alarm trigger without DTSTART")
	}
	neg := strings.HasPrefix(trigger, "-")
	body := strings.TrimPrefix(strings.TrimPrefix(trigger, "-"), "+")
	if !strings.HasPrefix(body, "P") {
		return time.Time{}, fmt.Errorf("unsupported TRIGGER %q", trigger)
	}
	body = strings.TrimPrefix(body, "P")
	var dur time.Duration
	if strings.HasPrefix(body, "T") || strings.Contains(body, "T") {
		tpart := body
		if i := strings.IndexByte(body, 'T'); i >= 0 {
			tpart = body[i+1:]
		}
		n := 0
		for _, r := range tpart {
			switch {
			case r >= '0' && r <= '9':
				n = n*10 + int(r-'0')
			case r == 'H':
				dur += time.Duration(n) * time.Hour
				n = 0
			case r == 'M':
				dur += time.Duration(n) * time.Minute
				n = 0
			case r == 'S':
				dur += time.Duration(n) * time.Second
				n = 0
			default:
				return time.Time{}, fmt.Errorf("unsupported TRIGGER %q", trigger)
			}
		}
	} else if strings.HasSuffix(body, "D") {
		days, err := strconv.Atoi(strings.TrimSuffix(body, "D"))
		if err != nil {
			return time.Time{}, err
		}
		dur = time.Duration(days) * 24 * time.Hour
	} else {
		return time.Time{}, fmt.Errorf("unsupported TRIGGER %q", trigger)
	}
	if neg {
		return start.Add(-dur), nil
	}
	return start.Add(dur), nil
}
