package model

import "context"

// SlackRoute maps an opted-in Slack channel to its Helpin destination.
type SlackRoute struct {
	MailboxIDs        []string `json:"mailbox_ids,omitempty" gorm:"column:mailbox_ids;serializer:json;->;-:migration"`
	PublicTeamInboxes bool     `json:"public_team_inboxes,omitempty" gorm:"not null;default:false"`
	AccountID         string   `json:"-" gorm:"primaryKey"`
	ChannelID         string   `json:"channel_id" gorm:"primaryKey"`
	CurrentChannelID  string   `json:"-" gorm:"->;-:migration"`
	ChannelName       string   `json:"channel_name"`
	Purpose           string   `json:"purpose"`
	CRMCompanyID      *string  `json:"crm_company_id" gorm:"type:uuid"`
	MailboxID         *string  `json:"mailbox_id"`
	TeamID            *string  `json:"team_id"`
	EpicID            *string  `json:"epic_id"`
}

func (SlackRoute) TableName() string { return "slack_channel_routes" }

// SlackThread identifies the intended original thread for an explicitly linked task.
type SlackThread struct {
	InboxConversationID string `json:"-"`
	AccountID           string `json:"account"`
	Channel             string `json:"channel"`
	Root                string `json:"root"`
	Permalink           string `json:"permalink,omitempty"`
}

type slackSourceKey struct{}

func WithSlackSource(ctx context.Context, permalink string) context.Context {
	return context.WithValue(ctx, slackSourceKey{}, permalink)
}

func SlackSource(ctx context.Context) (string, bool) {
	url, ok := ctx.Value(slackSourceKey{}).(string)
	return url, ok
}

// SlackNotificationPreferences belong to the linked member of one installation.
type SlackNotificationPreferences struct {
	Overrides   JSONB           `json:"-" gorm:"column:notify_categories;type:jsonb"`
	Categories  map[string]bool `json:"categories,omitempty" gorm:"-"`
	AccountID   string          `json:"-"`
	SlackUser   string          `json:"-"`
	TeamID      string          `json:"team_id,omitempty"`
	BotUserID   string          `json:"bot_user_id,omitempty"`
	Linked      bool            `json:"linked"`
	Assignments bool            `json:"assignments"`
	Mentions    bool            `json:"mentions"`
}

type UpdateSlackNotificationPreferences struct {
	Categories  map[string]bool `json:"categories"`
	Assignments *bool           `json:"assignments"`
	Mentions    *bool           `json:"mentions"`
}

// Slack notification rows are independent within each module. Legacy switches
// supply defaults for their original event families until a row is customized.
var SlackCategoryDefaults = map[string]string{
	"projects.assignments": "assignments", "projects.mentions": "mentions",
	"projects.status_changes": "off", "projects.comments": "off", "projects.subscriptions": "off", "projects.sprints": "off",
	"docs.mentions": "mentions", "docs.updates": "off",
	"support.assignments": "assignments", "support.mentions": "mentions", "support.replies": "off",
	"crm.assignments": "assignments", "crm.mentions": "mentions", "crm.signals": "off", "crm.updates": "off",
	"agents.attention": "direct", "agents.activity": "off",
	"automation.attention": "direct", WorkflowFailureCategory: "off", "automation.activity": "off",
	"integrations.connection": "direct",
}

func (p SlackNotificationPreferences) CategoryEnabled(key string) bool {
	family, known := SlackCategoryDefaults[key]
	if !p.Linked || !known {
		return false
	}
	if value, ok := p.Overrides[key].(bool); ok {
		return value
	}
	switch family {
	case "assignments":
		return p.Assignments
	case "mentions":
		return p.Mentions
	default:
		return family == "direct"
	}
}
