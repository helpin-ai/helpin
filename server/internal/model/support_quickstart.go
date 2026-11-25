package model

// QuickstartHelpCenterWebsite is the platform of a help center whose
// articles cannot be copied one by one; it is crawled for the AI like the
// website. Every other platform, such as "intercom" or "zendesk", names a
// help center that is copied article by article.
const QuickstartHelpCenterWebsite = "website"

// Answer modes a support quickstart can turn on for the AI agent.
const (
	QuickstartAIModeAnswer = "answer"
	QuickstartAIModeDraft  = "draft"
	QuickstartAIModeOff    = "off"
)

// SupportQuickstartDetectRequest asks what support setup a website suggests.
type SupportQuickstartDetectRequest struct {
	WebsiteURL string `json:"website_url"`
}

// SupportQuickstartHelpCenter is a help center found for a website.
type SupportQuickstartHelpCenter struct {
	URL      string `json:"url"`
	Name     string `json:"name,omitempty"`
	Platform string `json:"platform"`
}

// SupportQuickstartDetection is what a company website shows about the
// company and its help center. Nothing is saved.
type SupportQuickstartDetection struct {
	WebsiteURL  string `json:"website_url"`
	CompanyName string `json:"company_name"`
	// WebsiteReadable is false when the site could not be read; the setup
	// still works, with less filled in.
	WebsiteReadable bool                         `json:"website_readable"`
	LogoURL         string                       `json:"logo_url,omitempty"`
	FaviconURL      string                       `json:"favicon_url,omitempty"`
	BrandColor      string                       `json:"brand_color,omitempty"`
	HelpCenter      *SupportQuickstartHelpCenter `json:"help_center,omitempty"`
}

// SupportQuickstartStartRequest sets up support for a workspace.
type SupportQuickstartStartRequest struct {
	WebsiteURL    string `json:"website_url"`
	CompanyName   string `json:"company_name"`
	HelpCenterURL string `json:"help_center_url"`
	// AIMode is "answer" (the AI replies to customers) or "draft" (it writes
	// private suggestions for the team). Empty means "answer".
	AIMode string `json:"ai_mode"`
}

// SupportQuickstartArticles is the help center copy's progress.
type SupportQuickstartArticles struct {
	// Status is "importing", "done" or "failed".
	Status   string `json:"status"`
	Imported int    `json:"imported"`
	Total    int    `json:"total"`
	Failed   int    `json:"failed"`
	// Current is the article copied most recently, while the copy runs.
	Current string `json:"current,omitempty"`
}

// SupportQuickstartSource is a site crawled as knowledge for the AI.
type SupportQuickstartSource struct {
	Name         string `json:"name"`
	URL          string `json:"url"`
	Status       string `json:"status"`
	IndexedPages int    `json:"indexed_pages"`
}

// SupportQuickstartStatus is how far a workspace's support setup has come.
type SupportQuickstartStatus struct {
	Started  bool                       `json:"started"`
	Articles *SupportQuickstartArticles `json:"articles,omitempty"`
	Sources  []SupportQuickstartSource  `json:"sources"`
	// AgentReady is true once the AI has indexed knowledge to answer from.
	AgentReady bool `json:"agent_ready"`
	// Done is true when nothing is still importing or crawling.
	Done   bool   `json:"done"`
	AIMode string `json:"ai_mode"`
}

// SupportQuickstartPreviewRequest asks for the public onboarding preview of a
// company website.
type SupportQuickstartPreviewRequest struct {
	WebsiteURL string `json:"website_url"`
	// HelpCenterURL replaces the help center found on the website.
	HelpCenterURL string `json:"help_center_url,omitempty"`
}

// The kinds of event a preview sends, and the steps they belong to.
const (
	// QuickstartEventLog is one line of work in progress on a step.
	QuickstartEventLog = "log"
	// QuickstartEventFailed ends a step that could not finish; without a
	// step, the whole preview stopped.
	QuickstartEventFailed     = "failed"
	QuickstartEventSite       = "site"
	QuickstartEventHelpCenter = "help_center"
	QuickstartEventArticles   = "articles"
	QuickstartEventPreview    = "preview"

	QuickstartStepSite       = "site"
	QuickstartStepHelpCenter = "help_center"
	QuickstartStepArticles   = "articles"
	QuickstartStepAnswers    = "answers"
)

// SupportQuickstartEvent is one message of the preview as it runs. Each step
// sends log lines while it works, then one event carrying its result.
type SupportQuickstartEvent struct {
	Type string `json:"type"`
	Step string `json:"step,omitempty"`
	// Text is a log line or a failure message, safe to show.
	Text string `json:"text,omitempty"`
	// Site is the website's name and branding. Its help center is set only
	// when the address given was the help center itself; the help center
	// found afterwards arrives in HelpCenterURL.
	Site *SupportQuickstartDetection `json:"site,omitempty"`
	// HelpCenterURL is the help center in use; empty when there is none.
	HelpCenterURL string `json:"help_center_url,omitempty"`
	// HelpCenter holds the counts of a help center whose articles were listed.
	HelpCenter *PublicHelpCenterImportPreview `json:"help_center,omitempty"`
	// Articles are the articles or pages the agent read.
	Articles []PublicHelpCenterArticleRef `json:"articles,omitempty"`
	Preview  *SupportQuickstartPreview    `json:"preview,omitempty"`
}

// SupportQuickstartPreview is the agent's work on a company's public pages:
// customer questions it answered, and questions it found no article for.
type SupportQuickstartPreview struct {
	// Available is false when this server has no AI to answer with.
	Available bool                      `json:"available"`
	Answers   []SupportQuickstartAnswer `json:"answers"`
	// Gaps are common customer questions no article title covers. They are
	// only looked for when the help center's articles could be listed.
	Gaps []string `json:"gaps"`
}

// SupportQuickstartAnswerRequest asks the support agent, before any account
// exists, to answer one customer question from a company's public pages.
type SupportQuickstartAnswerRequest struct {
	WebsiteURL    string `json:"website_url"`
	HelpCenterURL string `json:"help_center_url,omitempty"`
	Question      string `json:"question"`
}

// SupportQuickstartAnswer is the agent's preview answer.
type SupportQuickstartAnswer struct {
	// Available is false when this server has no AI to answer with.
	Available bool   `json:"available"`
	Question  string `json:"question"`
	// CanAnswer is false when the pages read don't cover the question.
	CanAnswer bool                         `json:"can_answer"`
	Answer    string                       `json:"answer,omitempty"`
	Sources   []PublicHelpCenterArticleRef `json:"sources"`
}
