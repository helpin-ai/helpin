package service

import (
	"context"
	"errors"

	"github.com/helpin-ai/helpin/server/internal/model"
)

var ErrDockPinKind = errors.New("pin kind must be chat or run")

// SetPin saves a personal preference after checking the underlying resource.
func (s *DockChatService) SetPin(ctx context.Context, workspaceID, userID, kind, id string, pinned bool) error {
	if kind != "chat" && kind != "run" {
		return ErrDockPinKind
	}
	if pinned {
		if kind == "chat" {
			chat, err := s.accessibleChat(ctx, workspaceID, userID, id)
			if err != nil {
				return err
			}
			if chat.ArchivedAt != nil {
				return ErrDockChatNotFound
			}
		} else {
			if _, err := s.agentService.GetDockRunForActor(ctx, workspaceID, userID, id); err != nil {
				return err
			}
		}
	}
	// Unpin remains possible after access is revoked; it only removes the actor's preference.
	return s.chatRepo.SetPin(ctx, model.DockPin{WorkspaceID: workspaceID, UserID: userID, Kind: kind, ResourceID: id}, pinned)
}

// ListPins reauthorizes pins so stale preferences never expose revoked resources.
func (s *DockChatService) ListPins(ctx context.Context, workspaceID, userID string) (*model.DockPinsResponse, error) {
	pins, err := s.chatRepo.ListPins(ctx, workspaceID, userID)
	if err != nil {
		return nil, err
	}
	response := &model.DockPinsResponse{Chats: []model.DockChat{}, Runs: []model.DockRunSummary{}}
	var runs []model.AgentRun
	for _, pin := range pins {
		if pin.Kind == "chat" {
			chat, err := s.accessibleChat(ctx, workspaceID, userID, pin.ResourceID)
			if errors.Is(err, ErrDockChatNotFound) {
				continue
			}
			if err != nil {
				return nil, err
			}
			if chat.ArchivedAt == nil {
				response.Chats = append(response.Chats, *chat)
			}
		} else if pin.Kind == "run" {
			run, err := s.agentService.GetDockRunForActor(ctx, workspaceID, userID, pin.ResourceID)
			if errors.Is(err, ErrDockRunNotFound) {
				continue
			}
			if err != nil {
				return nil, err
			}
			runs = append(runs, *run)
		}
	}
	if err := s.hydrateActiveRunStatuses(ctx, workspaceID, response.Chats); err != nil {
		return nil, err
	}
	if len(runs) > 0 {
		summaries, err := s.agentService.summarizeDockRuns(ctx, workspaceID, runs)
		if err != nil {
			return nil, err
		}
		response.Runs = summaries.Runs
	}
	return response, nil
}
