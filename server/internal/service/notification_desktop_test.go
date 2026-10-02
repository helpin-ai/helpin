package service

import (
	"encoding/json"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestNotificationWSPayloadDesktop(t *testing.T) {
	event := model.NotificationEventInput{EventType: "task.mention", EntityType: "task", EntityID: "task-1", Title: "Sam mentioned you", Body: "Please review", Category: "mention"}
	var payload map[string]interface{}
	if err := json.Unmarshal(buildNotificationWSData("user-1", event, "normal", "snoozed"), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["delivery_id"] == nil || payload["delivery_id"] == "" {
		t.Fatal("missing delivery identity for local WebSocket hubs")
	}
	for key, want := range map[string]string{"recipient_id": "user-1", "entity_id": "task-1", "title": "Sam mentioned you", "body": "Please review", "status": "snoozed", "category": "mentions"} {
		if payload[key] != want {
			t.Errorf("%s = %v, want %s", key, payload[key], want)
		}
	}
}
