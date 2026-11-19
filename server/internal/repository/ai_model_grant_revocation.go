package repository

import (
	"context"
	"time"
)

type ModelGrantRevocation struct {
	GrantID      string
	WorkspaceID  string
	ProfileID    string
	ConnectionID string
}

func (r *AIConnectionRepository) DueModelGrantRevocations(ctx context.Context) ([]ModelGrantRevocation, error) {
	var batch []ModelGrantRevocation
	err := r.db.WithContext(ctx).Table("ai_model_grant_revocations").Where("retry_at <= ?", time.Now().UTC()).Order("retry_at, grant_id").Limit(32).Find(&batch).Error
	return batch, err
}

func (r *AIConnectionRepository) CompleteModelGrantRevocation(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Exec("DELETE FROM ai_model_grant_revocations WHERE grant_id = ?", id).Error
}

func (r *AIConnectionRepository) DelayModelGrantRevocation(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Table("ai_model_grant_revocations").Where("grant_id = ?", id).Update("retry_at", time.Now().Add(time.Minute).UTC()).Error
}
