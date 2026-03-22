package service

import (
	"context"
	"log/slog"
	"time"
)

// RunAssumedResolutionScan scans for idle AI-pending conversations and marks them as resolved.
// Should be called periodically (e.g., every hour) by a cron job or Temporal workflow.
func (s *SupportAIService) RunAssumedResolutionScan(ctx context.Context) error {
	if s == nil || s.db == nil {
		return nil
	}

	// Find all workspaces with AI-first enabled installations.
	type wsSettings struct {
		WorkspaceID string
		Settings    string
	}
	var installations []wsSettings
	if err := s.db.WithContext(ctx).
		Table("support_widget_installations").
		Select("workspace_id, settings").
		Where("active = true").
		Find(&installations).Error; err != nil {
		return err
	}

	for _, inst := range installations {
		settings := parseSettings(inst.Settings)
		if !settings.AIEnabled || settings.AIResponseMode != "ai_first" || settings.AIAutoResolveTimeout <= 0 {
			continue
		}

		cutoff := time.Now().Add(-time.Duration(settings.AIAutoResolveTimeout) * time.Hour)

		// Find conversations with ai_state='pending' where last message is from AI and older than cutoff.
		// Use a raw query for the subquery on last message.
		result := s.db.WithContext(ctx).Exec(`
			UPDATE support_conversations
			SET ai_state = 'resolved',
			    ai_resolved_at = NOW(),
			    ai_resolution_type = 'assumed',
			    updated_at = NOW()
			WHERE workspace_id = ?
			  AND ai_state = 'pending'
			  AND id IN (
			    SELECT sc.id FROM support_conversations sc
			    INNER JOIN (
			      SELECT conversation_id, MAX(created_at) as last_msg_at
			      FROM support_messages
			      WHERE sender_type IN ('agent', 'ai')
			      GROUP BY conversation_id
			    ) lm ON lm.conversation_id = sc.id
			    WHERE sc.workspace_id = ?
			      AND sc.ai_state = 'pending'
			      AND lm.last_msg_at < ?
			      AND NOT EXISTS (
			        SELECT 1 FROM support_messages sm2
			        WHERE sm2.conversation_id = sc.id
			          AND sm2.sender_type = 'customer'
			          AND sm2.created_at > lm.last_msg_at
			      )
			  )
		`, inst.WorkspaceID, inst.WorkspaceID, cutoff)

		if result.Error != nil {
			slog.ErrorContext(ctx, "assumed resolution scan failed",
				"workspace_id", inst.WorkspaceID,
				"error", result.Error,
			)
			continue
		}

		if result.RowsAffected > 0 {
			slog.InfoContext(ctx, "assumed resolution scan completed",
				"workspace_id", inst.WorkspaceID,
				"resolved_count", result.RowsAffected,
			)
		}
	}

	return nil
}
