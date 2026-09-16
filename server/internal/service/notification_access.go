package service

import (
	"context"
	"errors"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

const notificationEmailOnly = "email_only"
const notificationTeamKey = "notification_team_id"

type NotificationAccessChecker interface {
	CanReceive(context.Context, string, model.NotificationEventInput) (bool, error)
}

type NotificationAccessPolicy struct {
	authz     *authorization.AuthzService
	resources *repository.NotificationRepository
}

func NewNotificationAccessPolicy(authz *authorization.AuthzService, resources *repository.NotificationRepository) *NotificationAccessPolicy {
	return &NotificationAccessPolicy{authz: authz, resources: resources}
}

func (p *NotificationAccessPolicy) CanReceive(ctx context.Context, userID string, event model.NotificationEventInput) (bool, error) {
	cache, _ := ctx.Value(notificationAccessCacheKey{}).(map[string]*authorization.Actor)
	key := event.WorkspaceID + ":" + userID
	actor := cache[key]
	var err error
	if actor == nil {
		actor, err = p.authz.ResolveActor(ctx, event.WorkspaceID, userID)
		if err == nil && cache != nil {
			cache[key] = actor
		}
	}

	if err != nil {
		if errors.Is(err, authorization.ErrNotAMember) || errors.Is(err, authorization.ErrMembershipPending) || errors.Is(err, authorization.ErrMembershipInactive) || errors.Is(err, authorization.ErrMembershipRevoked) {
			return false, nil
		}
		return false, err
	}
	module := model.ModulePM
	switch event.EntityType {
	case "support_conversation":
		module = model.ModuleSupport
	case "crm_signal":
		module = model.ModuleCRM
	case "doc", "document":
		module = model.ModuleDocs
	case "external_mcp_server":
		// External tools live in settings; using an agent does not require Automation access.
		if !p.authz.Can(actor, authorization.PermSettingsRead) {
			return false, nil
		}
	case "agent_run":
		if taskID, ok := event.Metadata["task_id"].(string); !ok || taskID == "" {
			module = model.ModuleAutomation
		}
	}
	allowed, err := p.authz.CanAccessModule(ctx, actor, module)
	if err != nil || !allowed {
		return false, err
	}
	return p.resources.CanReadNotificationResource(ctx, actor, event)
}

// SetAccessChecker is wired in both API and worker composition roots.
func (s *NotificationService) SetAccessChecker(checker NotificationAccessChecker) *NotificationService {
	s.accessChecker = checker
	return s
}

func (s *NotificationService) canReceive(ctx context.Context, userID string, event model.NotificationEventInput) (bool, error) {
	if s.accessChecker == nil {
		return true, nil
	} // Legacy isolated service fixtures; runtime always supplies the policy.
	return s.accessChecker.CanReceive(ctx, userID, event)
}

func cloneNotificationMetadata(metadata model.JSONB) model.JSONB {
	result := model.JSONB{}
	for key, value := range metadata {
		result[key] = value
	}
	return result
}

func notificationTeam(metadata model.JSONB) string {
	team, _ := metadata[notificationTeamKey].(string)
	return team
}

func eventFromNotification(n model.Notification) model.NotificationEventInput {
	return model.NotificationEventInput{WorkspaceID: n.WorkspaceID, EntityType: n.EntityType, EntityID: n.EntityID, EventType: n.EventType, Metadata: n.Metadata, TeamID: notificationTeam(n.Metadata)}
}

type notificationAccessCacheKey struct{}

func withNotificationAccessCache(ctx context.Context) context.Context {
	if ctx.Value(notificationAccessCacheKey{}) != nil {
		return ctx
	}
	return context.WithValue(ctx, notificationAccessCacheKey{}, map[string]*authorization.Actor{})
}
