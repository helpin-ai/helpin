package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
)

const workspaceContextMaxPageBytes = 256 * 1024
const workspaceContextMaxPromptChars = 18000
const workspaceContextProvider = "openrouter"
const workspaceContextModel = "deepseek/deepseek-v4-flash-0731"

type workspaceContextLLM interface {
	ChatCompletion(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error)
}

type WorkspaceContextFetcher interface {
	FetchText(ctx context.Context, rawURL string) (string, error)
}

type HTTPWorkspaceContextFetcher struct {
	Client *http.Client
}

func (f HTTPWorkspaceContextFetcher) FetchText(ctx context.Context, rawURL string) (string, error) {
	client := f.Client
	if client == nil {
		client = &http.Client{Timeout: 8 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Helpin-Onboarding/1.0")
	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return "", fmt.Errorf("fetch %s: status %d", rawURL, res.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, workspaceContextMaxPageBytes))
	if err != nil {
		return "", err
	}
	return htmlToPlainText(string(body)), nil
}

func (s *WorkspaceService) GenerateCompanyProductDescription(ctx context.Context, req model.GenerateWorkspaceContextDescriptionRequest) (*model.GenerateWorkspaceContextDescriptionResponse, error) {
	if s.contextLLM == nil {
		return nil, fmt.Errorf("company/product context generation is not configured")
	}
	websiteURL, err := normalizeWorkspaceWebsiteURL(&req.WebsiteURL)
	if err != nil {
		return nil, err
	}
	if websiteURL == nil || strings.TrimSpace(*websiteURL) == "" {
		return nil, fmt.Errorf("website_url is required")
	}

	fetcher := s.contextFetcher
	if fetcher == nil {
		fetcher = HTTPWorkspaceContextFetcher{}
	}
	pageText := fetchWorkspaceContextPages(ctx, fetcher, *websiteURL)
	if strings.TrimSpace(pageText) == "" {
		return nil, fmt.Errorf("could not read useful text from website")
	}
	if len(pageText) > workspaceContextMaxPromptChars {
		pageText = pageText[:workspaceContextMaxPromptChars]
	}

	meteringCtx := WithAIUsageMetering(ctx, AIUsageMeteringContext{
		WorkspaceID:    strings.TrimSpace(req.WorkspaceID),
		FeatureKey:     BillingFeatureCompanyProductContext,
		IdempotencyKey: aiUsageIdempotencyKey(strings.TrimSpace(req.WorkspaceID), "company_product_context", aiUsageStableHash(strings.TrimSpace(req.WorkspaceName)+"|"+strings.TrimSpace(*websiteURL))),
		Metadata: map[string]interface{}{
			"workspace_name": strings.TrimSpace(req.WorkspaceName),
			"website_url":    strings.TrimSpace(*websiteURL),
		},
	})

	chatRequest := llm.ChatRequest{
		Provider:     workspaceContextProvider,
		Model:        workspaceContextModel,
		SystemPrompt: "You draft compact, factual company/product context for AI agents. Use only the provided website text. Return plain text only.",
		Messages: []llm.Message{{
			Role: "user",
			Content: fmt.Sprintf(`Draft company/product context for workspace %q.

Write compact plain text with short labeled sections when useful:
Start with one unlabeled product summary pointer.
Audience:
Key capabilities:
Positioning:
Competitors:

Put "- " before each content pointer. Do not include a Product label. Use Competitors only when competitor names or alternatives are clearly present in the website text. Group related tools, channels, platforms, and integrations instead of listing every item. Use short lines and avoid long comma-separated lists. Do not use formatting syntax such as heading markers, bold markers, or code fences. Use as few words as possible without dropping important product facts. Avoid repeated claims, generic marketing language, unsupported claims, and granular website details that should remain in website sources. Keep it useful for support, docs, planning, and engineering agents.

Website text:
%s`, strings.TrimSpace(req.WorkspaceName), pageText),
		}},
		Temperature: 0.2,
		MaxTokens:   2400,
	}
	resp, err := s.contextLLM.ChatCompletion(meteringCtx, chatRequest)
	if err != nil {
		return nil, fmt.Errorf("generate company/product context: %w", err)
	}
	description := normalizeCompanyProductContextPlainText(resp.Content)
	if description == "" {
		return nil, fmt.Errorf("generated company/product context was empty")
	}
	return &model.GenerateWorkspaceContextDescriptionResponse{
		Description:           description,
		CompanyProductContext: description,
	}, nil
}

func normalizeCompanyProductContextPlainText(raw string) string {
	cleaned := strings.ReplaceAll(raw, "\r\n", "\n")
	cleaned = strings.ReplaceAll(cleaned, "\r", "\n")
	cleaned = strings.ReplaceAll(cleaned, "**", "")
	cleaned = strings.ReplaceAll(cleaned, "__", "")
	lines := strings.Split(cleaned, "\n")
	out := make([]string, 0, len(lines))
	seenContent := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "```") {
			continue
		}
		line = markdownHeadingRE.ReplaceAllString(line, "")
		line = markdownNumberedListRE.ReplaceAllString(line, "- ")
		line = normalizeCompanyProductContextLine(line, !seenContent)
		if strings.TrimSpace(line) != "" {
			seenContent = true
		}
		out = append(out, strings.TrimSpace(line))
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

func normalizeCompanyProductContextLine(line string, firstContentLine bool) string {
	lower := strings.ToLower(line)
	if lower == "product:" {
		return ""
	}
	if firstContentLine && strings.HasPrefix(lower, "product:") {
		return "- " + strings.TrimSpace(line[len("Product:"):])
	}
	if lower == "customers:" {
		return "Audience:"
	}
	if strings.HasPrefix(lower, "customers:") {
		return "Audience:\n- " + strings.TrimSpace(line[len("Customers:"):])
	}
	if strings.HasPrefix(line, "* ") || strings.HasPrefix(line, "+ ") {
		return "- " + strings.TrimSpace(line[2:])
	}
	return line
}

func fetchWorkspaceContextPages(ctx context.Context, fetcher WorkspaceContextFetcher, baseURL string) string {
	candidates := []string{
		baseURL,
		joinWebsitePath(baseURL, "/features"),
		joinWebsitePath(baseURL, "/product"),
		joinWebsitePath(baseURL, "/solutions"),
		joinWebsitePath(baseURL, "/pricing"),
		joinWebsitePath(baseURL, "/about"),
	}
	seen := map[string]bool{}
	var b strings.Builder
	for _, candidate := range candidates {
		if seen[candidate] {
			continue
		}
		seen[candidate] = true
		text, err := fetcher.FetchText(ctx, candidate)
		if err != nil || strings.TrimSpace(text) == "" {
			continue
		}
		b.WriteString("\n\nURL: ")
		b.WriteString(candidate)
		b.WriteString("\n")
		b.WriteString(strings.TrimSpace(text))
	}
	return b.String()
}

func joinWebsitePath(baseURL string, path string) string {
	return strings.TrimRight(baseURL, "/") + path
}

var (
	scriptStyleRE = regexp.MustCompile(`(?is)<(script|style|noscript)[^>]*>.*?</(script|style|noscript)>`)
	tagRE         = regexp.MustCompile(`(?s)<[^>]+>`)
	spaceRE       = regexp.MustCompile(`\s+`)

	markdownHeadingRE      = regexp.MustCompile(`^#{1,6}\s+`)
	markdownNumberedListRE = regexp.MustCompile(`^\d+[.)]\s+`)
)

func htmlToPlainText(raw string) string {
	withoutScripts := scriptStyleRE.ReplaceAllString(raw, " ")
	withoutTags := tagRE.ReplaceAllString(withoutScripts, " ")
	replacer := strings.NewReplacer(
		"&amp;", "&",
		"&lt;", "<",
		"&gt;", ">",
		"&quot;", `"`,
		"&#39;", "'",
		"&nbsp;", " ",
	)
	return strings.TrimSpace(spaceRE.ReplaceAllString(replacer.Replace(withoutTags), " "))
}
