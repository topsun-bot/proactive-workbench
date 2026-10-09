package calendar

import (
	"strings"
	"unicode"
)

func indexFold(s, sub string) int {
	return strings.Index(fold(s), fold(sub))
}

func fold(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range strings.ToLower(s) {
		if unicode.IsSpace(r) {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
