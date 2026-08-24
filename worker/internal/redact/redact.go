package redact

import (
	"regexp"
	"strings"
)

var (
	keyValuePattern = regexp.MustCompile(`(?i)(\b(?:password|passwd|pwd|secret|token|authorization)\b\s*[=:]\s*)(\S+)`)
	bearerPattern   = regexp.MustCompile(`(?i)(\bbearer\s+)([A-Za-z0-9._~+/=-]+)`)
	urlPattern      = regexp.MustCompile(`([a-zA-Z][a-zA-Z0-9+.-]*://[^:/\s@]+):([^@\s]+)@`)
)

// New builds a redaction function that replaces known secrets and credential
// patterns before anything leaves the worker.
func New(secrets ...string) func(string) string {
	return func(input string) string {
		s := input
		for _, secret := range secrets {
			if secret != "" && len(secret) >= 4 {
				s = strings.ReplaceAll(s, secret, "******")
			}
		}
		s = bearerPattern.ReplaceAllString(s, "${1}******")
		s = keyValuePattern.ReplaceAllString(s, "${1}******")
		s = urlPattern.ReplaceAllString(s, "${1}:******@")
		return s
	}
}
