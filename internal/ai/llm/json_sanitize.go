package llm

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

var trailingCommaRE = regexp.MustCompile(`,(\s*[}\]])`)

func sanitizeJSON(input string) string {
	s := strings.TrimPrefix(strings.TrimSpace(input), "\ufeff")
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

	if obj := extractJSONObject(s); obj != "" {
		s = obj
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

func extractJSONObject(s string) string {
	start := strings.Index(s, "{")
	if start < 0 {
		return ""
	}

	depth := 0
	inString := false
	escaped := false
	for i := start; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 0 {
			break
		}

		if inString {
			if escaped {
				escaped = false
			} else {
				switch r {
				case '\\':
					escaped = true
				case '"':
					inString = false
				}
			}
			i += size
			continue
		}

		switch r {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[start : i+size]
			}
		}
		i += size
	}

	end := strings.LastIndex(s, "}")
	if end > start {
		return s[start : end+1]
	}
	return ""
}
