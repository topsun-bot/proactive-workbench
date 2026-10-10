package clock

import (
	"fmt"
	"time"

	// Embed IANA zoneinfo so the static linux/amd64 binary can load
	// Asia/Shanghai (and other zones) without /usr/share/zoneinfo.
	_ "time/tzdata"
)

// Clock is injectable so "tomorrow 08:00" is deterministic in tests.
type Clock struct {
	Now      func() time.Time
	Location *time.Location
}

func Live(loc *time.Location) Clock {
	if loc == nil {
		loc = time.Local
	}
	return Clock{
		Now:      func() time.Time { return time.Now() },
		Location: loc,
	}
}

func Fixed(now time.Time, loc *time.Location) Clock {
	if loc == nil {
		loc = now.Location()
	}
	return Clock{
		Now:      func() time.Time { return now },
		Location: loc,
	}
}

// NextMorning returns 08:00 (or hour:minute) on the next calendar day.
func (c Clock) NextMorning(hour, minute int) (time.Time, error) {
	return NextMorning(c.Now().In(c.Location), c.Location, hour, minute)
}

func NextMorning(now time.Time, loc *time.Location, hour, minute int) (time.Time, error) {
	if hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		return time.Time{}, fmt.Errorf("invalid reminder time %02d:%02d", hour, minute)
	}
	local := now.In(loc)
	startOfToday := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
	tomorrow := startOfToday.AddDate(0, 0, 1)
	return time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), hour, minute, 0, 0, loc), nil
}
