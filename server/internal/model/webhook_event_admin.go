package model

// WebhookEventListRequest holds query parameters for listing webhook events.
type WebhookEventListRequest struct {
	Page      int    `json:"page"`
	PerPage   int    `json:"per_page"`
	EventType string `json:"event_type,omitempty"`
	Provider  string `json:"provider,omitempty"`
}

// WebhookEventListResponse is the paginated response for webhook events.
type WebhookEventListResponse struct {
	Data       []SupportEmailWebhookEvent `json:"data"`
	Page       int                        `json:"page"`
	PerPage    int                        `json:"per_page"`
	Total      int64                      `json:"total"`
	TotalPages int                        `json:"total_pages"`
}
