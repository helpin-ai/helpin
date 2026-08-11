package service

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/llm"
)

func TestDocsImportAIRequestTimeoutAllowsLongArticleFormatting(t *testing.T) {
	if defaultDocsImportAIRequestTimeout != 4*time.Minute {
		t.Fatalf("request timeout = %s, want 4m", defaultDocsImportAIRequestTimeout)
	}
}

func TestDocsImportAIConversionDefaultsToGPT56Luna(t *testing.T) {
	config := (DocsImportAIConversionConfig{}).withDefaults()
	if config.Provider != "openrouter" {
		t.Fatalf("provider = %q, want openrouter", config.Provider)
	}
	if config.Model != "openai/gpt-5.6-luna" {
		t.Fatalf("model = %q, want openai/gpt-5.6-luna", config.Model)
	}
}

type docsImportAIStub struct {
	response string
	request  llm.ChatRequest
	calls    int
}

func (s *docsImportAIStub) ChatCompletion(_ context.Context, request llm.ChatRequest) (*llm.ChatResponse, error) {
	s.calls++
	s.request = request
	return &llm.ChatResponse{Content: s.response}, nil
}

func TestDocsImportAIConversionUsesConfiguredModelAndPreservesStructure(t *testing.T) {
	provider := &docsImportAIStub{
		response: `{"markdown":"Use these steps to configure the integration correctly:\n\n1. Open **Settings** in your workspace.\n2. Paste API_KEY=abc123 into the field.\n3. Read the [configuration guide](https://example.com/guide)."}`,
	}
	service := &DocsImportService{
		llmProvider: provider,
		aiConversion: DocsImportAIConversionConfig{
			Enabled:  true,
			Provider: "openrouter",
			Model:    "deepseek/deepseek-v4-flash-0731",
		}.withDefaults(),
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	source := `<p>Use these steps to configure the integration correctly:</p>
		<ol><li>Open <strong>Settings</strong> in your workspace.</li>
		<li>Paste <code>API_KEY=abc123</code> into the field.</li>
		<li>Read the <a href="https://example.com/guide">configuration guide</a>.</li></ol>`

	result, warnings, err := service.convertHelpScoutHTML(
		context.Background(),
		"workspace-1",
		"article-1",
		"Configure the integration",
		source,
	)
	if err != nil {
		t.Fatalf("convertHelpScoutHTML() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings = %#v, want none", warnings)
	}
	if provider.calls != 1 {
		t.Fatalf("provider calls = %d, want 1", provider.calls)
	}
	if provider.request.Provider != "openrouter" ||
		provider.request.Model != "deepseek/deepseek-v4-flash-0731" {
		t.Fatalf("unexpected routing: provider=%q model=%q", provider.request.Provider, provider.request.Model)
	}
	if !provider.request.JSONMode || provider.request.Temperature != 0.1 {
		t.Fatalf("request did not enable deterministic JSON output: %#v", provider.request)
	}
	if !strings.Contains(provider.request.SystemPrompt, "Never add advice") ||
		!strings.Contains(provider.request.SystemPrompt, "Preserve every fact") {
		t.Fatal("system prompt is missing lossless-editing constraints")
	}

	encoded, err := json.Marshal(result.Doc)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	if !strings.Contains(string(encoded), `"type":"orderedList"`) {
		t.Fatalf("result does not contain an ordered list: %s", encoded)
	}
	if !strings.Contains(string(encoded), "https://example.com/guide") {
		t.Fatalf("result lost source URL: %s", encoded)
	}
}

func TestDocsImportAIConversionFallsBackWhenURLIsLost(t *testing.T) {
	provider := &docsImportAIStub{
		response: `{"markdown":"Read the configuration guide for all setup details today."}`,
	}
	service := &DocsImportService{
		llmProvider: provider,
		aiConversion: DocsImportAIConversionConfig{
			Enabled: true,
		}.withDefaults(),
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	source := `<p>Read the <a href="https://example.com/guide">configuration guide</a>
		for all setup details today.</p>`

	result, warnings, err := service.convertHelpScoutHTML(
		context.Background(),
		"workspace-1",
		"article-2",
		"Setup details",
		source,
	)
	if err != nil {
		t.Fatalf("convertHelpScoutHTML() error = %v", err)
	}
	if result == nil {
		t.Fatal("fallback result is nil")
	}
	if len(warnings) == 0 || warnings[len(warnings)-1].Type != "ai_conversion_fallback" {
		t.Fatalf("warnings = %#v, want ai_conversion_fallback", warnings)
	}

	encoded, err := json.Marshal(result.Doc)
	if err != nil {
		t.Fatalf("marshal fallback result: %v", err)
	}
	if !strings.Contains(string(encoded), "https://example.com/guide") {
		t.Fatalf("fallback result lost source URL: %s", encoded)
	}
}

func TestDocsImportAIConversionDisabledDoesNotCallProvider(t *testing.T) {
	provider := &docsImportAIStub{response: `{"markdown":"changed"}`}
	service := &DocsImportService{
		llmProvider:  provider,
		aiConversion: DocsImportAIConversionConfig{Enabled: false}.withDefaults(),
		logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	result, _, err := service.convertHelpScoutHTML(
		context.Background(),
		"workspace-1",
		"article-3",
		"Original",
		"<p>Original article content.</p>",
	)
	if err != nil {
		t.Fatalf("convertHelpScoutHTML() error = %v", err)
	}
	if result == nil {
		t.Fatal("deterministic result is nil")
	}
	if provider.calls != 0 {
		t.Fatalf("provider calls = %d, want 0", provider.calls)
	}
}

func TestDocsImportAIConversionUsesDeterministicConverterForVideo(t *testing.T) {
	provider := &docsImportAIStub{response: `{"markdown":"[video](https://www.youtube.com/embed/ptlSTecgl3c)"}`}
	service := &DocsImportService{
		llmProvider:  provider,
		aiConversion: DocsImportAIConversionConfig{Enabled: true}.withDefaults(),
		logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	result, warnings, err := service.convertHelpScoutHTML(
		context.Background(),
		"workspace-1",
		"article-video",
		"Video article",
		`<h2>Walkthrough</h2><iframe src="https://www.youtube.com/embed/ptlSTecgl3c"></iframe>`,
	)
	if err != nil {
		t.Fatalf("convertHelpScoutHTML() error = %v", err)
	}
	if provider.calls != 0 {
		t.Fatalf("provider calls = %d, want deterministic conversion without AI", provider.calls)
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings = %#v, want none", warnings)
	}
	encoded, err := json.Marshal(result.Doc)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	if !strings.Contains(string(encoded), `"type":"videoEmbed"`) || !strings.Contains(string(encoded), `"embedUrl":"https://www.youtube.com/embed/ptlSTecgl3c"`) {
		t.Fatalf("expected native video embed, got: %s", encoded)
	}
}

func TestParseDocsImportAIResponseAcceptsJSONFence(t *testing.T) {
	markdown, err := parseDocsImportAIResponse("```json\n{\"markdown\":\"## Setup\"}\n```")
	if err != nil {
		t.Fatalf("parseDocsImportAIResponse() error = %v", err)
	}
	if markdown != "## Setup" {
		t.Fatalf("markdown = %q, want %q", markdown, "## Setup")
	}
}
