package calendar

import (
	"context"
	"fmt"
)

const (
	// UserFacingUnconfigured is shown when ics_path is empty.
	UserFacingUnconfigured = "日历未配置"
	// UserFacingUnreadable is shown when the ICS path cannot be read.
	UserFacingUnreadable = "日历暂时读不到"
	// UnconfiguredSource labels an empty, honest calendar — not invented events.
	UnconfiguredSource = "calendar: ICS path is not configured"
	errUnconfigured    = "calendar: ICS path is not configured"
)

// Unconfigured is a calendar.Source used when the user has no ics_path.
// List methods return an error; they never invent events.
type Unconfigured struct{}

// NewUnconfigured is the default live calendar when config has no ICS path.
func NewUnconfigured() *Unconfigured {
	return &Unconfigured{}
}

func (u *Unconfigured) ListEvents(ctx context.Context, w Window) ([]Event, error) {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}
	if err := w.Valid(); err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("%s", errUnconfigured)
}

func (u *Unconfigured) ListReminders(ctx context.Context, w Window) ([]Reminder, error) {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}
	if err := w.Valid(); err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("%s", errUnconfigured)
}

// UserMessageForError maps a calendar error to UI copy.
func UserMessageForError(err error) string {
	if err == nil {
		return ""
	}
	if err.Error() == errUnconfigured {
		return UserFacingUnconfigured
	}
	return UserFacingUnreadable
}
