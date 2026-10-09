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
// Only documented tomorrow-8am umbrella phrases match. Conflicting or
// larger clock expressions (eight pm, 十八点, 18:00) are rejected.
func Recognize(text string) Intent {
	folded := fold(text)
	if !mentionsUmbrella(folded) || !mentionsTomorrow(folded) || !mentionsEightAM(folded) {
		return Intent{Kind: Unknown}
	}
	return Intent{Kind: UmbrellaReminder, Hour: 8, Minute: 0}
}

func mentionsUmbrella(folded string) bool {
	return strings.Contains(folded, "umbrella") ||
		strings.Contains(folded, "带伞") ||
		strings.Contains(folded, "带把伞") ||
		(strings.Contains(folded, "伞") && (strings.Contains(folded, "提醒") || strings.Contains(folded, "remind")))
}

func mentionsTomorrow(folded string) bool {
	return strings.Contains(folded, "tomorrow") ||
		strings.Contains(folded, "明天") ||
		strings.Contains(folded, "明早")
}

func mentionsEightAM(folded string) bool {
	if hasConflictingClock(folded) {
		return false
	}
	explicit := []string{
		"8am", "08:00", "8:00", "8:00am", "08:00am",
		"早上八点", "上午八点", "明早八点",
		"早上8点", "上午8点", "明早8点",
		"早上08:00", "上午08:00",
	}
	for _, p := range explicit {
		if strings.Contains(folded, p) {
			return true
		}
	}
	morning := strings.Contains(folded, "早上") ||
		strings.Contains(folded, "上午") ||
		strings.Contains(folded, "明早") ||
		strings.Contains(folded, "morning")
	if strings.Contains(folded, "八点") || strings.Contains(folded, "8点") {
		return morning
	}
	if hasLatinToken(folded, "eight") || strings.Contains(folded, "8:00") {
		return morning || hasLatinToken(folded, "am") || strings.HasSuffix(folded, "8am") || strings.Contains(folded, "8am")
	}
	return false
}

func hasConflictingClock(folded string) bool {
	conflicts := []string{
		"pm", "p.m.", "p.m",
		"eightpm", "8pm", "08pm",
		"晚上", "傍晚", "下午",
		"十八点", "18点", "18:00",
		"二十点", "20:00", "20点",
	}
	for _, c := range conflicts {
		if strings.Contains(folded, c) {
			return true
		}
	}
	return false
}

func hasLatinToken(s, token string) bool {
	start := 0
	for {
		i := strings.Index(s[start:], token)
		if i < 0 {
			return false
		}
		i += start
		beforeOK := i == 0 || !isLatinLetter(rune(s[i-1]))
		after := i + len(token)
		afterOK := after == len(s) || !isLatinLetter(rune(s[after]))
		if beforeOK && afterOK {
			return true
		}
		start = i + 1
	}
}

func isLatinLetter(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
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
