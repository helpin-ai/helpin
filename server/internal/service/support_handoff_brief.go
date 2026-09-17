package service

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// SupportHandoffBrief carries a bounded, optional agent-authored briefing.
// Missing context falls back to attributed transcript excerpts, never guesses.
type SupportHandoffBrief struct {
	ExpectedRunID       string   `json:"-"`
	Issue               string   `json:"issue_summary"`
	AttemptedSteps      []string `json:"attempted_steps"`
	UnresolvedQuestions []string `json:"unresolved_questions"`
}

func supportControlNote(conv *model.SupportConversation, actorID, content string, now time.Time) *model.SupportMessage {
	note := &model.SupportMessage{ID: uuid.NewString(), WorkspaceID: conv.WorkspaceID, ConversationID: conv.ID, SenderType: "agent", SenderDisplayName: strPtr("AI control"), Content: content, MessageType: "note", IsInternal: true, CreatedAt: now}
	if actorID != "" {
		note.SenderUserID = &actorID
	}
	return note
}

func briefText(text string) string {
	text = strings.Join(strings.Fields(text), " ")
	chars := []rune(text)
	if len(chars) > 700 {
		return string(chars[:700]) + "…"
	}
	return text
}

func briefList(items []string) string {
	var lines []string
	for _, item := range items {
		if text := briefText(item); text != "" {
			lines = append(lines, "- "+text)
		}
		if len(lines) == 5 {
			break
		}
	}
	return strings.Join(lines, "\n")
}

func buildSupportHandoffNote(conv *model.SupportConversation, history []model.SupportMessage, reason string, brief SupportHandoffBrief, now time.Time) *model.SupportMessage {
	issue := briefText(brief.Issue)
	var customer, ai []string
	var refs []string
	// Read only public conversation content. Retain attribution: a suggestion
	// from AI is not evidence that the customer performed it.
	for i := len(history) - 1; i >= 0; i-- {
		msg := history[i]
		if msg.IsInternal || msg.MessageType != "reply" || msg.DeletedAt.Valid {
			continue
		}
		text := briefText(msg.Content)
		if text == "" {
			continue
		}
		if msg.SenderType == "customer" && len(customer) < 2 {
			customer = append(customer, "Customer said: "+text)
			refs = append(refs, msg.ID)
		} else if (msg.SenderType == "ai" || msg.SenderAgentID != nil) && len(ai) < 2 {
			ai = append(ai, "AI said: "+text)
			refs = append(refs, msg.ID)
		}
	}
	if issue == "" {
		if len(customer) > 0 {
			issue = customer[0]
		} else {
			issue = briefText(conv.Subject)
		}
	}
	if issue == "" {
		issue = "No issue details recorded yet."
	}
	attempted := briefList(brief.AttemptedSteps)
	if attempted == "" {
		attempted = briefList(ai)
	}
	if attempted == "" {
		attempted = "No attempted steps recorded."
	}
	unresolved := briefList(brief.UnresolvedQuestions)
	if unresolved == "" {
		unresolved = "A teammate needs to review the customer's latest request; the remaining steps have not been established."
		if len(customer) > 0 {
			unresolved += "\n" + briefList(customer)
		}
	}
	content := fmt.Sprintf("AI handoff\n\nIssue\n%s\n\nAlready tried / suggested\n%s\n\nStill unresolved\n%s\n\nReason for handoff\n%s", issue, attempted, unresolved, briefText(strings.ReplaceAll(reason, "_", " ")))

	note := supportControlNote(conv, "", content, now)
	meta, _ := json.Marshal(map[string]any{"ai_handoff_brief": true, "reason": reason, "source_message_ids": refs, "agent_authored": brief.Issue != "" || len(brief.AttemptedSteps) > 0 || len(brief.UnresolvedQuestions) > 0})
	note.Metadata = string(meta)
	return note
}
