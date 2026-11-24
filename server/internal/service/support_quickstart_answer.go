package service

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
)

// ErrSupportQuickstartBusy means the server's daily allowance of preview
// answers is spent.
var ErrSupportQuickstartBusy = errors.New("support preview answers are busy")

const (
	quickstartAnswerTimeout = 45 * time.Second
	// quickstartAnswerSources is how many articles are read for a visitor's
	// question; quickstartPreviewArticles for the agent's own questions.
	quickstartAnswerSources     = 3
	quickstartPreviewArticles   = 6
	quickstartAnswerPages       = 4
	quickstartAnswerSourceChars = 4000
	quickstartAnswerQuestionMax = 300
	quickstartAnswerMaxTokens   = 3200
	quickstartPreviewAnswers    = 3
	quickstartPreviewGaps       = 3
	quickstartPreviewGapMax     = 140
	// quickstartPreviewTitles caps the article titles the agent is shown when
	// it looks for questions no article covers.
	quickstartPreviewTitles = 300
	// quickstartAnswerDailyLimit caps preview answers for the whole server,
	// since anyone can ask for one without an account.
	quickstartAnswerDailyLimit = 500
	quickstartPreviewCacheTTL  = 24 * time.Hour
	quickstartListingCacheTTL  = time.Hour
)

// quickstartAI runs the preview answer's completion. Production passes the
// AI completion service; availability is checked when it can report it.
type quickstartAI interface {
	ChatCompletion(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error)
}

// quickstartSource is one page the preview answer may use.
type quickstartSource struct {
	id    string
	title string
	url   string
	text  string
}

// quickstartListing is a help center's counts and every article it lists.
type quickstartListing struct {
	Preview *model.PublicHelpCenterImportPreview `json:"preview"`
	Refs    []model.PublicHelpCenterArticleRef   `json:"refs"`
}

type quickstartAnswerContract struct {
	Question      string   `json:"question"`
	CanAnswer     bool     `json:"can_answer"`
	Answer        string   `json:"answer"`
	CitedSourceID []string `json:"cited_source_ids"`
	Confidence    float64  `json:"confidence"`
}

type quickstartPreviewContract struct {
	Answers []quickstartAnswerContract `json:"answers"`
	Gaps    []string                   `json:"gaps"`
}

// PreviewAnswer has the support agent answer one customer question from the
// company's public help center or website, before any account exists. The
// answer must cite the pages it used; anything it can't ground is reported as
// not answerable.
func (s *SupportQuickstartService) PreviewAnswer(ctx context.Context, req model.SupportQuickstartAnswerRequest) (*model.SupportQuickstartAnswer, error) {
	website, helpURL, err := normalizePreviewAddresses(req.WebsiteURL, req.HelpCenterURL)
	if err != nil {
		return nil, err
	}
	question := strings.Join(strings.Fields(req.Question), " ")
	if question == "" {
		return nil, fmt.Errorf("%w: Ask a question.", ErrSupportQuickstartInvalid)
	}
	if utf8.RuneCountInString(question) > quickstartAnswerQuestionMax {
		return nil, fmt.Errorf("%w: Keep the question under %d characters.", ErrSupportQuickstartInvalid, quickstartAnswerQuestionMax)
	}
	out := &model.SupportQuickstartAnswer{Question: question, Sources: []model.PublicHelpCenterArticleRef{}}
	if !s.previewAnswersAvailable() {
		return out, nil
	}
	if !s.consumePreviewAnswerBudget(ctx) {
		return nil, ErrSupportQuickstartBusy
	}
	ctx, cancel := context.WithTimeout(ctx, quickstartAnswerTimeout)
	defer cancel()

	out.Available = true
	var sources []quickstartSource
	if listing := s.helpCenterListing(ctx, helpURL); listing != nil {
		sources = s.readArticles(ctx, pickPreviewArticles(listing.Refs, question), nil)
	} else {
		sources = s.readPages(ctx, previewPages(website, helpURL), nil)
	}
	if len(sources) == 0 {
		return out, nil
	}
	contract, err := s.generatePreview(ctx, companyNameFromURL(website), question, sources, nil)
	if err != nil {
		return nil, fmt.Errorf("preview answer: %w", err)
	}
	if len(contract.Answers) == 0 {
		return out, nil
	}
	if cited, ok := validatePreviewAnswer(&contract.Answers[0], question, sources); ok {
		out.CanAnswer, out.Answer, out.Sources = true, strings.TrimSpace(contract.Answers[0].Answer), cited
	}
	return out, nil
}

// normalizePreviewAddresses checks the website and the optional help center
// address of a preview request.
func normalizePreviewAddresses(rawWebsite, rawHelpCenter string) (website, helpURL string, err error) {
	if website, err = normalizeQuickstartURL(rawWebsite, "website"); err != nil {
		return "", "", err
	}
	if strings.TrimSpace(rawHelpCenter) != "" {
		if helpURL, err = normalizeQuickstartURL(rawHelpCenter, "help center"); err != nil {
			return "", "", err
		}
	}
	return website, helpURL, nil
}

func (s *SupportQuickstartService) previewAnswersAvailable() bool {
	if s.deps.AI == nil {
		return false
	}
	if available, ok := s.deps.AI.(workspaceContextAvailability); ok {
		return available.FeatureAvailable(BillingFeatureOnboardingPreviewAnswer, "")
	}
	return true
}

// previewPages are the pages read when a help center's articles can't be
// listed: the help center's own page, then the website's main pages.
func previewPages(website, helpURL string) []string {
	pages := workspaceContextCandidates(website)
	if helpURL != "" {
		pages = append([]string{helpURL}, pages...)
	}
	return pages
}

// helpCenterListing lists a public Intercom help center, kept for an hour so
// a visitor's follow-up questions don't read the whole help center again. It
// is nil when there is no help center or its articles can't be listed.
func (s *SupportQuickstartService) helpCenterListing(ctx context.Context, helpURL string) *quickstartListing {
	if helpURL == "" {
		return nil
	}
	key := "onb:listing:" + aiUsageStableHash(strings.TrimRight(helpURL, "/"))
	var listing quickstartListing
	if s.cacheGet(ctx, key, &listing) && len(listing.Refs) > 0 {
		return &listing
	}
	preview, refs, err := s.deps.Imports.PublicHelpCenterArticles(ctx, helpURL)
	if err != nil || len(refs) == 0 {
		s.logger.DebugContext(ctx, "preview: help center articles unavailable", "url", helpURL, "error", err)
		return nil
	}
	listing = quickstartListing{Preview: preview, Refs: refs}
	s.cacheSet(ctx, key, listing, quickstartListingCacheTTL)
	return &listing
}

// readArticles reads the articles side by side. read, when set, is told the
// title of each article as it arrives.
func (s *SupportQuickstartService) readArticles(ctx context.Context, refs []model.PublicHelpCenterArticleRef, read func(title string)) []quickstartSource {
	sources := make([]quickstartSource, len(refs))
	var wg sync.WaitGroup
	for i, ref := range refs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			title, text, err := s.deps.Imports.ReadPublicArticle(ctx, ref.URL)
			if err != nil {
				s.logger.DebugContext(ctx, "preview: article unavailable", "url", ref.URL, "error", err)
				return
			}
			sources[i] = quickstartSource{title: cmp.Or(title, ref.Title), url: ref.URL, text: text}
			if read != nil && strings.TrimSpace(text) != "" {
				read(sources[i].title)
			}
		}()
	}
	wg.Wait()
	return numberedSources(sources)
}

// readPages reads website pages side by side and keeps the first few that
// could be read. read, when set, is told each page as it arrives.
func (s *SupportQuickstartService) readPages(ctx context.Context, pages []string, read func(title string)) []quickstartSource {
	fetcher := s.deps.Pages
	if fetcher == nil {
		fetcher = HTTPWorkspaceContextFetcher{}
	}
	sources := make([]quickstartSource, len(pages))
	var wg sync.WaitGroup
	for i, page := range pages {
		wg.Add(1)
		go func() {
			defer wg.Done()
			text, err := fetcher.FetchText(ctx, page)
			if err != nil {
				s.logger.DebugContext(ctx, "preview: page unavailable", "url", page, "error", err)
				return
			}
			sources[i] = quickstartSource{title: pageTitle(page), url: page, text: text}
			if read != nil && strings.TrimSpace(text) != "" {
				read(sources[i].title)
			}
		}()
	}
	wg.Wait()
	numbered := numberedSources(sources)
	if len(numbered) > quickstartAnswerPages {
		numbered = numbered[:quickstartAnswerPages]
	}
	return numbered
}

// numberedSources drops unreadable pages, trims each one and gives the rest
// the ids the answer cites.
func numberedSources(read []quickstartSource) []quickstartSource {
	var sources []quickstartSource
	for _, source := range read {
		if text := strings.TrimSpace(source.text); text != "" {
			source.text = excerptText(text, quickstartAnswerSourceChars)
			source.id = "s" + strconv.Itoa(len(sources)+1)
			sources = append(sources, source)
		}
	}
	return sources
}

// pickPreviewArticles chooses the articles to read. For a question, those
// whose title and summary share the most words with it. Without one, the
// first article of each collection in turn, so the agent's own questions
// cover different subjects.
func pickPreviewArticles(refs []model.PublicHelpCenterArticleRef, question string) []model.PublicHelpCenterArticleRef {
	words := previewQuestionWords(question)
	if len(words) == 0 {
		return pickAcrossCollections(refs, quickstartPreviewArticles)
	}
	limit := min(quickstartAnswerSources, len(refs))
	type scored struct {
		ref   model.PublicHelpCenterArticleRef
		score int
	}
	ranked := make([]scored, 0, len(refs))
	for _, ref := range refs {
		if score := sharedWords(ref.Title+" "+ref.Summary, words); score > 0 {
			ranked = append(ranked, scored{ref: ref, score: score})
		}
	}
	if len(ranked) == 0 {
		return refs[:limit]
	}
	slices.SortStableFunc(ranked, func(a, b scored) int { return b.score - a.score })
	picked := make([]model.PublicHelpCenterArticleRef, 0, limit)
	for _, item := range ranked[:min(limit, len(ranked))] {
		picked = append(picked, item.ref)
	}
	return picked
}

// pickAcrossCollections takes each collection's first article, then each
// one's second, until it has enough.
func pickAcrossCollections(refs []model.PublicHelpCenterArticleRef, limit int) []model.PublicHelpCenterArticleRef {
	var order []string
	groups := map[string][]model.PublicHelpCenterArticleRef{}
	for _, ref := range refs {
		if _, seen := groups[ref.Collection]; !seen {
			order = append(order, ref.Collection)
		}
		groups[ref.Collection] = append(groups[ref.Collection], ref)
	}
	limit = min(limit, len(refs))
	picked := make([]model.PublicHelpCenterArticleRef, 0, limit)
	for round := 0; len(picked) < limit; round++ {
		for _, name := range order {
			if round < len(groups[name]) && len(picked) < limit {
				picked = append(picked, groups[name][round])
			}
		}
	}
	return picked
}

// sharedWords counts how many of the words appear in the text.
func sharedWords(text string, words []string) int {
	text = strings.ToLower(text)
	count := 0
	for _, word := range words {
		if strings.Contains(text, word) {
			count++
		}
	}
	return count
}

// _previewQuestionFillers are long words that say nothing about a question's
// subject.
var _previewQuestionFillers = []string{
	"what", "when", "where", "which", "does", "have", "your", "with", "from", "that", "this", "will",
	"there", "they", "their", "about", "would", "could", "should", "into", "after", "before", "offer",
}

// previewQuestionWords keeps the words of a question that say what it is
// about: four letters or more, lowercased, without filler words, and without
// a plural "s" so "refunds" also finds "refund".
func previewQuestionWords(question string) []string {
	var words []string
	for _, word := range strings.FieldsFunc(strings.ToLower(question), func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r > 127)
	}) {
		if utf8.RuneCountInString(word) < 4 || slices.Contains(_previewQuestionFillers, word) {
			continue
		}
		if utf8.RuneCountInString(word) > 4 {
			word = strings.TrimSuffix(word, "s")
		}
		if !slices.Contains(words, word) {
			words = append(words, word)
		}
	}
	return words
}

// generatePreview has the agent answer from the sources. With a question it
// answers that one. Without, it picks the questions customers most likely
// ask that the sources answer; given the help center's article titles, it
// also lists common questions none of them covers.
func (s *SupportQuickstartService) generatePreview(ctx context.Context, company, question string, sources []quickstartSource, titles []string) (*quickstartPreviewContract, error) {
	var pages strings.Builder
	for _, source := range sources {
		fmt.Fprintf(&pages, "<page id=%q title=%q>\n%s\n</page>\n", source.id, source.title, source.text)
	}
	task := "Customer question: " + question + "\nReturn exactly one answer, for this question. Leave gaps empty."
	if question == "" {
		task = "Write the " + strconv.Itoa(quickstartPreviewAnswers) + " questions real customers of " + company +
			" are most likely to ask that these pages clearly answer, each from a different page where you can. " +
			"Write each in a customer's own words, as a normal sentence under 15 words, in the pages' language; " +
			"never copy a page title as the question. Then answer it."
		if len(titles) == 0 {
			task += " Leave gaps empty."
		} else {
			task += "\n\nThen fill gaps with up to " + strconv.Itoa(quickstartPreviewGaps) +
				" other questions customers of a company like this often ask that no title in the article list covers. " +
				"Write them in a customer's own words, as normal sentences under 15 words. Leave gaps empty if the list covers the common questions." +
				"\n\nArticle list:\n- " + strings.Join(titles[:min(len(titles), quickstartPreviewTitles)], "\n- ")
		}
	}
	system := `You are the customer support agent for ` + company + `. You answer customers using ONLY the provided pages from the company's own help center or website.

Rules for every answer:
- Use only facts stated in the pages. Exact values (prices, limits, plan names) must appear verbatim in a page.
- Do not include any URLs or links.
- Be friendly and concise, under 90 words: a short paragraph, or brief numbered steps for a procedure, each step on its own line. You may use **bold** and "- " or "1. " lists; no headings, tables or code.
- Start with the answer itself. Never repeat the question.
- cited_source_ids must list the ids of the pages you used.
- If the pages do not confidently answer the question, set can_answer=false and leave the answer empty. Never guess.
- confidence is your honest 0-1 estimate that the answer is correct and fully grounded.

Return JSON only, in exactly this shape:
{"answers":[{"question":"...","can_answer":true,"answer":"...","cited_source_ids":["s1"],"confidence":0.9}],"gaps":["..."]}`
	text := map[string]any{"type": "string"}
	resp, err := completeAI(ctx, s.deps.AI, AICompletionRequest{
		FeatureKey:     BillingFeatureOnboardingPreviewAnswer,
		IdempotencyKey: aiUsageIdempotencyKey("", BillingFeatureOnboardingPreviewAnswer, uuid.NewString()),
		Chat: llm.ChatRequest{
			SystemPrompt: system,
			Messages:     []llm.Message{{Role: "user", Content: task + "\n\nPages:\n" + pages.String()}},
			MaxTokens:    quickstartAnswerMaxTokens,
			// A visitor is watching: long reasoning costs them the wait and,
			// on reasoning models, the tokens the answers need.
			Reasoning: &llm.ReasoningConfig{Effort: "low"},
			JSONMode:  true,
			// Without strict mode some models answer in a shape of their own.
			JSONSchemaStrict: true,
			JSONSchema: map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"required":             []string{"answers", "gaps"},
				"properties": map[string]any{
					"answers": map[string]any{"type": "array", "items": map[string]any{
						"type":                 "object",
						"additionalProperties": false,
						"required":             []string{"question", "can_answer", "answer", "cited_source_ids", "confidence"},
						"properties": map[string]any{
							"question":         text,
							"can_answer":       map[string]any{"type": "boolean"},
							"answer":           text,
							"cited_source_ids": map[string]any{"type": "array", "items": text},
							"confidence":       map[string]any{"type": "number"},
						},
					}},
					"gaps": map[string]any{"type": "array", "items": text},
				},
			},
		},
	})
	if err != nil {
		return nil, err
	}
	var contract quickstartPreviewContract
	if err := json.Unmarshal([]byte(strings.TrimSpace(resp.Content)), &contract); err != nil {
		return nil, fmt.Errorf("parse answers (finish reason %q): %w", resp.FinishReason, err)
	}
	return &contract, nil
}

// validatePreviewAnswer accepts only a confident, link-free answer to a real
// question that cites pages that were actually read, and returns those pages.
func validatePreviewAnswer(contract *quickstartAnswerContract, question string, sources []quickstartSource) ([]model.PublicHelpCenterArticleRef, bool) {
	if contract == nil || !contract.CanAnswer || question == "" || strings.TrimSpace(contract.Answer) == "" {
		return nil, false
	}
	if contract.Confidence < helpcenterAnswerConfidenceFloor || helpcenterLinkPattern.MatchString(contract.Answer) {
		return nil, false
	}
	var cited []model.PublicHelpCenterArticleRef
	for _, source := range sources {
		if slices.Contains(contract.CitedSourceID, source.id) {
			cited = append(cited, model.PublicHelpCenterArticleRef{Title: source.title, URL: source.url})
		}
	}
	return cited, len(cited) > 0
}

// consumePreviewAnswerBudget counts one preview answer against the server's
// daily allowance. Without Redis the per-visitor rate limit is the only cap.
func (s *SupportQuickstartService) consumePreviewAnswerBudget(ctx context.Context) bool {
	if s.deps.Redis == nil {
		return true
	}
	key := "onb:answer:daily:" + time.Now().UTC().Format("20060102")
	count, err := s.deps.Redis.Incr(ctx, key).Result()
	if err != nil {
		s.logger.WarnContext(ctx, "preview answer budget check failed", "error", err)
		return true
	}
	if count == 1 {
		s.deps.Redis.Expire(ctx, key, 48*time.Hour)
	}
	return count <= quickstartAnswerDailyLimit
}

// cacheGet fills value from the cache and reports whether it was there.
func (s *SupportQuickstartService) cacheGet(ctx context.Context, key string, value any) bool {
	if s.deps.Redis == nil {
		return false
	}
	raw, err := s.deps.Redis.Get(ctx, key).Bytes()
	if err != nil {
		if !errors.Is(err, redis.Nil) {
			s.logger.WarnContext(ctx, "preview cache read failed", "error", err)
		}
		return false
	}
	return json.Unmarshal(raw, value) == nil
}

func (s *SupportQuickstartService) cacheSet(ctx context.Context, key string, value any, ttl time.Duration) {
	if s.deps.Redis == nil {
		return
	}
	raw, err := json.Marshal(value)
	if err != nil {
		s.logger.WarnContext(ctx, "preview cache encode failed", "error", err)
		return
	}
	if err := s.deps.Redis.Set(ctx, key, raw, ttl).Err(); err != nil {
		s.logger.WarnContext(ctx, "preview cache write failed", "error", err)
	}
}

// pageTitle names a website page the way people say it: "acme.com/pricing".
func pageTitle(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	return strings.TrimPrefix(parsed.Hostname(), "www.") + strings.TrimSuffix(parsed.Path, "/")
}
