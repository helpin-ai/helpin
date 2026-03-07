package repository

import (
	"context"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

func resolveRewardEmployeeIDTx(ctx context.Context, tx *gorm.DB, workspaceID, reference string) (string, error) {
	reference = strings.TrimSpace(reference)
	if reference == "" {
		return "", nil
	}

	var member struct {
		ID string
	}
	err := tx.WithContext(ctx).
		Table("workspace_members").
		Select("id").
		Where("workspace_id = ? AND id = ?", workspaceID, reference).
		Scan(&member).Error
	if err != nil {
		return "", fmt.Errorf("resolve reward employee member: %w", err)
	}
	if member.ID != "" {
		return member.ID, nil
	}
	return "", fmt.Errorf("workspace member not found for reward employee")
}
