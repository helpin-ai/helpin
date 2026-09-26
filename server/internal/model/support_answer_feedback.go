package model

import (
	"encoding/json"
	"time"
)

// SupportAnswerFeedback is the visitor's saved assessment of one public AI answer.
type SupportAnswerFeedback struct {
	Helpful     bool      `json:"helpful"`
	SubmittedAt time.Time `json:"submitted_at"`
}

// VisitorFeedback returns validated, server-persisted feedback from message metadata.
func (m SupportMessage) VisitorFeedback() *SupportAnswerFeedback {
	var metadata struct {
		Feedback *struct {
			Helpful     *bool     `json:"helpful"`
			SubmittedAt time.Time `json:"submitted_at"`
		} `json:"visitor_feedback"`
	}
	if json.Unmarshal([]byte(m.Metadata), &metadata) != nil || metadata.Feedback == nil || metadata.Feedback.Helpful == nil || metadata.Feedback.SubmittedAt.IsZero() {
		return nil
	}
	return &SupportAnswerFeedback{Helpful: *metadata.Feedback.Helpful, SubmittedAt: metadata.Feedback.SubmittedAt}
}

// CanReceiveVisitorFeedback excludes private, human and automated holding messages.
func (m SupportMessage) CanReceiveVisitorFeedback() bool {
	if !m.WidgetVisible() || (m.MessageType != "reply" && m.MessageType != "") || m.Content == "" {
		return false
	}
	var metadata struct {
		AIAgentID string `json:"ai_agent_id"`
		Delayed   bool   `json:"delayed_team_reply"`
	}
	if m.Metadata != "" && json.Unmarshal([]byte(m.Metadata), &metadata) != nil {
		return false
	}
	return !metadata.Delayed && (m.SenderType == "ai" || (m.SenderType == "agent" && metadata.AIAgentID != ""))
}

// WidgetAnswerFeedbackRequest is authenticated with the visitor's widget session.
type WidgetAnswerFeedbackRequest struct {
	SessionToken string `json:"session_token"`
	Helpful      *bool  `json:"helpful"`
}
