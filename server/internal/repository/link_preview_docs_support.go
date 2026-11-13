package repository

import (
	"context"
	"errors"
	"strconv"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/linkpreview"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func (q *previewLookup) document(id string) *model.LinkPreview {
	var doc model.DocsDocument
	if !q.get(&doc, id, "id, title, excerpt, space_id, status, owner_id", "deleted_at IS NULL AND status = 'published' AND visibility = 'workspace_wide'") {
		return nil
	}
	var space model.DocsSpace
	if !q.get(&space, doc.SpaceID, "id, name", "deleted_at IS NULL AND visibility = 'workspace_wide'") {
		return nil
	}
	return previewCard("document", doc.Title, stringValue(doc.Excerpt), previewField("Space", space.Name), previewField("Status", previewLabel(doc.Status)), previewField("Owner", q.member(doc.OwnerID, false)))
}

func (q *previewLookup) space(t linkpreview.Target) *model.LinkPreview {
	var space model.DocsSpace
	if !q.get(&space, t.ID, "id, name, type", "deleted_at IS NULL AND visibility = 'workspace_wide'") {
		return nil
	}
	if t.Query.Get("collection") == "__uncollected__" {
		return previewCard("space", "Uncategorized documents in Helpin", "")
	}
	typeName := "Internal"
	if space.Type == model.SpaceTypeExternalCapable {
		typeName = "Help center"
	}
	return previewCard("space", space.Name, "", previewField("Teams", q.teams("docs_space_teams", "space_id", space.ID)), previewField("Type", typeName))
}

func (q *previewLookup) collection(t linkpreview.Target) *model.LinkPreview {
	var space model.DocsSpace
	if !q.get(&space, t.ParentID, "id, name", "deleted_at IS NULL AND visibility = 'workspace_wide'") {
		return nil
	}
	var collection model.DocsCollection
	if !q.get(&collection, t.ID, "id, name, description, space_id", "deleted_at IS NULL") || collection.SpaceID != space.ID {
		// Invalid selectors retain only their eligible containing-space preview.
		return q.space(linkpreview.Target{ID: t.ParentID})
	}
	return previewCard("collection", collection.Name, stringValue(collection.Description), previewField("Space", space.Name), previewField("Teams", q.teams("docs_space_teams", "space_id", space.ID)))
}

func (q *previewLookup) conversation(id string) *model.LinkPreview {
	var conversation model.SupportConversation
	if !q.get(&conversation, id, "id, subject, display_id, status, mailbox_id, assigned_user_id, assigned_agent_id", "anonymized_at IS NULL AND merged_into_id IS NULL") {
		return nil
	}
	assignee := q.member(conversation.AssignedUserID, true)
	if conversation.AssignedUserID == nil && conversation.AssignedAgentID != nil {
		assignee = q.name("agents", *conversation.AssignedAgentID)
	}
	card := previewCard("conversation", conversation.Subject, "", previewField("Inbox", q.name("support_mailboxes", stringValue(conversation.MailboxID))), previewField("Status", previewLabel(conversation.Status)), previewField("Assignee", assignee))
	if conversation.DisplayID > 0 {
		card.Reference = "#" + strconv.Itoa(conversation.DisplayID)
	}
	return card
}

func (q *previewLookup) mailbox(id string) *model.LinkPreview {
	var mailbox model.SupportMailbox
	if !q.get(&mailbox, id, "id, name, description, linked_team_id") {
		return nil
	}
	return previewCard("mailbox", mailbox.Name, stringValue(mailbox.Description), previewField("Team", q.team(mailbox.LinkedTeamID)))
}

// GetPublicDocument resolves the existing explicit document grant without
// fetching document versions or bodies. The workspace toggle does not revoke it.
func (r *LinkPreviewRepository) GetPublicDocument(ctx context.Context, token string) (*model.LinkPreviewRecord, error) {
	var doc model.DocsDocument
	err := r.db.WithContext(ctx).Select("id, title, excerpt, status, updated_at").
		Where("share_token = ? AND is_publicly_shared = true AND deleted_at IS NULL AND status <> 'archived'", token).First(&doc).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	card := previewCard("document", doc.Title, stringValue(doc.Excerpt), previewField("Status", previewLabel(doc.Status)), previewField("Updated", doc.UpdatedAt.Format("Jan 2, 2006")))
	return &model.LinkPreviewRecord{Preview: *card, Modules: []model.ModuleID{model.ModuleDocs}}, nil
}

// GetPublicResource follows an active bearer share and reads only the public
// title/status/date fields. It never loads transcripts, run inputs or artifacts.
func (r *LinkPreviewRepository) GetPublicResource(ctx context.Context, token string) (*model.LinkPreviewRecord, error) {
	share, err := NewPublicShareRepository(r.db).GetActiveByToken(ctx, token)
	if err != nil || share == nil {
		return nil, err
	}
	var ws model.Workspace
	err = r.db.WithContext(ctx).Select("id, timezone").Where("id = ?", share.WorkspaceID).First(&ws).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	q := &previewLookup{db: r.db.WithContext(ctx), workspace: &ws}
	var card *model.LinkPreview
	switch share.ResourceType {
	case model.PublicShareResourceDockChat:
		card = q.chat(share.ResourceID, true)
	case model.PublicShareResourceAgentRun:
		var run model.AgentRun
		// Support previews are creator-only even through the public share reader.
		if q.get(&run, share.ResourceID, "id, agent_id, status, started_at", "COALESCE(target_type, '') <> 'support_preview'") {
			card = previewCard("run", q.name("agents", run.AgentID), "", previewField("Status", previewLabel(run.Status)), previewField("Started", q.date(run.StartedAt, true)))
		}
	}
	if q.err != nil {
		return nil, q.err
	}
	if card == nil {
		return nil, nil
	}
	return &model.LinkPreviewRecord{Preview: *card, Modules: []model.ModuleID{model.ModuleAgents}}, nil
}
