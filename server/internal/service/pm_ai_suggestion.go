package service

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
)

var (
	ErrPMAISuggestionForbidden = errors.New("active workspace membership is required")
	ErrPMAISuggestionInput     = errors.New("a current revision and valid review decision are required")
)

type pmAISuggestionStore interface {
	List(context.Context, string, string, int) ([]model.PMAISuggestionItem, int64, error)
	Get(context.Context, string, string, string) (*model.PMAISuggestionDetail, error)
	Decide(context.Context, string, string, string, string, string) (*model.PMAISuggestionDecisionResult, error)
}

type PMAISuggestionService struct{ store pmAISuggestionStore }

func NewPMAISuggestionService(store pmAISuggestionStore) *PMAISuggestionService {
	return &PMAISuggestionService{store: store}
}

func pmAISuggestionActor(ctx context.Context, ws string) (*authorization.Actor, error) {
	actor := authorization.GetActor(ctx)
	if actor == nil || actor.UserID == "" || actor.WorkspaceMemberID == "" || actor.WorkspaceID != ws || actor.Status != model.WorkspaceMemberStatusActive {
		return nil, ErrPMAISuggestionForbidden
	}
	return actor, nil
}

func (s *PMAISuggestionService) List(ctx context.Context, ws string, page int) ([]model.PMAISuggestionItem, int64, error) {
	actor, err := pmAISuggestionActor(ctx, ws)
	if err != nil {
		return nil, 0, err
	}
	if page < 1 {
		page = 1
	}
	if page > 1000000 {
		page = 1000000
	}
	return s.store.List(ctx, ws, actor.UserID, page)
}
func (s *PMAISuggestionService) Get(ctx context.Context, ws, id string) (*model.PMAISuggestionDetail, error) {
	actor, err := pmAISuggestionActor(ctx, ws)
	if err != nil {
		return nil, err
	}
	if _, err := uuid.Parse(id); err != nil {
		return nil, ErrPMAISuggestionInput
	}
	return s.store.Get(ctx, ws, actor.UserID, id)
}
func (s *PMAISuggestionService) Decide(ctx context.Context, ws, id, decision string, req model.PMAISuggestionDecision) (*model.PMAISuggestionDecisionResult, error) {
	actor, err := pmAISuggestionActor(ctx, ws)
	if err != nil {
		return nil, err
	}
	if _, err := uuid.Parse(id); err != nil {
		return nil, ErrPMAISuggestionInput
	}
	if strings.TrimSpace(req.Revision) == "" || (decision != "accept" && decision != "dismiss") {
		return nil, ErrPMAISuggestionInput
	}
	return s.store.Decide(ctx, ws, actor.UserID, id, req.Revision, decision)
}
