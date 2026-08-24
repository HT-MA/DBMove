package engine

import "strings"

// tailSuffix formats the tail of a failed command's output for error messages.
func tailSuffix(tail []string) string {
	if len(tail) == 0 {
		return ""
	}
	const max = 8
	if len(tail) > max {
		tail = tail[len(tail)-max:]
	}
	return ": " + strings.Join(tail, " | ")
}
