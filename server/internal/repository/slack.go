package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SlackRepository stores installation grants and channel routing.
type SlackRepository struct{ db *gorm.DB }

// NewSlackRepository returns the workspace Slack store.
func NewSlackRepository(db *gorm.DB) *SlackRepository { return &SlackRepository{db: db} }

// SaveState records a short-lived browser grant.
func (r *SlackRepository) SaveState(ctx context.Context, hash, workspace, user string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("DELETE FROM slack_install_states WHERE expires_at <= ?", time.Now()).Error; err != nil {
			return err
		}
		return tx.Exec("INSERT INTO slack_install_states(state_hash,workspace_id,user_id,expires_at) VALUES (?,?,?,?)", hash, workspace, user, time.Now().Add(10*time.Minute)).Error
	})
}

// ConsumeState validates the grant against the currently authenticated installer.
func (r *SlackRepository) ConsumeState(ctx context.Context, hash, workspace, user string) (bool, error) {
	q := r.db.WithContext(ctx).Exec("DELETE FROM slack_install_states WHERE state_hash=? AND workspace_id=? AND user_id=? AND expires_at>?", hash, workspace, user, time.Now())
	return q.RowsAffected == 1, q.Error
}

// Installation finds the active Slack installation in one workspace.
func (r *SlackRepository) Installation(ctx context.Context, workspace string) (*model.SupportChannelAccount, error) {
	var a model.SupportChannelAccount
	err := r.db.WithContext(ctx).Where("workspace_id=? AND provider='slack' AND disabled_at IS NULL", workspace).Limit(1).Find(&a).Error
	if err != nil || a.ID == "" {
		return nil, err
	}
	return &a, nil
}

// CreateInstallation initializes teammate AI from the workspace's shared model only on first install.
func (r *SlackRepository) CreateInstallation(ctx context.Context, a *model.SupportChannelAccount) error {
	settings, err := NewAIProfileRepository(r.db).Settings(ctx, a.WorkspaceID)
	if err != nil {
		return err
	}
	a.AIRepliesDisabled = settings.DefaultProfileID == nil
	return NewSupportChannelAccountRepository(r.db).Create(ctx, a)
}

// PreviousInstallation preserves thread history only when both workspaces match.
func (r *SlackRepository) PreviousInstallation(ctx context.Context, workspace, team string) (*model.SupportChannelAccount, error) {
	var a model.SupportChannelAccount
	err := r.db.WithContext(ctx).Where("workspace_id=? AND provider='slack' AND external_account_id=? AND disabled_at IS NOT NULL", workspace, team).Order("disabled_at DESC").Limit(1).Find(&a).Error
	if err != nil || a.ID == "" {
		return nil, err
	}
	return &a, nil
}

func (r *SlackRepository) Reactivate(ctx context.Context, a *model.SupportChannelAccount, token, handle, name, user string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&model.SupportChannelAccount{}).Where("id=? AND workspace_id=? AND disabled_at IS NOT NULL", a.ID, a.WorkspaceID).
			Updates(map[string]any{"disabled_at": nil, "status": "active", "token_encrypted": token, "handle": handle, "display_name": name, "connected_by": user, "auth_error_count": 0})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		// The admin opts channels back in and teammates renew their links.
		for _, table := range []string{"slack_channel_routes", "slack_task_drafts", "slack_link_grants", "slack_members"} {
			if err := tx.Exec("DELETE FROM "+table+" WHERE account_id=?", a.ID).Error; err != nil {
				return err
			}
		}
		return tx.Exec("UPDATE support_inbound_jobs SET status='completed',payload='',lease_token='' WHERE channel_account_id=? AND status IN ('pending','processing','failed')", a.ID).Error
	})
}

// Routes lists the installation's enabled channels.
func (r *SlackRepository) Routes(ctx context.Context, account string) ([]model.SlackRoute, error) {
	rows := make([]model.SlackRoute, 0)
	err := slackRoutes(r.db.WithContext(ctx), account).Order("r.channel_name").Scan(&rows).Error
	return rows, err
}

func slackRoutes(db *gorm.DB, account string) *gorm.DB {
	aggregate := "jsonb_agg(f.inbox_key ORDER BY f.inbox_key)"
	if db.Dialector.Name() == "sqlite" {
		aggregate = "json_group_array(f.inbox_key)"
	}
	return db.Table("slack_channel_routes r").Select("r.*, (SELECT "+aggregate+" FROM slack_feed_inboxes f WHERE f.account_id=r.account_id AND f.channel_id=r.channel_id) AS mailbox_ids, COALESCE(ids.current_id,r.channel_id) AS current_channel_id").
		Joins("LEFT JOIN slack_channel_ids ids ON ids.account_id=r.account_id AND ids.channel_id=r.channel_id").
		Where("r.account_id=?", account)
}

// Route returns a channel's opt-in record.
func (r *SlackRepository) Route(ctx context.Context, account, channel string) (*model.SlackRoute, error) {
	var row model.SlackRoute
	err := slackRoutes(r.db.WithContext(ctx), account).
		Where("r.channel_id=COALESCE((SELECT routing_id FROM slack_channel_ids WHERE account_id=? AND channel_id=?),?)", account, channel, channel).
		Limit(1).Scan(&row).Error
	if row.AccountID == "" || err != nil {
		return nil, err
	}
	return &row, nil
}

// EffectiveRoute supplies on-demand behavior for an invited channel without saving a rule.
func (r *SlackRepository) EffectiveRoute(ctx context.Context, account, channel string) (*model.SlackRoute, error) {
	rule, err := r.Route(ctx, account, channel)
	if err != nil || rule != nil {
		return rule, err
	}
	id, err := lookupSlackChannelID(r.db.WithContext(ctx), account, channel)
	if err != nil {
		return nil, err
	}
	if id.RoutingID == "" {
		id.RoutingID, id.CurrentID = channel, channel
	}
	return &model.SlackRoute{AccountID: account, ChannelID: id.RoutingID, CurrentChannelID: id.CurrentID, Purpose: "on_request"}, nil
}

var ErrSlackInboxAlreadyShared = errors.New("an inbox already has a Slack channel rule")

// SetRoute updates the validated destination for one channel.
func (r *SlackRepository) SetRoute(ctx context.Context, row model.SlackRoute, preserveCompany bool) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockSlackInstallation(tx, row.AccountID); err != nil {
			return err
		}
		if err := tx.Exec("INSERT INTO slack_channel_ids(account_id,channel_id,routing_id,current_id) VALUES(?,?,?,?) ON CONFLICT DO NOTHING", row.AccountID, row.ChannelID, row.ChannelID, row.ChannelID).Error; err != nil {
			return err
		}
		id, err := lookupSlackChannelID(tx, row.AccountID, row.ChannelID)
		if err != nil {
			return err
		}
		row.ChannelID = id.RoutingID
		columns := []string{"channel_name", "purpose", "mailbox_id", "team_id", "epic_id", "public_team_inboxes"}
		if preserveCompany {
			row.CRMCompanyID = nil
		} else {
			columns = append(columns, "crm_company_id")
		}
		if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "account_id"}, {Name: "channel_id"}}, DoUpdates: clause.AssignmentColumns(columns)}).Create(&row).Error; err != nil {
			return err
		}
		if err := tx.Exec("DELETE FROM slack_feed_inboxes WHERE account_id=? AND channel_id=?", row.AccountID, row.ChannelID).Error; err != nil {
			return err
		}
		if row.Purpose == "inbox" {
			var conflicting string
			if err := tx.Raw("SELECT r.channel_name FROM slack_feed_inboxes f JOIN slack_channel_routes r USING(account_id,channel_id) WHERE f.account_id=? AND f.inbox_key IN ? LIMIT 1", row.AccountID, row.MailboxIDs).Scan(&conflicting).Error; err != nil {
				return err
			}
			if conflicting != "" {
				return fmt.Errorf("%w in #%s", ErrSlackInboxAlreadyShared, conflicting)
			}
			for _, id := range row.MailboxIDs {
				if err := tx.Exec("INSERT INTO slack_feed_inboxes(account_id,channel_id,inbox_key) VALUES(?,?,?)", row.AccountID, row.ChannelID, id).Error; err != nil {
					return err
				}
			}
		}
		return tx.Exec(`UPDATE slack_feed_threads t SET active=false FROM support_conversations c WHERE c.id=t.conversation_id AND t.account_id=? AND t.channel_id=? AND NOT EXISTS(SELECT 1 FROM slack_feed_inboxes f WHERE f.account_id=t.account_id AND f.channel_id=t.channel_id AND f.inbox_key=coalesce(c.mailbox_id::text,'shared'))`, row.AccountID, row.ChannelID).Error
	})
}

// RemoveRoute restores on-demand intake and removes channel defaults.
func (r *SlackRepository) RemoveRoute(ctx context.Context, account, channel string) error {
	return r.db.WithContext(ctx).Where("account_id=? AND channel_id=COALESCE((SELECT routing_id FROM slack_channel_ids WHERE account_id=? AND channel_id=?),?)", account, account, channel, channel).Delete(&model.SlackRoute{}).Error
}

// Member finds a linked Helpin identity, scoped to this Slack installation.
func (r *SlackRepository) Member(ctx context.Context, account, user string) (string, error) {
	var id string
	err := r.db.WithContext(ctx).Table("slack_members").Select("user_id").Where("account_id=? AND slack_user=?", account, user).Scan(&id).Error
	return id, err
}

// MatchMember only inserts a link when the verified email identifies exactly one active member.
func (r *SlackRepository) MatchMember(ctx context.Context, a *model.SupportChannelAccount, user, email string) (string, error) {
	err := r.db.WithContext(ctx).Exec(`INSERT INTO slack_members(account_id,slack_user,user_id)
    SELECT ?,?,u.id FROM users u JOIN workspace_members m ON m.user_id=u.id
    WHERE m.workspace_id=? AND m.status='active' AND u.email_verified_at IS NOT NULL AND lower(u.email)=lower(?)
    AND (SELECT count(*) FROM users v JOIN workspace_members n ON n.user_id=v.id
         WHERE n.workspace_id=m.workspace_id AND n.status='active' AND v.email_verified_at IS NOT NULL AND lower(v.email)=lower(u.email))=1
    ON CONFLICT DO NOTHING`, a.ID, user, a.WorkspaceID, email).Error
	if err != nil {
		return "", err
	}
	return r.Member(ctx, a.ID, user)
}

// GrantLink records an expiring link supplied privately to the Slack user.
func (r *SlackRepository) GrantLink(ctx context.Context, account, user, hash string, note *model.SupportInboundJob) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		expires := time.Now().Add(10 * time.Minute)
		if note != nil {
			note.AvailableAt = expires
			if err := NewSupportInboundJobRepository(tx).Create(ctx, note); err != nil {
				return err
			}
		}
		if err := tx.Exec("DELETE FROM slack_link_grants WHERE expires_at<=?", time.Now()).Error; err != nil {
			return err
		}
		return tx.Exec("INSERT INTO slack_link_grants(token_hash,account_id,slack_user,expires_at) VALUES (?,?,?,?)", hash, account, user, expires).Error
	})
}

// ResumeFeedNotes releases only notes from this linked sender and installation.
func (r *SlackRepository) ResumeFeedNotes(ctx context.Context, account, slackUser string) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&model.SupportInboundJob{}).
		Where("channel_account_id = ? AND subject_key = ? AND status = ? AND available_at > ?",
			account, model.SupportChannelSubjectKey(account, slackUser), model.InboundAwaitingSlackLink, now).
		Updates(map[string]any{"status": "pending", "available_at": now}).Error
}

// Link consumes the grant and binds the two verified identities.
func (r *SlackRepository) Link(ctx context.Context, account, user, hash string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var slackUser string
		if err := tx.Raw("DELETE FROM slack_link_grants WHERE token_hash=? AND account_id=? AND expires_at>? RETURNING slack_user", hash, account, time.Now()).Scan(&slackUser).Error; err != nil {
			return err
		}
		if slackUser == "" {
			return gorm.ErrRecordNotFound
		}
		result := tx.Exec("INSERT INTO slack_members(account_id,slack_user,user_id) VALUES (?,?,?) ON CONFLICT (account_id,slack_user) DO UPDATE SET user_id=excluded.user_id WHERE slack_members.user_id=excluded.user_id", account, slackUser, user)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return gorm.ErrDuplicatedKey
		}
		return NewSlackRepository(tx).ResumeFeedNotes(ctx, account, slackUser)
	})
}

// ThreadTask looks up the PM record already linked to this discussion.
func (r *SlackRepository) ThreadTask(ctx context.Context, account, channel, root string) (string, error) {
	var id string
	err := r.db.WithContext(ctx).Table("slack_task_threads").Select("task_id").Where("account_id=? AND channel_id=? AND thread_ts=?", account, channel, root).Scan(&id).Error
	return id, err
}

// LinkTask establishes a thread link after the caller checks the task audience.
func (r *SlackRepository) LinkTask(ctx context.Context, account, channel, root, id, actor string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		source, _ := model.SlackSource(ctx)
		return bindSlackTask(ctx, tx, model.SlackThread{AccountID: account, Channel: channel, Root: root, Permalink: source}, id, actor)
	})
}

func bindSlackTask(ctx context.Context, tx *gorm.DB, thread model.SlackThread, id, actor string) error {
	if err := tx.WithContext(ctx).Exec("INSERT INTO slack_task_threads(account_id,channel_id,thread_ts,task_id) VALUES(?,?,?,?)", thread.AccountID, thread.Channel, thread.Root, id).Error; err != nil {
		return err
	}
	if strings.HasPrefix(thread.Channel, "D") {
		return nil
	} // DMs have no shared task discussion.
	var route model.SlackRoute
	if err := tx.Clauses(clause.Locking{Strength: "SHARE"}).Where("account_id=? AND channel_id=?", thread.AccountID, thread.Channel).Limit(1).Find(&route).Error; err != nil {
		return err
	}
	if thread.InboxConversationID != "" {
		if route.Purpose != "inbox" {
			return errors.New("the Slack inbox route changed during review")
		}
		// A linked support task gets a single receipt. Only PM routes create shared discussions.
		receipt := tx.Exec(`INSERT INTO support_inbound_jobs(id,kind,workspace_id,channel_account_id,payload,status,available_at)
 SELECT gen_random_uuid(),'slack_pm_delivery',c.workspace_id,t.account_id,
 json_build_object('inbox',t.id,'task',?::text)::text,'pending',now()
 FROM slack_feed_threads t JOIN support_conversations c ON c.id=t.conversation_id
 WHERE t.active AND t.account_id=? AND t.channel_id=? AND t.root_ts=? AND c.id=?
 AND c.status<>'spam' AND c.anonymized_at IS NULL AND c.merged_into_id IS NULL
 AND EXISTS(SELECT 1 FROM slack_feed_inboxes f WHERE f.account_id=t.account_id AND f.channel_id=t.channel_id AND f.inbox_key=coalesce(c.mailbox_id::text,'shared'))`, id, thread.AccountID, thread.Channel, thread.Root, thread.InboxConversationID)
		if receipt.Error != nil {
			return receipt.Error
		}
		if receipt.RowsAffected != 1 {
			return errors.New("the Slack inbox conversation changed during review")
		}
		return nil
	}
	if route.Purpose != "pm" {
		return nil
	}
	root := model.PMComment{EntityType: "task", EntityID: id, AuthorID: actor, Body: "Replies in this discussion are shared with #" + route.ChannelName + "."}
	if thread.Permalink != "" {
		root.SlackPermalink = &thread.Permalink
	}
	if err := tx.Create(&root).Error; err != nil {
		return err
	}
	if err := tx.Exec("UPDATE pm_comments SET slack_discussion_channel=? WHERE id=?", route.ChannelName, root.ID).Error; err != nil {
		return err
	}
	if err := tx.Exec("UPDATE slack_task_threads SET comment_id=? WHERE account_id=? AND channel_id=? AND thread_ts=?", root.ID, thread.AccountID, thread.Channel, thread.Root).Error; err != nil {
		return err
	}
	return tx.Exec(`INSERT INTO support_inbound_jobs(id,kind,workspace_id,channel_account_id,payload,status,available_at)
 SELECT gen_random_uuid(),'slack_pm_delivery',workspace_id,?,json_build_object('task',?::text,'channel',?::text,'root',?::text)::text,'pending',now() FROM pm_tasks WHERE id=?`, thread.AccountID, id, thread.Channel, thread.Root, id).Error
}

// BindSlackTask links the discussion inside the task creation transaction.
func (r *PMTaskRepository) BindSlackTask(ctx context.Context, thread model.SlackThread, id, actor string) error {
	return bindSlackTask(ctx, r.db, thread, id, actor)
}

// EpicOptions returns active epics for team-scoped selection.
func (r *SlackRepository) EpicOptions(ctx context.Context, ws string) ([]model.PMEpic, error) {
	var rows []model.PMEpic
	err := r.db.WithContext(ctx).Select("id,name,team_id").Where("workspace_id=? AND archived=false", ws).Order("name,id").Find(&rows).Error
	return rows, err
}

// SaveDraft stores a short-lived review scoped to the member and source.
func (r *SlackRepository) SaveDraft(ctx context.Context, id, account, user string, conversation *string, content []byte) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("DELETE FROM slack_task_drafts WHERE expires_at<=?", time.Now()).Error; err != nil {
			return err
		}
		return tx.Exec("INSERT INTO slack_task_drafts(id,account_id,user_id,conversation_id,content,expires_at) VALUES(?,?,?,?,?::jsonb,?)", id, account, user, conversation, string(content), time.Now().Add(15*time.Minute)).Error
	})
}

// Draft retrieves one member's unexpired review.
func (r *SlackRepository) Draft(ctx context.Context, id, account, user string) ([]byte, error) {
	var content string
	err := r.db.WithContext(ctx).Table("slack_task_drafts").Select("content").Where("id=? AND account_id=? AND user_id=? AND expires_at>?", id, account, user, time.Now()).Scan(&content).Error
	return []byte(content), err
}

// SupportThread resolves an accessible live source record by thread identity.
func (r *SlackRepository) SupportThread(ctx context.Context, a *model.SupportChannelAccount, key string) (*model.SupportConversation, error) {
	c, err := r.SupportThreadOrigin(ctx, a, key)
	if c != nil && c.MergedIntoID != nil {
		return nil, err
	}
	return c, err
}

// SupportThreadOrigin includes merged sources so customer status is scoped to the original thread.
func (r *SlackRepository) SupportThreadOrigin(ctx context.Context, a *model.SupportChannelAccount, key string) (*model.SupportConversation, error) {
	var c model.SupportConversation
	err := r.db.WithContext(ctx).Where("workspace_id=? AND channel_account_id=? AND provider_thread_key=? AND anonymized_at IS NULL AND status<>'spam'", a.WorkspaceID, a.ID, key).Limit(1).Find(&c).Error
	if err != nil || c.ID == "" {
		return nil, err
	}
	return &c, nil
}

// PublishedReply reads only outbound messages confirmed in this thread.
func (r *SlackRepository) PublishedReply(ctx context.Context, a *model.SupportChannelAccount, c string) (model.SupportMessage, error) {
	var m model.SupportMessage
	err := r.db.WithContext(ctx).Model(&m).Where("workspace_id=? AND is_internal=false AND message_type='reply' AND sender_type IN ('user','agent','ai') AND deleted_at IS NULL", a.WorkspaceID).
		Where("(channel_account_id=? AND external_message_key IS NOT NULL AND COALESCE(origin_conversation_id,conversation_id)=?) OR EXISTS (SELECT 1 FROM support_channel_outbox o WHERE o.message_id=support_messages.id AND o.channel_account_id=? AND o.dispatch_status='accepted' AND coalesce(o.provider_origin_id,o.conversation_id)=?)", a.ID, c, a.ID, c).
		Order("created_at DESC, id DESC").Limit(1).Find(&m).Error
	return m, err
}

// RecordUpdateRequest claims the owner alert once per published update.
func (r *SlackRepository) RecordUpdateRequest(ctx context.Context, c, key string) (bool, error) {
	q := r.db.WithContext(ctx).Exec("INSERT INTO slack_update_requests(conversation_id,published_key) VALUES(?,?) ON CONFLICT(conversation_id) DO UPDATE SET published_key=excluded.published_key WHERE slack_update_requests.published_key<>excluded.published_key", c, key)
	return q.RowsAffected == 1, q.Error
}

// SlackDiscussion holds the PM comment and Slack card for one thread.
type SlackDiscussion struct{ TaskID, CommentID, CardTS string }

// Discussion looks up the current task binding.
func (r *SlackRepository) Discussion(ctx context.Context, a, channel, root string) (SlackDiscussion, error) {
	var d SlackDiscussion
	err := r.db.WithContext(ctx).Table("slack_task_threads").Select("task_id,coalesce(comment_id::text,'') AS comment_id,coalesce(card_ts,'') AS card_ts").Where("account_id=? AND channel_id=? AND thread_ts=?", a, channel, root).Scan(&d).Error
	return d, err
}

// RecordCard remembers the provider timestamp so changes update the original card.
func (r *SlackRepository) RecordCard(ctx context.Context, a, channel, root, ts string) error {
	return r.db.WithContext(ctx).Table("slack_task_threads").Where("account_id=? AND channel_id=? AND thread_ts=?", a, channel, root).UpdateColumn("card_ts", ts).Error
}

// EnsureAgentChat gives concurrent deliveries in one private thread a single Dock conversation.
func (r *SlackRepository) EnsureAgentChat(ctx context.Context, workspace, user, id, title string) error {
	return r.db.WithContext(ctx).Exec(`INSERT INTO dock_chats (id,workspace_id,user_id,title,visibility,slack_thread,created_at,updated_at)
        VALUES (?,?,?,?,'private',true,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP) ON CONFLICT (id) DO NOTHING`, id, workspace, user, title).Error
}

// Limit projections to the accepted turn even after the same run has resumed.
const slackTurnMessages = ` FROM agent_run_messages turn
 JOIN agent_run_messages reply ON reply.workspace_id=turn.workspace_id AND reply.dock_chat_id=turn.dock_chat_id
 AND reply.run_id=turn.run_id AND reply.dock_chat_sequence>turn.dock_chat_sequence
 WHERE turn.workspace_id=? AND turn.dock_chat_id=? AND turn.id=? AND turn.role='user'
 AND reply.role='assistant'
 AND NOT EXISTS (SELECT 1 FROM agent_run_messages later WHERE later.dock_chat_id=turn.dock_chat_id
 AND later.role='user' AND later.dock_chat_sequence BETWEEN turn.dock_chat_sequence+1 AND reply.dock_chat_sequence) `

func (r *SlackRepository) AgentTurnTools(ctx context.Context, workspace, chat, message string) ([]string, error) {
	var row struct{ ToolInvocations []byte }
	err := r.db.WithContext(ctx).Raw("SELECT reply.tool_invocations "+slackTurnMessages+
		` AND reply.tool_invocations IS NOT NULL AND CAST(reply.tool_invocations AS text) NOT IN ('[]','null') ORDER BY reply.dock_chat_sequence DESC LIMIT 1`, workspace, chat, message).Scan(&row).Error
	if err != nil || len(row.ToolInvocations) == 0 {
		return nil, err
	}
	var calls []struct {
		Name string `json:"tool_name"`
	}
	if err = json.Unmarshal(row.ToolInvocations, &calls); err != nil {
		return nil, err
	}
	var names []string
	for _, call := range calls {
		names = append(names, call.Name)
	}
	return names, nil
}

// AgentTurnStatus checks the backing run of the accepted message, even if the chat has since moved on.
func (r *SlackRepository) AgentTurnStatus(ctx context.Context, workspace, chat, message string) (model.AgentRun, error) {
	var run model.AgentRun
	err := r.db.WithContext(ctx).Table("agent_run_messages turn").Select("runs.status, runs.error_message").Joins("JOIN agent_runs runs ON runs.id=turn.run_id AND runs.workspace_id=turn.workspace_id").Where("turn.workspace_id=? AND turn.dock_chat_id=? AND turn.id=? AND turn.role='user'", workspace, chat, message).Scan(&run).Error
	return run, err
}

// AgentAnswer returns only the final response for the accepted turn. Progress, tool
// output, and answers to subsequent turns must never be sent to Slack.
func (r *SlackRepository) AgentAnswer(ctx context.Context, workspace, chat, message string) (string, error) {
	var answer string
	err := r.db.WithContext(ctx).Raw("SELECT reply.content "+slackTurnMessages+
		" AND reply.message_type='assistant_final' ORDER BY reply.dock_chat_sequence LIMIT 1", workspace, chat, message).Scan(&answer).Error
	return answer, err
}

// NotificationPreferences also returns connection state for a member who has yet to link.
func (r *SlackRepository) NotificationPreferences(ctx context.Context, workspace, user string) (model.SlackNotificationPreferences, error) {
	var pref model.SlackNotificationPreferences
	err := r.db.WithContext(ctx).Table("support_channel_accounts a").
		Select(`a.id AS account_id, a.external_account_id AS team_id, a.handle AS bot_user_id,
			coalesce(m.slack_user,'') AS slack_user, m.user_id IS NOT NULL AS linked,
			m.notify_categories, coalesce(m.notify_assignments,false) AS assignments, coalesce(m.notify_mentions,false) AS mentions`).
		Joins("LEFT JOIN slack_members m ON m.account_id=a.id AND m.user_id=?", user).
		Where("a.workspace_id=? AND a.provider='slack' AND a.disabled_at IS NULL", workspace).
		Limit(1).Scan(&pref).Error
	return pref, err
}

func (r *SlackRepository) UpdateNotificationPreferences(ctx context.Context, workspace, user string, patch model.UpdateSlackNotificationPreferences) (bool, error) {
	var found bool
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row struct {
			AccountID string
			Overrides model.JSONB `gorm:"column:notify_categories;type:jsonb"`
		}
		result := tx.Table("slack_members").Select("account_id, notify_categories").
			Where("user_id=? AND account_id IN (SELECT id FROM support_channel_accounts WHERE workspace_id=? AND provider='slack' AND disabled_at IS NULL)", user, workspace).
			Clauses(clause.Locking{Strength: "UPDATE"}).Limit(1).Find(&row)
		if result.Error != nil || result.RowsAffected == 0 {
			return result.Error
		}
		found = true
		if row.Overrides == nil {
			row.Overrides = model.JSONB{}
		}
		updates := map[string]any{}
		for family, value := range map[string]*bool{"assignments": patch.Assignments, "mentions": patch.Mentions} {
			if value == nil {
				continue
			}
			updates["notify_"+family] = *value
			// A choice made through the older family switch is explicit for each of its rows.
			for key, group := range model.SlackCategoryDefaults {
				if group == family {
					row.Overrides[key] = *value
				}
			}
		}
		for key, value := range patch.Categories {
			row.Overrides[key] = value
		}
		updates["notify_categories"] = row.Overrides
		return tx.Table("slack_members").Where("account_id=? AND user_id=?", row.AccountID, user).Updates(updates).Error
	})
	return found, err
}

// WithDMWindow serializes Slack sends for the linked recipient, including across worker replicas.
// Update the receipt only after Slack accepts the post or edit.
func (r *SlackRepository) WithDMWindow(ctx context.Context, account, user, slackUser string, send func([]byte) ([]byte, error)) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		member := tx.Table("slack_members").Where("account_id=? AND user_id=? AND slack_user=?", account, user, slackUser)
		var row struct{ DMWindow []byte }
		result := member.Session(&gorm.Session{}).Select("dm_window").Clauses(clause.Locking{Strength: "UPDATE"}).Limit(1).Find(&row)
		if result.Error != nil || result.RowsAffected == 0 {
			return result.Error
		}
		next, err := send(row.DMWindow)
		if err != nil || next == nil {
			return err
		}
		return member.UpdateColumn("dm_window", string(next)).Error
	})
}

// Actionable DMs may wait in the retry queue while the user resolves the request.
func (r *SlackRepository) NotificationActionPending(ctx context.Context, recipient string, event model.NotificationEventInput) (bool, error) {
	var count int64
	db := r.db.WithContext(ctx)
	switch event.EventType {
	case "task.agent_attention_required":
		interaction, _ := event.Metadata["interaction_id"].(string)
		err := db.Table("agent_run_interactions i").
			Joins("JOIN agent_runs r ON r.id=i.run_id AND r.workspace_id=i.workspace_id").
			Where("i.workspace_id=? AND i.run_id=? AND i.status=? AND (i.request_id=? OR CAST(i.id AS text)=?)", event.WorkspaceID, event.EntityID, model.AgentRunInteractionStatusPending, interaction, interaction).
			Where("r.status IN ?", []string{model.AgentRunStatusQueued, model.AgentRunStatusRunning, model.AgentRunStatusPaused}).Count(&count).Error
		return count > 0, err
	case "external_mcp.reauthorization_required":
		err := db.Model(&model.ExternalMCPServer{}).
			Where("workspace_id=? AND id=? AND status=? AND coalesce(authorized_by_user_id,created_by)=?", event.WorkspaceID, event.EntityID, model.ExternalMCPStatusReauthorizationNeeded, recipient).Count(&count).Error
		return count > 0, err
	default:
		return true, nil
	}
}
