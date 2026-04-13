package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

var (
	schemaBlockPattern    = regexp.MustCompile(`(?s)Return a JSON object with this exact structure:\s*(\{.*?\})\s*RULES:`)
	transcriptLinePattern = regexp.MustCompile(`(?m)^\[(\d+)ms ([^\]]+)\]:\s*(.+)$`)
)

// StubProvider generates deterministic, local JSON without calling any model.
// It is intended for offline development and installer smoke tests.
type StubProvider struct{}

func NewStubProvider() *StubProvider {
	return &StubProvider{}
}

func (p *StubProvider) Name() string {
	return "stub"
}

func (p *StubProvider) IsAvailable() bool {
	return true
}

func (p *StubProvider) Generate(_ context.Context, prompt string) (string, error) {
	skeleton := extractSchemaSkeleton(prompt)
	if skeleton == nil {
		skeleton = map[string]interface{}{
			"summary":   map[string]interface{}{"overview": "", "phases": []interface{}{}},
			"sections":  map[string]interface{}{},
			"citations": []interface{}{},
		}
	}

	lines := transcriptLinePattern.FindAllStringSubmatch(prompt, -1)
	summary := map[string]interface{}{
		"overview": "",
		"phases":   []map[string]interface{}{},
	}
	if len(lines) > 0 {
		first := strings.TrimSpace(lines[0][3])
		last := strings.TrimSpace(lines[len(lines)-1][3])
		summary["overview"] = fmt.Sprintf("Stub summary generated from %d transcript lines. Opening: %s. Latest: %s.", len(lines), trimForStub(first), trimForStub(last))
		summary["phases"] = []map[string]interface{}{
			{
				"title":    "Stub phase",
				"summary":  fmt.Sprintf("Conversation captured in stub mode across %d transcript lines.", len(lines)),
				"start_ms": mustInt(lines[0][1]),
				"end_ms":   mustInt(lines[len(lines)-1][1]),
			},
		}
	}
	skeleton["summary"] = summary

	if sections, ok := skeleton["sections"].(map[string]interface{}); ok {
		for key, raw := range sections {
			switch typed := raw.(type) {
			case []interface{}:
				if len(typed) == 0 && len(lines) > 0 {
					sections[key] = []string{fmt.Sprintf("Stub mode: no real LLM analysis for %s", key)}
				}
			}
		}
	}

	skeleton["citations"] = []interface{}{}
	out, err := json.Marshal(skeleton)
	if err != nil {
		return "", fmt.Errorf("stub llm marshal: %w", err)
	}
	return string(out), nil
}

func extractSchemaSkeleton(prompt string) map[string]interface{} {
	matches := schemaBlockPattern.FindStringSubmatch(prompt)
	if len(matches) != 2 {
		return nil
	}
	var out map[string]interface{}
	if err := json.Unmarshal([]byte(matches[1]), &out); err != nil {
		return nil
	}
	return out
}

func trimForStub(s string) string {
	s = strings.TrimSpace(s)
	if len(s) <= 80 {
		return s
	}
	return s[:77] + "..."
}

func mustInt(raw string) int {
	var value int
	for _, r := range raw {
		if r < '0' || r > '9' {
			continue
		}
		value = value*10 + int(r-'0')
	}
	return value
}
