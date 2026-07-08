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

	resp, err := s.contextLLM.ChatCompletion(ctx, llm.ChatRequest{
		SystemPrompt: "You draft compact, factual company/product context for AI agents. Use only the provided website text. Return markdown.",
		Messages: []llm.Message{{
			Role: "user",
			Content: fmt.Sprintf(`Draft company/product context for workspace %q.

Write markdown with:
- a brief opening summary
- Core capabilities
- Target users
- Positioning, competitors, constraints, or product category when present

Use as few words as possible without dropping important product facts. Avoid repeated claims, generic marketing language, unsupported claims, and granular website details that should remain in website sources. Keep it useful for support, docs, planning, and engineering agents.

Website text:
%s`, strings.TrimSpace(req.WorkspaceName), pageText),
		}},
		Temperature: 0.2,
		MaxTokens:   1200,
	})
	if err != nil {
		return nil, fmt.Errorf("generate company/product context: %w", err)
	}
	description := strings.TrimSpace(resp.Content)
	if description == "" {
		return nil, fmt.Errorf("generated company/product context was empty")
	}
	return &model.GenerateWorkspaceContextDescriptionResponse{
		Description:           description,
		CompanyProductContext: description,
	}, nil
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
