package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// QueueDailySourceSync queues fresh crawls through the existing per-source
// workflow. Queue failures do not prevent other workspaces from refreshing.
func (s *SupportContentSyncService) QueueDailySourceSync(ctx context.Context) error {
	if s == nil || s.sourceRepo == nil || s.starter == nil {
		return fmt.Errorf("daily content sync pipeline is not configured")
	}
	since := model.NextContentSourceAutoSync(time.Now()).AddDate(0, 0, -1)
	var queueErrors []error
	err := s.sourceRepo.ForEachAutoSyncSource(ctx, since, func(source model.SupportContentSource) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := s.queueSourceSync(ctx, source.WorkspaceID, source.ID, &since); err != nil {
			queueErrors = append(queueErrors, fmt.Errorf("queue website %s: %w", source.ID, err))
		}
		return nil
	})
	return errors.Join(append(queueErrors, err)...)
}
