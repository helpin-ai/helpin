package service

import (
	"context"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/helpin-ai/helpin/server/internal/model"
)

const quickstartPreviewTimeout = 90 * time.Second

// quickstartCachedPreview is a finished preview, kept so the next visitor to
// the same site gets the same answers without another AI call.
type quickstartCachedPreview struct {
	Articles []model.PublicHelpCenterArticleRef `json:"articles"`
	Preview  *model.SupportQuickstartPreview    `json:"preview"`
}

// Preview shows a visitor, before any account exists, what the support agent
// would do with their company's public pages. It reads the site, finds and
// lists the help center, reads articles from different collections, answers the
// questions customers most likely ask, and names common questions no article
// covers. emit receives each step's log lines and result as they happen, one
// call at a time. Only an address the person can fix is returned as an error,
// before anything is emitted.
func (s *SupportQuickstartService) Preview(ctx context.Context, req model.SupportQuickstartPreviewRequest, emit func(model.SupportQuickstartEvent)) error {
	website, helpOverride, err := normalizePreviewAddresses(req.WebsiteURL, req.HelpCenterURL)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, quickstartPreviewTimeout)
	defer cancel()
	// Articles are read side by side, so their log lines arrive together.
	var mu sync.Mutex
	send := func(event model.SupportQuickstartEvent) {
		mu.Lock()
		defer mu.Unlock()
		emit(event)
	}
	note := func(step string) func(string) {
		return func(text string) {
			send(model.SupportQuickstartEvent{Type: model.QuickstartEventLog, Step: step, Text: text})
		}
	}

	note(model.QuickstartStepSite)("Opening " + pageTitle(website))
	site, linked, err := s.readSite(ctx, website)
	if err != nil {
		return err
	}
	// Sent before the help center is known, so the brand shows right away.
	send(model.SupportQuickstartEvent{Type: model.QuickstartEventSite, Site: site})

	helpCenter := site.HelpCenter
	switch {
	case helpOverride != "":
		helpCenter = &model.SupportQuickstartHelpCenter{URL: helpOverride, Platform: model.QuickstartHelpCenterWebsite}
	case helpCenter == nil:
		note(model.QuickstartStepHelpCenter)("Looking for your help center")
		helpCenter = s.findHelpCenter(ctx, website, linked)
	}
	helpURL := ""
	var listing *quickstartListing
	if helpCenter != nil {
		helpURL = helpCenter.URL
		// An address the person typed is tried too: it may be a help center
		// whose articles can be listed.
		if helpOverride != "" || helpCenter.Platform != model.QuickstartHelpCenterWebsite {
			note(model.QuickstartStepHelpCenter)("Counting the articles on " + pageTitle(helpURL))
			listing = s.helpCenterListing(ctx, helpURL)
		}
	}
	found := model.SupportQuickstartEvent{Type: model.QuickstartEventHelpCenter, HelpCenterURL: helpURL}
	if listing != nil {
		found.HelpCenter = listing.Preview
	}
	send(found)

	if !s.previewAnswersAvailable() {
		send(model.SupportQuickstartEvent{Type: model.QuickstartEventPreview, Preview: &model.SupportQuickstartPreview{}})
		return nil
	}
	// The number changes when cached answers would no longer pass the current checks.
	cacheKey := "onb:preview:3:" + aiUsageStableHash(strings.TrimRight(website, "/")+"|"+strings.TrimRight(helpURL, "/"))
	var cached quickstartCachedPreview
	if s.cacheGet(ctx, cacheKey, &cached) && cached.Preview != nil {
		send(model.SupportQuickstartEvent{Type: model.QuickstartEventArticles, Articles: cached.Articles})
		send(model.SupportQuickstartEvent{Type: model.QuickstartEventPreview, Preview: cached.Preview})
		return nil
	}

	read := func(title string) { note(model.QuickstartStepArticles)("Read “" + title + "”") }
	var sources []quickstartSource
	// What every article is called and about, to tell which questions none covers.
	var titles, subjects []string
	if listing != nil {
		sources = s.readArticles(ctx, listing.Preview.Source, pickPreviewArticles(listing.Refs, ""), read)
		for _, ref := range listing.Refs {
			titles = append(titles, ref.Title)
			subjects = append(subjects, ref.Title+" "+ref.Summary)
		}
	} else {
		sources = s.readPages(ctx, previewPages(website, helpURL), read)
	}
	articles := make([]model.PublicHelpCenterArticleRef, 0, len(sources))
	for _, source := range sources {
		articles = append(articles, model.PublicHelpCenterArticleRef{Title: source.title, URL: source.url})
	}
	send(model.SupportQuickstartEvent{Type: model.QuickstartEventArticles, Articles: articles})

	preview := &model.SupportQuickstartPreview{Available: true, Answers: []model.SupportQuickstartAnswer{}, Gaps: []string{}}
	if len(sources) == 0 {
		send(model.SupportQuickstartEvent{Type: model.QuickstartEventPreview, Preview: preview})
		return nil
	}
	fail := func(text string) {
		send(model.SupportQuickstartEvent{Type: model.QuickstartEventFailed, Step: model.QuickstartStepAnswers, Text: text})
	}
	if !s.consumePreviewAnswerBudget(ctx) {
		fail("So many people are trying Helpin right now that previews are paused. Create your account to keep going.")
		return nil
	}
	note(model.QuickstartStepAnswers)("Writing answers to the questions your customers ask")
	contract, err := s.generatePreview(ctx, site.CompanyName, "", sources, titles)
	if err != nil {
		s.logger.WarnContext(ctx, "support quickstart preview answers", "error", err)
		fail("Your agent couldn’t answer just now.")
		return nil
	}
	preview.Answers = validPreviewAnswers(contract.Answers, sources)
	preview.Gaps = validPreviewGaps(contract.Gaps, subjects)
	if len(preview.Answers) > 0 {
		s.cacheSet(ctx, cacheKey, quickstartCachedPreview{Articles: articles, Preview: preview}, quickstartPreviewCacheTTL)
	}
	send(model.SupportQuickstartEvent{Type: model.QuickstartEventPreview, Preview: preview})
	return nil
}

// validPreviewAnswers keeps the agent's grounded answers, one per question.
func validPreviewAnswers(contracts []quickstartAnswerContract, sources []quickstartSource) []model.SupportQuickstartAnswer {
	answers := []model.SupportQuickstartAnswer{}
	for i := range contracts {
		question := strings.Join(strings.Fields(contracts[i].Question), " ")
		cited, ok := validatePreviewAnswer(&contracts[i], question, sources)
		if !ok || slices.ContainsFunc(answers, func(a model.SupportQuickstartAnswer) bool { return strings.EqualFold(a.Question, question) }) {
			continue
		}
		answers = append(answers, model.SupportQuickstartAnswer{
			Available: true, Question: question, CanAnswer: true, Answer: strings.TrimSpace(contracts[i].Answer), Sources: cited,
		})
		if len(answers) == quickstartPreviewAnswers {
			break
		}
	}
	return answers
}

// validPreviewGaps keeps the questions no article covers, going by each
// article's title and summary. A question whose subject shows up in one is
// dropped: naming a gap that isn't one is worse than missing one.
func validPreviewGaps(gaps, subjects []string) []string {
	kept := []string{}
	if len(subjects) == 0 {
		return kept
	}
	for _, gap := range gaps {
		gap = strings.Join(strings.Fields(gap), " ")
		if gap == "" || utf8.RuneCountInString(gap) > quickstartPreviewGapMax || helpcenterLinkPattern.MatchString(gap) || slices.Contains(kept, gap) {
			continue
		}
		words := previewQuestionWords(gap)
		// An article sharing half of the question's subject words covers it.
		needed := (len(words) + 1) / 2
		if needed == 0 || slices.ContainsFunc(subjects, func(subject string) bool { return sharedWords(subject, words) >= needed }) {
			continue
		}
		kept = append(kept, gap)
		if len(kept) == quickstartPreviewGaps {
			break
		}
	}
	return kept
}
