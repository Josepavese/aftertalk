package llm

import (
	"regexp"
	"strings"
)

var trailingCommaRE = regexp.MustCompile(`,(\s*[}\]])`)

func sanitizeJSON(input string) string {
	s := strings.TrimSpace(input)
	if s == "" {
		return s
	}

	if strings.HasPrefix(s, "```") {
		if idx := strings.Index(s, "\n"); idx != -1 {
			s = s[idx+1:]
		}
		if end := strings.LastIndex(s, "```"); end != -1 {
			s = s[:end]
		}
		s = strings.TrimSpace(s)
	}

	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start >= 0 && end > start {
		s = s[start : end+1]
	}

	for {
		next := trailingCommaRE.ReplaceAllString(s, "$1")
		if next == s {
			break
		}
		s = next
	}

	return strings.TrimSpace(s)
}
