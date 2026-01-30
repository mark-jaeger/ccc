package shellutil

import "strings"

// Quote returns s wrapped in single quotes with internal single quotes
// escaped for safe use in sh/bash command strings.
func Quote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
