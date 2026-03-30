package worker

import (
	"encoding/json"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func executeInternalCommand(ctx *ExecutionContext, targetType, targetID, commandName string, input json.RawMessage) (json.RawMessage, bool, error) {
	if ctx == nil || ctx.Services == nil || ctx.Services.ExecuteInternalCommand == nil {
		return nil, false, nil
	}

	meta := model.InternalCommandContext{
		WorkspaceID: ctx.WorkspaceID,
		ActorID:     ctx.AgentID,
		AgentID:     ctx.AgentID,
		RunID:       ctx.RunID,
		TargetType:  strings.TrimSpace(targetType),
		TargetID:    strings.TrimSpace(targetID),
	}
	if meta.TargetType == "" {
		meta.TargetType = strings.TrimSpace(ctx.TargetType)
	}
	if meta.TargetID == "" {
		meta.TargetID = strings.TrimSpace(ctx.TargetID)
	}

	output, err := ctx.Services.ExecuteInternalCommand(ctx.Context, meta, commandName, input)
	if err != nil {
		return nil, true, err
	}
	return output, true, nil
}
