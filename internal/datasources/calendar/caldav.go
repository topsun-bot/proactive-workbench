package calendar

import (
	"context"
	"errors"
	"fmt"
)

// ErrCalDAVStub is returned by the CalDAV adapter. It never dials the network.
var ErrCalDAVStub = errors.New("calendar: CalDAV adapter is a stub (no HTTP); use ICS files or see DESIGN.md")

// CalDAV is a placeholder for a later REPORT calendar-query client.
// Constructing it is allowed so the interface stays stable; every list call
// fails with ErrCalDAVStub so unit tests and CI cannot hit a CalDAV host.
type CalDAV struct {
	BaseURL  string
	Username string
}

func NewCalDAV(baseURL, username string) (*CalDAV, error) {
	if baseURL == "" {
		return nil, fmt.Errorf("calendar: CalDAV URL is required to construct the stub (calls still do not dial)")
	}
	return &CalDAV{BaseURL: baseURL, Username: username}, nil
}

func (c *CalDAV) ListEvents(ctx context.Context, w Window) ([]Event, error) {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}
	if err := w.Valid(); err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("%w: %s", ErrCalDAVStub, c.BaseURL)
}

func (c *CalDAV) ListReminders(ctx context.Context, w Window) ([]Reminder, error) {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}
	if err := w.Valid(); err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("%w: %s", ErrCalDAVStub, c.BaseURL)
}
