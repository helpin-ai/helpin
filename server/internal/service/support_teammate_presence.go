package service

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

const supportTeammateAwayThreshold = 5 * time.Minute

var ErrInvalidTeammateStatus = errors.New("invalid teammate status")

// ListTeammatePresence returns live support availability for active workspace members.
func (s *SupportInboxService) ListTeammatePresence(ctx context.Context, workspaceID string) ([]model.SupportTeammatePresenceStatus, error) {
	statuses, err := resolveSupportTeammatePresenceStatuses(
		ctx,
		s.workspaceRepo,
		s.presence,
		s.statusOverrideRepo,
		workspaceID,
		time.Now(),
	)
	if err != nil {
		return nil, err
	}

	sort.Slice(statuses, func(i, j int) bool {
		return statuses[i].UserID < statuses[j].UserID
	})

	return statuses, nil
}

func (s *SupportInboxService) GetTeammatePresence(ctx context.Context, workspaceID, userID string) (*model.SupportTeammatePresenceStatus, error) {
	statuses, err := s.ListTeammatePresence(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	for _, status := range statuses {
		if status.UserID == userID {
			result := status
			return &result, nil
		}
	}
	return &model.SupportTeammatePresenceStatus{
		UserID: userID,
		Status: model.SupportTeammateStatusOffline,
		Source: model.SupportTeammateStatusSourceAuto,
	}, nil
}

func (s *SupportInboxService) UpdateMyTeammatePresence(ctx context.Context, workspaceID, userID string, manualStatus *string) (*model.SupportTeammatePresenceStatus, error) {
	if workspaceID == "" || userID == "" {
		return nil, nil
	}
	if s.statusOverrideRepo == nil {
		return s.GetTeammatePresence(ctx, workspaceID, userID)
	}

	if manualStatus == nil || *manualStatus == "" {
		if err := s.statusOverrideRepo.DeleteForUser(ctx, workspaceID, userID); err != nil {
			return nil, err
		}
	} else {
		switch *manualStatus {
		case model.SupportTeammateStatusOnline, model.SupportTeammateStatusAway, model.SupportTeammateStatusOffline:
		default:
			return nil, ErrInvalidTeammateStatus
		}
		if _, err := s.statusOverrideRepo.Upsert(ctx, workspaceID, userID, *manualStatus); err != nil {
			return nil, err
		}
	}

	status, err := s.GetTeammatePresence(ctx, workspaceID, userID)
	if err != nil {
		return nil, err
	}
	if status != nil && s.wsPublisher != nil {
		PublishSupportTeammatePresence(ctx, s.wsPublisher, workspaceID, *status)
	}
	return status, nil
}
