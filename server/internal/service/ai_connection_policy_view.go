package service

import (
	"context"
	"errors"

	"github.com/helpin-ai/helpin/server/internal/aiusage"
	"github.com/helpin-ai/helpin/server/internal/model"
)

// ErrAIConnectionPolicyUnavailable identifies an intentional edition restriction.
// Infrastructure failures must remain errors, not a disabled selection or fallback.
var ErrAIConnectionPolicyUnavailable = errors.New("this AI connection is not enabled for new runs")

func connectionPolicyView(ctx context.Context, policy AIConnectionAdmissionPolicy, workspace string, connection *model.AIConnection) (*model.AIConnectionPolicyView, error) {
	if connection == nil || connection.WorkspaceID != workspace {
		return &model.AIConnectionPolicyView{Message: "This connection is unavailable to this workspace member."}, nil
	}
	snapshot := &model.AIExecutionPolicySnapshot{Mode: "community", FundingMode: aiusage.FundingCustomerUnbilled}
	if policy != nil {
		var err error
		snapshot, err = policy.ResolveConnectionPolicy(ctx, workspace, connection)
		if errors.Is(err, ErrAIConnectionPolicyUnavailable) {
			// This sentinel is returned only by trusted edition policy, with static copy.
			return &model.AIConnectionPolicyView{Message: err.Error()}, nil
		}
		if err != nil {
			return nil, err
		}
		if snapshot == nil {
			return nil, errors.New("AI connection policy returned no accepted policy")
		}
	}
	return &model.AIConnectionPolicyView{Allowed: true, Pricing: snapshot}, nil
}

// Profile policy is projected only from connections visible to the member. It
// describes prospective funding; accepted runs keep their persisted snapshot.
func profileRoutePolicy(route model.AIProfileRoute, scope string, connections map[string]model.AIConnection) *model.AIConnectionPolicyView {
	c, ok := connections[route.ConnectionID]
	if !ok || c.Provider != route.Model.Provider || (scope == "workspace" && c.Scope != "workspace") {
		return &model.AIConnectionPolicyView{Message: "This connection is unavailable to this workspace member."}
	}
	return c.Policy
}
