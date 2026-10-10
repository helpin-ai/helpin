package service

import (
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestCurrentSupportFollowUp(t *testing.T) {
	now := time.Now().UTC()
	for _, tc := range []struct {
		name    string
		change  func(*model.SupportConversation, *model.SupportAIFollowUp)
		visible bool
	}{
		{"scheduled", func(_ *model.SupportConversation, e *model.SupportAIFollowUp) { e.Status = "scheduled" }, true},
		{"reviewing", func(_ *model.SupportConversation, e *model.SupportAIFollowUp) { e.Status = "assessing" }, true},
		{"first reminder", func(c *model.SupportConversation, e *model.SupportAIFollowUp) {
			e.SentMessageID = strPtr("first")
			c.LastPublicMessageID = strPtr("first")
		}, true},
		{"final reminder", func(c *model.SupportConversation, e *model.SupportAIFollowUp) {
			e.SentMessageID = strPtr("first")
			e.SecondMessageID = strPtr("second")
			c.LastPublicMessageID = strPtr("second")
		}, true},
		{"stopped by teammate", func(_ *model.SupportConversation, e *model.SupportAIFollowUp) {
			e.Status = "cancelled"
			e.Reason = "cancelled_by_teammate"
		}, true},
		{"failed", func(_ *model.SupportConversation, e *model.SupportAIFollowUp) { e.Status = "failed" }, true},
		{"customer replied", func(c *model.SupportConversation, _ *model.SupportAIFollowUp) {
			c.LastPublicSenderType = strPtr("customer")
			c.LastPublicMessageID = strPtr("reply")
		}, false},
		{"new AI reply", func(c *model.SupportConversation, _ *model.SupportAIFollowUp) {
			c.LastPublicMessageID = strPtr("new-answer")
		}, false},
		{"human takeover", func(c *model.SupportConversation, _ *model.SupportAIFollowUp) { v := true; c.HumanTakeover = &v }, false},
		{"human assigned", func(c *model.SupportConversation, _ *model.SupportAIFollowUp) { c.AssignedUserID = strPtr("teammate") }, false},
		{"anonymized", func(c *model.SupportConversation, _ *model.SupportAIFollowUp) { c.AnonymizedAt = &now }, false},
		{"resolved", func(c *model.SupportConversation, _ *model.SupportAIFollowUp) { c.Status = "resolved" }, false},
		{"waiting for human", func(c *model.SupportConversation, _ *model.SupportAIFollowUp) {
			c.FlowState = strPtr("waiting_for_human")
		}, false},
		{"restarted AI", func(c *model.SupportConversation, _ *model.SupportAIFollowUp) { c.AIResumedAt = &now }, false},
		{"settings cancelled", func(_ *model.SupportConversation, e *model.SupportAIFollowUp) {
			e.Status = "cancelled"
			e.Reason = "settings_changed"
		}, false},
		{"skipped", func(_ *model.SupportConversation, e *model.SupportAIFollowUp) { e.Status = "skipped" }, false},
		{"handoff", func(_ *model.SupportConversation, e *model.SupportAIFollowUp) { e.Status = "handoff" }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := &model.SupportConversation{Status: "open", FlowState: strPtr("ai_handling"), AIState: strPtr("pending"), LastPublicSenderType: strPtr("ai"), LastPublicMessageID: strPtr("source")}
			e := &model.SupportAIFollowUp{Status: "waiting", SourceMessageID: "source", CreatedAt: now.Add(-time.Hour)}
			tc.change(c, e)
			if got := currentSupportFollowUp(c, e) != nil; got != tc.visible {
				t.Fatalf("visible=%v want=%v", got, tc.visible)
			}
		})
	}
}
