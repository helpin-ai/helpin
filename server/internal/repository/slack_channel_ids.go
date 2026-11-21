package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Slack threads and queued jobs use RoutingID; only calls to Slack use CurrentID.
type slackChannelID struct {
	RoutingID string
	CurrentID string
}

func lookupSlackChannelID(db *gorm.DB, account, channel string) (slackChannelID, error) {
	var id slackChannelID
	err := db.Table("slack_channel_ids").Select("routing_id,current_id").Where("account_id=? AND channel_id=?", account, channel).Limit(1).Scan(&id).Error
	return id, err
}

func (r *SlackRepository) CurrentChannelID(ctx context.Context, account, channel string) (string, error) {
	id, err := lookupSlackChannelID(r.db.WithContext(ctx), account, channel)
	if id.CurrentID == "" {
		return channel, err
	}
	return id.CurrentID, err
}

// Serialize settings edits and Slack's ID transitions for this installation.
func lockSlackInstallation(tx *gorm.DB, account string) error {
	var row struct{ ID string }
	return tx.Table("support_channel_accounts").Select("id").Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND provider='slack'", account).Take(&row).Error
}

func (r *SlackRepository) ChangeChannelID(ctx context.Context, account, oldID, newID string) error {
	if oldID == "" || newID == "" || oldID == newID {
		return errors.New("invalid Slack channel ID change")
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockSlackInstallation(tx, account); err != nil {
			return err
		}
		old, err := lookupSlackChannelID(tx, account, oldID)
		if err != nil {
			return err
		}
		next, err := lookupSlackChannelID(tx, account, newID)
		if err != nil {
			return err
		}
		// A replay may arrive after a later channel ID change.
		if old.RoutingID != "" && old.RoutingID == next.RoutingID {
			return nil
		}
		if old.RoutingID != "" && (old.CurrentID != oldID || next.RoutingID != "") {
			return errors.New("conflicting Slack channel identities")
		}
		routing, current := old.RoutingID, newID
		if next.RoutingID != "" {
			routing, current = next.RoutingID, next.CurrentID
		}
		if routing == "" {
			routing = oldID
		}
		if err := tx.Exec("UPDATE slack_channel_ids SET current_id=? WHERE account_id=? AND routing_id=?", current, account, routing).Error; err != nil {
			return err
		}
		return tx.Exec(`INSERT INTO slack_channel_ids(account_id,channel_id,routing_id,current_id)
			VALUES(?,?,?,?),(?,?,?,?) ON CONFLICT(account_id,channel_id) DO NOTHING`, account, oldID, routing, current, account, newID, routing, current).Error
	})
}
