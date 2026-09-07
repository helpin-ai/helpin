package model

import "testing"

func TestCodingSessionMessageEventPreservesDeliveryIdentity(t *testing.T) {
	for _, status := range []string{"pending", "sent", "failed"} {
		t.Run(status, func(t *testing.T) {
			clientID := "client-submission-1"
			event := CodingSessionEventFromAgentRunMessage(&AgentRun{ID: "run-1"}, &AgentRunMessage{
				ID: "message-1", Role: "user", ClientMessageID: &clientID, DeliveryStatus: status,
			})
			if event.Payload["client_message_id"] != clientID {
				t.Fatalf("client correlation lost: %#v", event.Payload["client_message_id"])
			}
			if event.Payload["delivery_status"] != status {
				t.Fatalf("delivery status lost: %#v", event.Payload["delivery_status"])
			}
			if event.Payload["persisted_message_id"] != "message-1" {
				t.Fatal("persisted identity changed")
			}
		})
	}
}
