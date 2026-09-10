package model

import "time"

// PMAISuggestionItem exposes only personal meeting follow-up context, never CRM records.
type PMAISuggestionItem struct {
	ID           string     `json:"id"`
	Title        string     `json:"title"`
	MeetingID    string     `json:"meeting_id"`
	MeetingTitle string     `json:"meeting_title"`
	MeetingAt    *time.Time `json:"meeting_at"`
	CreatedAt    time.Time  `json:"created_at"`
}

type PMAISuggestionDetail struct {
	PMAISuggestionItem
	DraftSubject string `json:"draft_subject"`
	DraftBody    string `json:"draft_body"`
	Revision     string `json:"revision"`
}

type PMAISuggestionDecision struct {
	Revision string `json:"revision"`
}

type PMAISuggestionDecisionResult struct {
	ID              string `json:"id"`
	Status          string `json:"status"`
	ExecutionStatus string `json:"execution_status"`
}
