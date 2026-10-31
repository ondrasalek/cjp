package textnorm

import (
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// NormalizeWord trims whitespace and converts a word to lowercase UTF-8 NFC.
func NormalizeWord(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	return norm.NFC.String(strings.ToLower(s))
}

// StripDiacritics converts a string to lowercase ASCII-friendly form by removing
// Czech/Latin diacritical marks (e.g., "Žluťoučký kůň" -> "zlutoucky kun").
func StripDiacritics(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return ""
	}
	// Fast path for pure ASCII strings
	isASCII := true
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			isASCII = false
			break
		}
	}
	if isASCII {
		return s
	}

	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	out, _, err := transform.String(t, s)
	if err != nil {
		return s
	}
	return out
}
