package service

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
)

const (
	workspaceContextMaxPageBytes   = 256 * 1024
	workspaceContextMaxPromptChars = 18000
	// workspaceContextFetchBudget bounds reading the website; pages are fetched
	// concurrently and whatever arrived by then is used.
	workspaceContextFetchBudget = 15 * time.Second
	// workspaceContextModelTimeout bounds the model call, including a fallback
	// route, so the request finishes within about 40 seconds.
	workspaceContextModelTimeout = 25 * time.Second
)

// Company/product context generation failure codes. They are part of the
// HTTP contract: the handler maps each to a fixed user-facing sentence.
const (
	WorkspaceContextErrAIUnavailable     = "ai_unavailable"
	WorkspaceContextErrWebsiteUnreadable = "website_unreadable"
	WorkspaceContextErrTimeout           = "timeout"
	WorkspaceContextErrGenerationFailed  = "generation_failed"
)

// WorkspaceContextError is a company/product context generation failure with
// a stable code. Err carries internal detail for logs only.
type WorkspaceContextError struct {
	Code string
	Err  error
}

func (e *WorkspaceContextError) Error() string {
	if e.Err == nil {
		return "company/product context generation: " + e.Code
	}
	return "company/product context generation: " + e.Code + ": " + e.Err.Error()
}

func (e *WorkspaceContextError) Unwrap() error { return e.Err }

// WorkspaceContextValidationError rejects invalid input with a message that is
// safe to show to the user.
type WorkspaceContextValidationError struct {
	Message string
}

func (e *WorkspaceContextValidationError) Error() string { return e.Message }

type workspaceContextLLM interface {
	ChatCompletion(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error)
}

// workspaceContextProfiles resolves the workspace default AI profile.
type workspaceContextProfiles interface {
	ResolveChatExecution(ctx context.Context, workspace, user string) (*AIProfileExecution, error)
}

// workspaceContextProfileCompleter runs a completion on a resolved profile.
type workspaceContextProfileCompleter interface {
	CompleteWithProfile(ctx context.Context, input AICompletionRequest, execution *AIProfileExecution) (*llm.ChatResponse, error)
}

// workspaceContextAvailability reports whether server-configured routes can run a feature.
type workspaceContextAvailability interface {
	FeatureAvailable(feature, operation string) bool
}

type workspaceContextCompletion func(ctx context.Context, input AICompletionRequest) (*llm.ChatResponse, error)

// SetContextAIProfiles lets context generation run on the workspace's default
// AI profile when a workspace is named.
func (s *WorkspaceService) SetContextAIProfiles(profiles workspaceContextProfiles) *WorkspaceService {
	s.contextProfiles = profiles
	return s
}

// GenerateCompanyProductDescription drafts company/product context from a
// public website. AI availability is checked before any page is fetched.
func (s *WorkspaceService) GenerateCompanyProductDescription(ctx context.Context, userID string, req model.GenerateWorkspaceContextDescriptionRequest) (*model.GenerateWorkspaceContextDescriptionResponse, error) {
	websiteURL, err := validateWorkspaceContextWebsite(req.WebsiteURL)
	if err != nil {
		return nil, err
	}
	workspaceID := strings.TrimSpace(req.WorkspaceID)
	complete, err := s.workspaceContextCompletion(ctx, workspaceID, userID)
	if err != nil {
		return nil, &WorkspaceContextError{Code: WorkspaceContextErrAIUnavailable, Err: err}
	}

	fetcher := s.contextFetcher
	if fetcher == nil {
		fetcher = HTTPWorkspaceContextFetcher{}
	}
	fetchCtx, cancelFetch := context.WithTimeout(ctx, workspaceContextFetchBudget)
	pageText := fetchWorkspaceContextPages(fetchCtx, fetcher, websiteURL)
	cancelFetch()
	if err := ctx.Err(); err != nil {
		return nil, &WorkspaceContextError{Code: WorkspaceContextErrTimeout, Err: err}
	}
	if strings.TrimSpace(pageText) == "" {
		return nil, &WorkspaceContextError{Code: WorkspaceContextErrWebsiteUnreadable, Err: errors.New("no readable text on the website")}
	}
	if len(pageText) > workspaceContextMaxPromptChars {
		pageText = pageText[:workspaceContextMaxPromptChars]
	}

	input := workspaceContextCompletionInput(workspaceID, strings.TrimSpace(req.WorkspaceName), websiteURL, pageText)
	modelCtx, cancelModel := context.WithTimeout(ctx, workspaceContextModelTimeout)
	defer cancelModel()
	resp, err := complete(modelCtx, input)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(modelCtx.Err(), context.DeadlineExceeded) {
			return nil, &WorkspaceContextError{Code: WorkspaceContextErrTimeout, Err: err}
		}
		return nil, &WorkspaceContextError{Code: WorkspaceContextErrGenerationFailed, Err: err}
	}
	description := normalizeCompanyProductContextPlainText(resp.Content)
	if description == "" {
		return nil, &WorkspaceContextError{Code: WorkspaceContextErrGenerationFailed, Err: errors.New("generated context was empty")}
	}
	return &model.GenerateWorkspaceContextDescriptionResponse{
		Description:           description,
		CompanyProductContext: description,
	}, nil
}

// workspaceContextCompletion picks the model: the workspace default AI profile
// when a workspace is named and it can run directly, otherwise the
// server-configured routes. It fails when neither is available.
func (s *WorkspaceService) workspaceContextCompletion(ctx context.Context, workspaceID, userID string) (workspaceContextCompletion, error) {
	var profileErr error
	if workspaceID != "" && s.contextProfiles != nil {
		completer, ok := s.contextLLM.(workspaceContextProfileCompleter)
		if !ok {
			profileErr = errors.New("profile completions are not configured")
		} else if execution, err := s.contextProfiles.ResolveChatExecution(ctx, workspaceID, userID); err != nil {
			profileErr = err
		} else {
			return func(ctx context.Context, input AICompletionRequest) (*llm.ChatResponse, error) {
				return completer.CompleteWithProfile(ctx, input, execution)
			}, nil
		}
	}
	if s.contextLLM == nil {
		return nil, errors.Join(profileErr, errors.New("server AI routes are not configured"))
	}
	if available, ok := s.contextLLM.(workspaceContextAvailability); ok && !available.FeatureAvailable(BillingFeatureCompanyProductContext, "") {
		return nil, errors.Join(profileErr, errors.New("no server AI provider is configured for context generation"))
	}
	return func(ctx context.Context, input AICompletionRequest) (*llm.ChatResponse, error) {
		return completeAI(ctx, s.contextLLM, input)
	}, nil
}

// validateWorkspaceContextWebsite normalizes the website and rejects addresses
// that can never be fetched: non-HTTP schemes, credentials, localhost and
// literal private, loopback, link-local or metadata addresses.
func validateWorkspaceContextWebsite(raw string) (string, error) {
	websiteURL, err := normalizeWorkspaceWebsiteURL(&raw)
	if err != nil {
		return "", &WorkspaceContextValidationError{Message: "Enter a valid website address, such as https://example.com."}
	}
	if websiteURL == nil || strings.TrimSpace(*websiteURL) == "" {
		return "", &WorkspaceContextValidationError{Message: "website_url is required"}
	}
	parsed, err := url.Parse(*websiteURL)
	if err != nil || !isAllowedSupportPreviewURL(parsed) {
		return "", &WorkspaceContextValidationError{Message: "Enter a public website address, such as https://example.com."}
	}
	if addr, err := netipParseHost(parsed.Hostname()); err == nil && !isPublicSupportPreviewIP(addr) {
		return "", &WorkspaceContextValidationError{Message: "Enter a public website address, such as https://example.com."}
	}
	return *websiteURL, nil
}

func workspaceContextCompletionInput(workspaceID, workspaceName, websiteURL, pageText string) AICompletionRequest {
	return AICompletionRequest{
		WorkspaceID:    workspaceID,
		FeatureKey:     BillingFeatureCompanyProductContext,
		IdempotencyKey: aiUsageIdempotencyKey(workspaceID, "company_product_context", aiUsageStableHash(workspaceName+"|"+websiteURL)),
		Metadata: map[string]interface{}{
			"workspace_name": workspaceName,
			"website_url":    websiteURL,
		},
		RequireComplete: true,
		Chat: llm.ChatRequest{
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
%s`, workspaceName, pageText),
			}},
			Temperature: 0.2,
			MaxTokens:   2400,
		},
	}
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
