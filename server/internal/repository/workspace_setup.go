package repository

import (
	"context"
	"fmt"
	"time"
)

// UsableSetupInvitationCount excludes expired/revoked invitations. An invitation
// is configuration evidence only, never proof of email delivery or membership.
func (r *SetupRepository) UsableSetupInvitationCount(ctx context.Context, workspaceID string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Table("workspace_invitations").
		Where("workspace_id = ? AND (status = 'accepted' OR (status = 'pending' AND expires_at > ?))", workspaceID, time.Now()).Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("read workspace setup invitations: %w", err)
	}
	return count, nil
}
