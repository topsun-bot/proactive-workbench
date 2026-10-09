package intent

import (
	"strings"
	"unicode"
)

type Kind int

const (
	Unknown Kind = iota
	UmbrellaReminder
)

type Intent struct {
	Kind   Kind
	Hour   int
	Minute int
}

// Recognize maps a free-text goal onto a known collaboration flow.
// Only the documented tomorrow-8am umbrella phrases match; other times
// (e.g. "today at 5pm") are unrecognized rather than silently rewritten.
func Recognize(text string) Intent {
	folded := fold(text)
	mentionsUmbrella := strings.Contains(folded, "umbrella") ||
		strings.Contains(folded, "带伞") ||
		strings.Contains(folded, "带把伞") ||
		(strings.Contains(folded, "伞") && (strings.Contains(folded, "提醒") || strings.Contains(folded, "remind")))
	mentionsTomorrow := strings.Contains(folded, "tomorrow") ||
		strings.Contains(folded, "明天") ||
		strings.Contains(folded, "明早")
	mentionsEight := strings.Contains(folded, "8am") ||
		strings.Contains(folded, "08:00") ||
		strings.Contains(folded, "8:00") ||
		strings.Contains(folded, "八点") ||
		strings.Contains(folded, "8点") ||
		strings.Contains(folded, "eight")
	if !mentionsUmbrella || !mentionsTomorrow || !mentionsEight {
		return Intent{Kind: Unknown}
	}
	return Intent{Kind: UmbrellaReminder, Hour: 8, Minute: 0}
}

func fold(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range strings.ToLower(s) {
		if unicode.IsSpace(r) {
			continue
		}
		if r == '：' {
			r = ':'
		}
		b.WriteRune(r)
	}
	return b.String()
}
