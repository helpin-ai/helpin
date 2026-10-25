package service

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// SupportHandoffBrief carries a bounded, optional agent-authored briefing.
// Missing context stays missing; the card never fills gaps with guesses.
type SupportHandoffBrief struct {
	InvestigationKind      string   `json:"investigation_kind,omitempty"`
	InvestigationQuestion  string   `json:"investigation_question,omitempty"`
	ExpectedMessageID      string   `json:"-"`
	ExpectedControlVersion *int64   `json:"-"`
	ExpectedNoRun          bool     `json:"-"`
	ExpectedRunID          string   `json:"-"`
	InvestigationRequested bool     `json:"-"`
	CustomerUpdate         string   `json:"-"`
	Issue                  string   `json:"issue_summary"`
	AttemptedSteps         []string `json:"attempted_steps"`
	UnresolvedQuestions    []string `json:"unresolved_questions"`
	// Checked holds findings already tied to the sources the run used.
	Checked []SupportHandoffCheck `json:"-"`
	// CheckedKnown is true when the run's tool calls were read, so an empty
	// Checked list honestly means AI checked nothing.
	CheckedKnown    bool     `json:"-"`
	ReasonNote      string   `json:"-"`
	UnavailableData []string `json:"-"`
	HelpArticleGap  string   `json:"-"`
	// HeldReply is the AI answer the reply check stopped from being sent.
	HeldReply string `json:"-"`
}

// SupportHandoffCheck is one "AI checked" line. Source names where the finding
// came from and is empty when no tool call backs it.
type SupportHandoffCheck struct {
	Source  string `json:"source,omitempty"`
	Finding string `json:"finding,omitempty"`
}

// supportHandoffCard is the structured AI handoff the inbox renders. Each
// fact appears once; empty parts are left out rather than filled with fallbacks.
type supportHandoffCard struct {
	Issue                  string                `json:"issue,omitempty"`
	Reason                 string                `json:"reason"`
	Checked                []SupportHandoffCheck `json:"checked,omitempty"`
	NothingChecked         bool                  `json:"nothing_checked,omitempty"`
	OpenQuestions          []string              `json:"open_questions,omitempty"`
	UnavailableData        []string              `json:"unavailable_data,omitempty"`
	HelpArticleGap         string                `json:"help_article_gap,omitempty"`
	HeldReply              string                `json:"held_reply,omitempty"`
	InvestigationRequested bool                  `json:"investigation_requested,omitempty"`
}

func supportControlNote(conv *model.SupportConversation, actorID, content string, now time.Time) *model.SupportMessage {
	note := &model.SupportMessage{ID: uuid.NewString(), WorkspaceID: conv.WorkspaceID, ConversationID: conv.ID, SenderType: "agent", SenderDisplayName: strPtr("AI control"), Content: content, MessageType: "note", IsInternal: true, CreatedAt: now}
	if actorID != "" {
		note.SenderUserID = &actorID
	}
	return note
}

func supportControlActivity(conv *model.SupportConversation, actorID, actorName, eventType, content, reason string, now time.Time) *model.SupportMessage {
	note := supportControlNote(conv, actorID, content, now)
	note.MessageType = "system"
	note.SystemEventType = &eventType
	note.SenderType = "user"
	note.SenderDisplayName = &actorName
	metadata, _ := json.Marshal(map[string]string{"reason": reason})
	note.Metadata = string(metadata)
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

func briefItems(items []string) []string {
	var out []string
	for _, item := range items {
		if text := briefText(item); text != "" {
			out = append(out, text)
		}
		if len(out) == 5 {
			break
		}
	}
	return out
}

func buildSupportHandoffNote(conv *model.SupportConversation, history []model.SupportMessage, reason string, brief SupportHandoffBrief, now time.Time) *model.SupportMessage {
	var refs []string
	for i := len(history) - 1; i >= 0 && len(refs) < 4; i-- {
		msg := history[i]
		if msg.IsInternal || msg.MessageType != "reply" || msg.DeletedAt.Valid || strings.TrimSpace(msg.Content) == "" {
			continue
		}
		if msg.SenderType == "customer" || msg.SenderType == "ai" || msg.SenderAgentID != nil {
			refs = append(refs, msg.ID)
		}
	}
	card := supportHandoffCard{
		Issue:                  briefText(brief.Issue),
		Reason:                 briefText(brief.ReasonNote),
		OpenQuestions:          briefItems(brief.UnresolvedQuestions),
		UnavailableData:        briefItems(brief.UnavailableData),
		HelpArticleGap:         briefText(brief.HelpArticleGap),
		HeldReply:              formatSupportDraftReply(brief.HeldReply),
		InvestigationRequested: brief.InvestigationRequested,
	}
	if card.Reason == "" {
		card.Reason = supportHandoffReasonText(reason)
	}
	for _, check := range brief.Checked {
		check = SupportHandoffCheck{Source: briefText(check.Source), Finding: briefText(check.Finding)}
		if check.Source != "" || check.Finding != "" {
			card.Checked = append(card.Checked, check)
		}
	}
	for _, step := range briefItems(brief.AttemptedSteps) {
		card.Checked = append(card.Checked, SupportHandoffCheck{Finding: step})
	}
	card.NothingChecked = brief.CheckedKnown && len(card.Checked) == 0

	note := supportControlNote(conv, "", renderSupportHandoffContent(card), now)
	agentAuthored := brief.Issue != "" || len(brief.AttemptedSteps) > 0 || len(brief.Checked) > 0 || len(brief.UnresolvedQuestions) > 0
	meta, _ := json.Marshal(map[string]any{"ai_handoff_brief": true, "reason": reason, "source_message_ids": refs, "agent_authored": agentAuthored, "handoff": card})
	note.Metadata = string(meta)
	return note
}

// renderSupportHandoffContent is the plain-text copy of the card for search,
// notifications and clients that predate the structured card.
func renderSupportHandoffContent(card supportHandoffCard) string {
	var b strings.Builder
	b.WriteString("AI handoff")
	if card.Issue != "" {
		b.WriteString("\n\n" + card.Issue)
	}
	b.WriteString("\n\nReason\n" + card.Reason)
	if lines := supportHandoffCheckLines(card.Checked); lines != "" {
		b.WriteString("\n\nAI checked\n" + lines)
	} else if card.NothingChecked {
		b.WriteString("\n\nAI checked\nNothing.")
	}
	if len(card.OpenQuestions) > 0 {
		b.WriteString("\n\nOpen question\n" + briefList(card.OpenQuestions))
	}
	if len(card.UnavailableData) > 0 {
		b.WriteString("\n\nCouldn't check\n" + briefList(card.UnavailableData))
	}
	if card.HeldReply != "" || card.HelpArticleGap != "" {
		b.WriteString("\n\nNext steps")
		if card.HeldReply != "" {
			b.WriteString("\n\nReply (not sent):\n\n" + card.HeldReply)
		}
		if card.HelpArticleGap != "" {
			b.WriteString("\n\nDocs: " + card.HelpArticleGap)
		}
	}
	return b.String()
}

func supportHandoffCheckLines(checks []SupportHandoffCheck) string {
	var lines []string
	for _, check := range checks {
		switch {
		case check.Source != "" && check.Finding != "":
			lines = append(lines, "- "+check.Source+": "+check.Finding)
		case check.Source != "":
			lines = append(lines, "- "+check.Source)
		case check.Finding != "":
			lines = append(lines, "- "+check.Finding)
		}
	}
	return strings.Join(lines, "\n")
}

// supportHandoffReasonText turns a machine handoff reason into a sentence a
// teammate can act on. Unknown reasons keep their readable code.
func supportHandoffReasonText(reason string) string {
	switch {
	case reason == "customer_requested" || reason == "customer_requested_human" || reason == "requested":
		return "The customer asked to talk to a person."
	case reason == "cannot_answer" || reason == "evidence_incomplete" || reason == "no_retrieval" || reason == "weak_retrieval":
		return "AI couldn't find a confirmed answer."
	case reason == "out_of_scope":
		return "This request is outside what AI is set up to handle."
	case reason == "risky_request":
		return "This needs a teammate's judgement, for example billing, account or legal matters."
	case reason == "billing_topic":
		return "Billing questions need a teammate."
	case reason == "agent_requested":
		return "AI decided a teammate should take over."
	case reason == "low_confidence":
		return "AI wasn't confident enough to send its answer."
	case reason == "mcp_reply_not_supported":
		return "AI's answer about the account wasn't backed by what it found, so it wasn't sent."
	case strings.HasPrefix(reason, "answer_validation_"):
		return "AI's answer didn't pass its source check, so it wasn't sent."
	case reason == "provider_unavailable" || reason == "llm_provider_unavailable":
		return "The AI service was unavailable, so AI didn't reply."
	case reason == "token_budget_exhausted":
		return "The workspace's AI usage limit was reached, so AI didn't reply."
	case reason == "max_followups_reached":
		return "AI reached its limit of replies in this conversation."
	case reason == "agent_no_reply":
		return "AI stopped without giving an answer."
	case reason == "stuck":
		return "AI got stuck and couldn't finish."
	case reason == "action_unavailable":
		return "AI can't do what the customer asked."
	case reason == "policy_blocked":
		return "Workspace rules don't let AI handle this."
	case reason == "ai_pipeline_error" || reason == "context_unavailable" || reason == "answer_generation_failed" || reason == "planner_unavailable":
		return "AI ran into a problem and couldn't reply."
	}
	return briefText(strings.ReplaceAll(reason, "_", " "))
}
