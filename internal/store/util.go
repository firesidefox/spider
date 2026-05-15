package store

import "strings"

// sqlPlaceholders returns n comma-separated "?" placeholders for SQL IN clauses.
func sqlPlaceholders(n int) string {
	if n == 0 {
		return ""
	}
	return "?" + strings.Repeat(",?", n-1)
}
