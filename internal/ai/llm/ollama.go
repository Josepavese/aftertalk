package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	ollamaapi "github.com/ollama/ollama/api"

	"github.com/Josepavese/aftertalk/internal/logging"
)

// OllamaProvider calls a locally-running Ollama instance via the official Go client.
// No CGO required — the client communicates with the Ollama daemon over HTTP.
type OllamaProvider struct {
	client *ollamaapi.Client
	model  string
	think  *bool
}

func NewOllamaProvider(cfg OllamaConfig) (*OllamaProvider, error) {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}

	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("ollama: invalid base URL %q: %w", baseURL, err)
	}

	model := cfg.Model
	if model == "" {
		model = "llama3.2:3b"
	}
	think := cfg.Think
	if think == nil && isThinkingModelDefaultOff(model) {
		disabled := false
		think = &disabled
	}

	client := ollamaapi.NewClient(u, http.DefaultClient)
	return &OllamaProvider{client: client, model: model, think: think}, nil
}

func (p *OllamaProvider) Generate(ctx context.Context, prompt string) (string, error) {
	logging.Infof("Ollama: generating with model %s", p.model)

	// Collect streamed response chunks into a single string.
	var sb strings.Builder
	stream := false
	jsonFormat := json.RawMessage(`"json"`)
	req := &ollamaapi.GenerateRequest{
		Model:  p.model,
		Prompt: prompt,
		Stream: &stream,
		Format: jsonFormat,
	}
	if p.think != nil {
		req.Think = &ollamaapi.ThinkValue{Value: *p.think}
	}

	var thinking strings.Builder
	err := p.client.Generate(ctx, req, func(resp ollamaapi.GenerateResponse) error {
		sb.WriteString(resp.Response)
		thinking.WriteString(resp.Thinking)
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("ollama generate: %w", err)
	}

	result := strings.TrimSpace(sb.String())
	thinkingText := strings.TrimSpace(thinking.String())
	if result == "" && thinkingText != "" {
		return "", fmt.Errorf("ollama: %w (model=%s, think=%v)", errReasoningOnly, p.model, p.thinkValueForLog())
	}
	logging.Infof("Ollama: response length=%d", len(result))
	return result, nil
}

func (p *OllamaProvider) Name() string { return "ollama" }

func (p *OllamaProvider) IsAvailable() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*1e9) // 2s
	defer cancel()
	_, err := p.client.Version(ctx)
	return err == nil
}

func isThinkingModelDefaultOff(model string) bool {
	name := strings.ToLower(model)
	return strings.Contains(name, "qwen3.5") || strings.Contains(name, "qwq")
}

func (p *OllamaProvider) thinkValueForLog() interface{} {
	if p.think == nil {
		return nil
	}
	return *p.think
}
