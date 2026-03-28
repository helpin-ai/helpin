package service

import (
	"encoding/json"

	"github.com/helpin-ai/helpin/server/internal/websocket"
)

func publishWorkspaceEvent(wsPublisher *websocket.Publisher, action, entity, entityID, workspaceID, actorID string) {
	if wsPublisher == nil || workspaceID == "" || entityID == "" {
		return
	}
	wsPublisher.Publish(websocket.Event{
		Action:      action,
		Entity:      entity,
		EntityID:    entityID,
		WorkspaceID: workspaceID,
		ActorID:     actorID,
	})
}

func publishWorkspaceEventWithParent(wsPublisher *websocket.Publisher, action, entity, entityID, workspaceID, actorID, parentType, parentID string, data any) {
	if wsPublisher == nil || workspaceID == "" || entityID == "" {
		return
	}
	var raw json.RawMessage
	if data != nil {
		raw, _ = json.Marshal(data)
	}
	wsPublisher.Publish(websocket.Event{
		Action:      action,
		Entity:      entity,
		EntityID:    entityID,
		WorkspaceID: workspaceID,
		ActorID:     actorID,
		ParentType:  parentType,
		ParentID:    parentID,
		Data:        raw,
	})
}
