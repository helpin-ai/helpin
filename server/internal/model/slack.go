package model

import "context"

// SlackRoute maps an opted-in Slack channel to its Helpin destination.
type SlackRoute struct {
	MailboxIDs       []string `json:"mailbox_ids,omitempty" gorm:"column:mailbox_ids;serializer:json;->;-:migration"`
	AccountID        string   `json:"-" gorm:"primaryKey"`
	ChannelID        string   `json:"channel_id" gorm:"primaryKey"`
	CurrentChannelID string   `json:"-" gorm:"->;-:migration"`
	ChannelName      string   `json:"channel_name"`
	Purpose          string   `json:"purpose"`
	CRMCompanyID     *string  `json:"crm_company_id" gorm:"type:uuid"`
	MailboxID        *string  `json:"mailbox_id"`
	TeamID           *string  `json:"team_id"`
	EpicID           *string  `json:"epic_id"`
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
	AccountID   string `json:"-"`
	SlackUser   string `json:"-"`
	TeamID      string `json:"team_id,omitempty"`
	BotUserID   string `json:"bot_user_id,omitempty"`
	Linked      bool   `json:"linked"`
	Assignments bool   `json:"assignments"`
	Mentions    bool   `json:"mentions"`
}

type UpdateSlackNotificationPreferences struct {
	Assignments *bool `json:"assignments"`
	Mentions    *bool `json:"mentions"`
}
