package anthropictokenizer

import "strings"

// CountTokens returns a lightweight Claude token estimate.
// Kiro uses this only for max-output truncation and usage fallback; exact
// billing still comes from upstream usage when available.
func CountTokens(text string) int {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0
	}
	runes := len([]rune(text))
	count := runes / 4
	if count <= 0 {
		return 1
	}
	return count
}
