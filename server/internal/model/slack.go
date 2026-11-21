package model

// SlackRoute maps an opted-in Slack channel to its Helpin destination.
type SlackRoute struct {
	AccountID        string  `json:"-" gorm:"primaryKey"`
	ChannelID        string  `json:"channel_id" gorm:"primaryKey"`
	CurrentChannelID string  `json:"-" gorm:"->;-:migration"`
	ChannelName      string  `json:"channel_name"`
	Purpose          string  `json:"purpose"`
	MailboxID        *string `json:"mailbox_id"`
	TeamID           *string `json:"team_id"`
	EpicID           *string `json:"epic_id"`
}

func (SlackRoute) TableName() string { return "slack_channel_routes" }

// SlackThread identifies the intended original thread for an explicitly linked task.
type SlackThread struct {
	AccountID string `json:"account"`
	Channel   string `json:"channel"`
	Root      string `json:"root"`
}
