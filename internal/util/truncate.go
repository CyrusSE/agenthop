package util

import "strings"

// TruncateRunes shortens s to at most n runes without splitting UTF-8 code points.
func TruncateRunes(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "…"
}

// EscapeLike escapes % and _ for use in SQL LIKE patterns with a fixed prefix/suffix.
func EscapeLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}
