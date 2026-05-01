package model

// EmailDiagnosticsConfig exposes non-secret support email configuration for
// platform-admin troubleshooting.
type EmailDiagnosticsConfig struct {
	AppEmailConfigured        bool   `json:"app_email_configured"`
	ReplyEmailConfigured      bool   `json:"reply_email_configured"`
	RouteEmailConfigured      bool   `json:"route_email_configured"`
	RedisConfigured           bool   `json:"redis_configured"`
	FallbackPollerEnabled     bool   `json:"fallback_poller_enabled"`
	AppFromEmail              string `json:"app_from_email,omitempty"`
	ReplyFromEmail            string `json:"reply_from_email,omitempty"`
	SupportEmailReplyDomain   string `json:"support_email_reply_domain"`
	SupportEmailRouteDomain   string `json:"support_email_route_domain"`
	ReplyInboundSecretSet     bool   `json:"reply_inbound_secret_set"`
	RouteInboundSecretSet     bool   `json:"route_inbound_secret_set"`
	ExpectedFallbackFromShape string `json:"expected_fallback_from_shape"`
	ExpectedReplyToShape      string `json:"expected_reply_to_shape"`
}

// EmailLogCount is a grouped support email log count.
type EmailLogCount struct {
	Direction string `json:"direction"`
	Status    string `json:"status"`
	Count     int64  `json:"count"`
}

// EmailDiagnosticsResponse is the platform-admin support email visibility
// payload.
type EmailDiagnosticsResponse struct {
	Config         EmailDiagnosticsConfig     `json:"config"`
	Queue          *EmailQueueResponse        `json:"queue"`
	RecentLogs     []SupportEmailLog          `json:"recent_logs"`
	LogCounts      []EmailLogCount            `json:"log_counts"`
	RecentWebhooks []SupportEmailWebhookEvent `json:"recent_webhooks"`
}

// EmailFallbackConversationDiagnosticsResponse explains why outbound replies in
// a support conversation did or did not enter the email fallback path.
type EmailFallbackConversationDiagnosticsResponse struct {
	ConversationID    string                                   `json:"conversation_id"`
	WorkspaceID       string                                   `json:"workspace_id"`
	Subject           string                                   `json:"subject"`
	Status            string                                   `json:"status"`
	CustomerEmail     string                                   `json:"customer_email,omitempty"`
	EmailUnsubscribed bool                                     `json:"email_unsubscribed"`
	ContactLastSeenAt *string                                  `json:"contact_last_seen_at,omitempty"`
	VisitorOnline     bool                                     `json:"visitor_online"`
	Settings          EmailFallbackConversationSettingsSummary `json:"settings"`
	Queue             EmailFallbackConversationQueueSummary    `json:"queue"`
	Messages          []EmailFallbackMessageDiagnostics        `json:"messages"`
}

type EmailFallbackConversationSettingsSummary struct {
	EmailFallbackEnabled            bool `json:"email_fallback_enabled"`
	EmailFallbackDelaySecs          int  `json:"email_fallback_delay_secs"`
	EmailFallbackMaxDeliveryAgeSecs int  `json:"email_fallback_max_delivery_age_secs"`
}

type EmailFallbackConversationQueueSummary struct {
	Queued             bool     `json:"queued"`
	FireAt             *string  `json:"fire_at,omitempty"`
	MessageIDs         []string `json:"message_ids,omitempty"`
	RedisChecked       bool     `json:"redis_checked"`
	RedisError         string   `json:"redis_error,omitempty"`
	DelayRemainingSecs int      `json:"delay_remaining_secs"`
}

type EmailFallbackMessageDiagnostics struct {
	ID                 string   `json:"id"`
	CreatedAt          string   `json:"created_at"`
	SenderType         string   `json:"sender_type"`
	MessageType        string   `json:"message_type"`
	IsInternal         bool     `json:"is_internal"`
	ContentPreview     string   `json:"content_preview,omitempty"`
	CancellableUntil   *string  `json:"cancellable_until,omitempty"`
	EmailNotifiedAt    *string  `json:"email_notified_at,omitempty"`
	EmailReadAt        *string  `json:"email_read_at,omitempty"`
	EmailLogID         string   `json:"email_log_id,omitempty"`
	EmailLogStatus     string   `json:"email_log_status,omitempty"`
	PostmarkMessageID  string   `json:"postmark_message_id,omitempty"`
	Eligible           bool     `json:"eligible"`
	Queued             bool     `json:"queued"`
	Due                bool     `json:"due"`
	ReconcileCandidate bool     `json:"reconcile_candidate"`
	Reasons            []string `json:"reasons"`
}
