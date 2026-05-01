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
