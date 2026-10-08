package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

type supportSetupSettings struct {
	AIEnabled          bool     `json:"ai_enabled"`
	AIAgentID          *string  `json:"ai_agent_id"`
	TriageEnabled      bool     `json:"triage_enabled"`
	WidgetHelpSpaceIDs []string `json:"widget_help_space_ids"`
}

// GetSupportSetupEvidence loads only support configuration. It never writes
// goals or achievements, and shares its predicates with the normal setup guide.
func (r *SetupRepository) GetSupportSetupEvidence(ctx context.Context, workspaceID string) (model.SetupEvidence, error) {
	evidence := model.SetupEvidence{}
	_, err := r.readSupportSetupEvidence(ctx, workspaceID, &evidence)
	return evidence, err
}

func (r *SetupRepository) readSupportSetupEvidence(ctx context.Context, workspaceID string, evidence *model.SetupEvidence) (supportSetupSettings, error) {
	emailRouteCount, err := r.ActiveEmailRouteCount(ctx, workspaceID)
	if err != nil {
		return supportSetupSettings{}, err
	}
	for _, query := range []struct {
		table, where string
		dest         *int64
	}{
		{"support_content_sources", "workspace_id = ? AND sync_status = 'ready' AND (indexed_pages > 0 OR indexed_chunks > 0)", &evidence.BrandKnowledgeSourceCount},
		{"support_mailboxes", "workspace_id = ? AND active = true", &evidence.TeamInboxCount},
	} {
		if err := r.db.WithContext(ctx).Table(query.table).Where(query.where, workspaceID).Count(query.dest).Error; err != nil {
			return supportSetupSettings{}, fmt.Errorf("read support setup evidence from %s: %w", query.table, err)
		}
	}
	verifiedWidgetCount, err := r.VerifiedWidgetInstallationCount(ctx, workspaceID)
	if err != nil {
		return supportSetupSettings{}, err
	}
	if err := r.db.WithContext(ctx).Table("docs_helpcenter_articles AS articles").
		Joins("JOIN docs_documents AS documents ON documents.id = articles.document_id").
		Joins("JOIN docs_spaces AS spaces ON spaces.id = documents.space_id AND spaces.workspace_id = documents.workspace_id").
		Where("documents.workspace_id = ? AND documents.deleted_at IS NULL AND spaces.deleted_at IS NULL AND spaces.type = 'external_capable' AND articles.public_published_at IS NOT NULL AND "+notSampleDocument, workspaceID).
		Count(&evidence.PublicHelpDocCount).Error; err != nil {
		return supportSetupSettings{}, fmt.Errorf("read published help-doc setup evidence: %w", err)
	}
	var supportSettingsJSON string
	if err := r.db.WithContext(ctx).Table("support_widget_installations").
		Select("settings").Where("workspace_id = ? AND active = true", workspaceID).
		Limit(1).Scan(&supportSettingsJSON).Error; err != nil {
		return supportSetupSettings{}, fmt.Errorf("read support settings setup evidence: %w", err)
	}
	var supportSettings supportSetupSettings
	if supportSettingsJSON != "" {
		if err := json.Unmarshal([]byte(supportSettingsJSON), &supportSettings); err != nil {
			return supportSetupSettings{}, fmt.Errorf("decode support settings setup evidence: %w", err)
		}
	}
	evidence.SupportAIAgentActive = supportSettings.AIEnabled && supportSettings.AIAgentID != nil && strings.TrimSpace(*supportSettings.AIAgentID) != ""
	if supportSettings.TriageEnabled && evidence.TeamInboxCount > 0 {
		if err := r.db.WithContext(ctx).Table("support_mailboxes AS mailboxes").
			Where("mailboxes.workspace_id = ? AND mailboxes.active = true AND ((mailboxes.triage_eligible = true AND (COALESCE(TRIM(mailboxes.routing_prompt), '') <> '' OR COALESCE(TRIM(mailboxes.description), '') <> '')) OR EXISTS (SELECT 1 FROM support_triage_rules rules WHERE rules.workspace_id = mailboxes.workspace_id AND rules.target_mailbox_id = mailboxes.id AND rules.active = true))", workspaceID).
			Count(&evidence.AutomaticRoutingCount).Error; err != nil {
			return supportSetupSettings{}, fmt.Errorf("read automatic support routing evidence: %w", err)
		}
	}

	evidence.SupportEmailInboxCount = emailRouteCount
	evidence.LiveChatInstallationCount = verifiedWidgetCount
	evidence.SupportChannelCount = verifiedWidgetCount + emailRouteCount
	return supportSettings, nil
}
