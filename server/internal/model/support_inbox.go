package model

import (
	"time"
)

// SupportConversation represents a support conversation (renamed from SupportTicket).
type SupportConversation struct {
	ID                string     `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID       string     `json:"workspace_id" gorm:"type:uuid;not null;index"`
	DisplayID         int        `json:"display_id" gorm:"not null;index"`
	Subject           string     `json:"subject" gorm:"not null"`
	Status            string     `json:"status" gorm:"not null;default:'open'"`     // open, in_progress, waiting, resolved, closed
	Priority          string     `json:"priority" gorm:"not null;default:'medium'"` // low, medium, high, urgent
	Channel           string     `json:"channel" gorm:"not null;default:'widget'"`  // widget, internal, email, api
	CustomerName      *string    `json:"customer_name"`
	CustomerEmail     *string    `json:"customer_email"`
	CustomerPhone     *string    `json:"customer_phone"`
	OpenedByUserID    *string    `json:"opened_by_user_id" gorm:"type:uuid"`
	AssignedAgentID   *string    `json:"assigned_agent_id" gorm:"type:uuid"`
	LinkedStoryID     *string    `json:"linked_story_id" gorm:"type:uuid"`
	Source            string     `json:"source" gorm:"not null;default:'internal'"` // widget, internal, email, api - kept for backward compat
	AnonymousID       *string    `json:"anonymous_id" gorm:"index"`
	CRMContactID      *string    `json:"crm_contact_id" gorm:"type:uuid;index"`
	ResolvedAt        *time.Time `json:"resolved_at"`
	ClosedAt          *time.Time `json:"closed_at"`
	TeamLastSeenAt    *time.Time `json:"team_last_seen_at" gorm:"type:timestamptz"`
	ContactLastSeenAt *time.Time `json:"contact_last_seen_at" gorm:"type:timestamptz"`
	CreatedAt         time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt         time.Time  `json:"updated_at" gorm:"autoUpdateTime"`

	// Virtual fields — populated by SELECT subqueries, not stored as columns.
	LastMessage *string `json:"last_message,omitempty" gorm:"->"`
	UnreadCount int     `json:"unread_count" gorm:"->"`
}

func (SupportConversation) TableName() string { return "support_conversations" }

// UnreadStats holds aggregate unread conversation counts for sidebar badges.
type UnreadStats struct {
	Total      int `json:"total"`
	MyInbox    int `json:"my_inbox"`
	Unassigned int `json:"unassigned"`
}

// ConversationListMeta holds metadata returned alongside paginated conversation lists.
type ConversationListMeta struct {
	Unread UnreadStats `json:"unread"`
}

// ConversationListResponse is the paginated conversation list with unread metadata.
type ConversationListResponse struct {
	Data       []SupportConversation `json:"data"`
	Total      int                   `json:"total"`
	Page       int                   `json:"page"`
	PerPage    int                   `json:"per_page"`
	TotalPages int                   `json:"total_pages"`
	Meta       ConversationListMeta  `json:"meta"`
}

// SupportMessage represents a message within a support conversation.
type SupportMessage struct {
	ID                string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID       string    `json:"workspace_id" gorm:"type:uuid;not null;index"`
	ConversationID    string    `json:"conversation_id" gorm:"type:uuid;index"`
	SenderType        string    `json:"sender_type" gorm:"not null"`                  // customer, user, agent, ai
	MessageType       string    `json:"message_type" gorm:"not null;default:'reply'"` // reply, csat_survey, system
	SenderUserID      *string   `json:"sender_user_id" gorm:"type:uuid"`
	SenderAgentID     *string   `json:"sender_agent_id" gorm:"type:uuid"`
	SenderDisplayName *string   `json:"sender_display_name"`
	SenderAvatarURL   *string   `json:"sender_avatar_url"`
	Content           string    `json:"content" gorm:"not null"`
	IsInternal        bool      `json:"is_internal" gorm:"not null;default:false"`
	Metadata          string    `json:"metadata" gorm:"type:jsonb;default:'{}'"` // JSONB for CSAT ratings, AI sources, etc.
	CreatedAt         time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt         time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (SupportMessage) TableName() string { return "support_messages" }

// SupportCannedResponse represents a canned response for quick replies.
type SupportCannedResponse struct {
	ID          string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID string    `json:"workspace_id" gorm:"type:uuid;not null;index"`
	ShortCode   string    `json:"short_code" gorm:"not null"` // e.g., "greeting", "thanks"
	Title       string    `json:"title" gorm:"not null"`
	Content     string    `json:"content" gorm:"not null"`
	CreatedByID string    `json:"created_by_id" gorm:"type:uuid"`
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (SupportCannedResponse) TableName() string { return "support_canned_responses" }

// SupportWidgetInstallation holds workspace-level widget configuration.
type SupportWidgetInstallation struct {
	ID          string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID string    `json:"workspace_id" gorm:"type:uuid;not null;uniqueIndex"`
	WidgetKey   string    `json:"widget_key" gorm:"not null"` // public key for embedding
	SecretKey   string    `json:"-" gorm:"not null"`          // for signing session tokens
	Settings    string    `json:"settings" gorm:"type:jsonb;not null;default:'{}'"`
	Active      bool      `json:"active" gorm:"not null;default:true"`
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (SupportWidgetInstallation) TableName() string { return "support_widget_installations" }

// SupportWidgetSession represents an external widget chat session (30-day TTL).
type SupportWidgetSession struct {
	ID             string     `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID    string     `json:"workspace_id" gorm:"type:uuid;not null;index"`
	ConversationID *string    `json:"conversation_id" gorm:"type:uuid;index"`
	SessionToken   string     `json:"-" gorm:"not null;uniqueIndex"`
	AnonymousID    string     `json:"anonymous_id" gorm:"not null"`
	IsAnonymous    bool       `json:"is_anonymous" gorm:"default:true"`
	CustomerName   *string    `json:"customer_name"`
	CustomerEmail  *string    `json:"customer_email"`
	CustomerPhone  *string    `json:"customer_phone"`
	UserAgent      *string    `json:"-"`
	LastPageURL    *string    `json:"last_page_url"`
	Timezone       *string    `json:"timezone"`
	Locale         *string    `json:"locale"`
	RevokedAt      *time.Time `json:"-" gorm:"index"`
	ExpiresAt      time.Time  `json:"expires_at" gorm:"not null"`
	CreatedAt      time.Time  `json:"created_at" gorm:"autoCreateTime"`
}

func (SupportWidgetSession) TableName() string { return "support_widget_sessions" }

// CreateConversationRequest is the payload for creating a support conversation.
type CreateConversationRequest struct {
	WorkspaceID   string  `json:"workspace_id"`
	Subject       string  `json:"subject"`
	Priority      string  `json:"priority"`
	CustomerName  *string `json:"customer_name"`
	CustomerEmail *string `json:"customer_email"`
	Source        string  `json:"source"`
}

// CreateMessageRequest is the payload for creating a support message.
type CreateMessageRequest struct {
	Content     string `json:"content"`
	IsInternal  bool   `json:"is_internal"`
	MessageType string `json:"message_type"` // reply, csat_survey, system
}

// LinkStoryRequest links a conversation to a story.
type LinkStoryRequest struct {
	StoryID string `json:"story_id"`
}

// AssignConversationAgentRequest assigns an agent to a conversation.
type AssignConversationAgentRequest struct {
	AgentID string `json:"agent_id"`
}

// UpdateConversationStatusRequest changes conversation status.
type UpdateConversationStatusRequest struct {
	Status string `json:"status"`
}

// WidgetSessionRequest creates a new widget session (legacy HTTP).
type WidgetSessionRequest struct {
	WorkspaceSlug string  `json:"workspace_slug"`
	WidgetKey     string  `json:"widget_key"`
	CustomerName  *string `json:"customer_name"`
	CustomerEmail *string `json:"customer_email"`
}

// WidgetMessageRequest sends a message via widget (legacy HTTP).
type WidgetMessageRequest struct {
	SessionToken string `json:"session_token"`
	Content      string `json:"content"`
}

// WidgetTypingRequest sends a typing indicator via widget HTTP fallback.
type WidgetTypingRequest struct {
	SessionToken string `json:"session_token"`
	IsTyping     bool   `json:"is_typing"`
}

// WidgetSessionRevokeRequest revokes a widget session (HTTP fallback for shutdown).
type WidgetSessionRevokeRequest struct {
	SessionToken string `json:"session_token"`
}

// ── WS Message Types ─────────────────────────────────────────────────

// WidgetWSMessage is the envelope for all widget WS messages.
type WidgetWSMessage struct {
	Type string         `json:"type"`
	Data map[string]any `json:"data,omitempty"`
}

// WidgetSessionCreateData is the payload for session:create.
type WidgetSessionCreateData struct {
	AnonymousID string `json:"anonymous_id"`
	PageURL     string `json:"page_url"`
	PageTitle   string `json:"page_title"`
	UserAgent   string `json:"user_agent"`
	Timezone    string `json:"timezone"`
	Locale      string `json:"locale"`
}

// WidgetSessionRestoreData is the payload for session:restore.
type WidgetSessionRestoreData struct {
	SessionToken string `json:"session_token"`
}

// WidgetSessionUpgradeData is the payload for session:upgrade.
type WidgetSessionUpgradeData struct {
	Email  string `json:"email"`
	Name   string `json:"name"`
	Source string `json:"source"` // "widget_prechat", "sdk_identify", or "sdk_lead"
}

// WidgetIdentifyRequest is the HTTP payload for POST /api/widget/identify (headless SDK path).
type WidgetIdentifyRequest struct {
	APIKey      string `json:"api_key"`
	AnonymousID string `json:"anonymous_id"`
	Email       string `json:"email"`
	Name        string `json:"name"`
	Source      string `json:"source"` // "widget_prechat", "sdk_identify", or "sdk_lead"
}

// WidgetMessageSendData is the payload for message:send.
type WidgetMessageSendData struct {
	Content string `json:"content"`
}

// WidgetTypingData is the payload for typing:start / typing:stop.
type WidgetTypingData struct {
	Content string `json:"content,omitempty"`
}

// WidgetConversationSelectData is the payload for conversation:select.
type WidgetConversationSelectData struct {
	ConversationID string `json:"conversation_id"`
}

// WidgetSessionJoinedPayload is sent to the client after session:create or session:restore.
type WidgetSessionJoinedPayload struct {
	SessionToken  string                `json:"session_token"`
	ExpiresAt     string                `json:"expires_at"`
	IsAnonymous   bool                  `json:"is_anonymous"`
	CustomerEmail string                `json:"customer_email,omitempty"`
	Conversations []SupportConversation `json:"conversations"`
	Messages      []SupportMessage      `json:"messages"`
}

// WidgetMessageReceivedPayload is sent to widget clients for new messages.
type WidgetMessageReceivedPayload struct {
	ID             string  `json:"id"`
	ConversationID string  `json:"conversation_id"`
	Content        string  `json:"content"`
	SenderType     string  `json:"sender_type"`
	SenderName     *string `json:"sender_name"`
	SenderAvatar   *string `json:"sender_avatar"`
	CreatedAt      string  `json:"created_at"`
}

// CannedResponseRequest is the payload for CRUD operations on canned responses.
type CannedResponseRequest struct {
	ShortCode string `json:"short_code"`
	Title     string `json:"title"`
	Content   string `json:"content"`
}

// TypingIndicatorRequest represents a typing indicator event.
type TypingIndicatorRequest struct {
	IsTyping bool   `json:"is_typing"`
	Content  string `json:"content,omitempty"`
}

// ViewingPresenceRequest represents a viewing presence event.
type ViewingPresenceRequest struct {
	Viewing bool `json:"viewing"`
}

// CsatSurveyRequest represents a CSAT rating submission.
type CsatSurveyRequest struct {
	Rating   int    `json:"rating"`
	Feedback string `json:"feedback"`
}

// BusinessHoursDay defines a single day's business hours.
type BusinessHoursDay struct {
	Start   string `json:"start"` // "09:00"
	End     string `json:"end"`   // "17:00"
	Enabled bool   `json:"enabled"`
}

// SupportInboxSettings holds all widget configuration stored as JSONB.
type SupportInboxSettings struct {
	// Identity Capture
	RequireEmailBeforeChat bool   `json:"require_email_before_chat"`
	RequirePhoneAfterEmail bool   `json:"require_phone_after_email"`
	WelcomeMessage         string `json:"welcome_message"`

	// CRM Integration
	AutoCreateCRMContact  bool   `json:"auto_create_crm_contact"`
	DefaultLifecycleStage string `json:"default_lifecycle_stage"` // subscriber, lead, opportunity
	AutoPromoteToLead     bool   `json:"auto_promote_to_lead"`

	// AI Auto-Reply
	AIEnabled             bool    `json:"ai_enabled"`
	AIAgentID             *string `json:"ai_agent_id"`
	AIConfidenceThreshold float64 `json:"ai_confidence_threshold"` // 0.0–1.0
	ShowTalkToHuman       bool    `json:"show_talk_to_human"`

	// Handoff Routing
	HandoffBehavior string  `json:"handoff_behavior"` // unassigned, assign_to_team, round_robin
	HandoffTeamID   *string `json:"handoff_team_id"`

	// Business Hours
	BusinessHoursEnabled  bool                        `json:"business_hours_enabled"`
	BusinessHoursTimezone string                      `json:"business_hours_timezone"` // IANA
	BusinessHoursSchedule map[string]BusinessHoursDay `json:"business_hours_schedule"` // mon-sun
	OutsideHoursMessage   string                      `json:"outside_hours_message"`

	// Widget Identity
	WidgetName         string   `json:"widget_name"`           // display name in widget header (defaults to workspace name)
	WidgetAvatarURL    string   `json:"widget_avatar_url"`     // custom avatar URL for the widget
	WidgetHelpSpaceIDs []string `json:"widget_help_space_ids"` // selected external-capable docs spaces shown in widget help

	// Branding
	BrandColor      string `json:"brand_color"` // hex "#6366F1"
	ShowBranding    bool   `json:"show_branding"`
	ColorScheme     string `json:"color_scheme"`      // system, light, dark
	ButtonColor     string `json:"button_color"`      // hex "#000000"
	ButtonIconColor string `json:"button_icon_color"` // hex "#FFFFFF"
	LogoURL         string `json:"logo_url"`          // URL to workspace logo for widget header

	// Launcher
	LauncherPosition string `json:"launcher_position"` // bottom_right, bottom_left
	LauncherIcon     string `json:"launcher_icon"`     // chat_bubble, question_mark, help

	// CSAT
	CSATEnabled bool `json:"csat_enabled"`
}

// DefaultSupportInboxSettings returns settings with sensible defaults.
func DefaultSupportInboxSettings() SupportInboxSettings {
	return SupportInboxSettings{
		RequireEmailBeforeChat: true,
		RequirePhoneAfterEmail: false,
		WelcomeMessage:         "Hi there! How can we help you today?",
		AutoCreateCRMContact:   true,
		DefaultLifecycleStage:  "subscriber",
		AutoPromoteToLead:      false,
		AIEnabled:              false,
		AIAgentID:              nil,
		AIConfidenceThreshold:  0.7,
		ShowTalkToHuman:        true,
		HandoffBehavior:        "unassigned",
		HandoffTeamID:          nil,
		BusinessHoursEnabled:   false,
		BusinessHoursTimezone:  "America/New_York",
		BusinessHoursSchedule: map[string]BusinessHoursDay{
			"mon": {Start: "09:00", End: "17:00", Enabled: true},
			"tue": {Start: "09:00", End: "17:00", Enabled: true},
			"wed": {Start: "09:00", End: "17:00", Enabled: true},
			"thu": {Start: "09:00", End: "17:00", Enabled: true},
			"fri": {Start: "09:00", End: "17:00", Enabled: true},
			"sat": {Start: "09:00", End: "17:00", Enabled: false},
			"sun": {Start: "09:00", End: "17:00", Enabled: false},
		},
		OutsideHoursMessage: "We're currently offline. Leave a message and we'll get back to you!",
		WidgetName:          "",
		WidgetAvatarURL:     "",
		WidgetHelpSpaceIDs:  []string{},
		BrandColor:          "#6366F1",
		ShowBranding:        true,
		ColorScheme:         "light",
		ButtonColor:         "#000000",
		ButtonIconColor:     "#FFFFFF",
		LogoURL:             "",
		LauncherPosition:    "bottom_right",
		LauncherIcon:        "chat_bubble",
		CSATEnabled:         false,
	}
}

// UpdateInstallationSettingsRequest is a PATCH payload with pointer fields.
type UpdateInstallationSettingsRequest struct {
	RequireEmailBeforeChat *bool                       `json:"require_email_before_chat,omitempty"`
	RequirePhoneAfterEmail *bool                       `json:"require_phone_after_email,omitempty"`
	WelcomeMessage         *string                     `json:"welcome_message,omitempty"`
	AutoCreateCRMContact   *bool                       `json:"auto_create_crm_contact,omitempty"`
	DefaultLifecycleStage  *string                     `json:"default_lifecycle_stage,omitempty"`
	AutoPromoteToLead      *bool                       `json:"auto_promote_to_lead,omitempty"`
	AIEnabled              *bool                       `json:"ai_enabled,omitempty"`
	AIAgentID              *string                     `json:"ai_agent_id,omitempty"`
	AIConfidenceThreshold  *float64                    `json:"ai_confidence_threshold,omitempty"`
	ShowTalkToHuman        *bool                       `json:"show_talk_to_human,omitempty"`
	HandoffBehavior        *string                     `json:"handoff_behavior,omitempty"`
	HandoffTeamID          *string                     `json:"handoff_team_id,omitempty"`
	BusinessHoursEnabled   *bool                       `json:"business_hours_enabled,omitempty"`
	BusinessHoursTimezone  *string                     `json:"business_hours_timezone,omitempty"`
	BusinessHoursSchedule  map[string]BusinessHoursDay `json:"business_hours_schedule,omitempty"`
	OutsideHoursMessage    *string                     `json:"outside_hours_message,omitempty"`
	WidgetName             *string                     `json:"widget_name,omitempty"`
	WidgetAvatarURL        *string                     `json:"widget_avatar_url,omitempty"`
	WidgetHelpSpaceIDs     []string                    `json:"widget_help_space_ids,omitempty"`
	BrandColor             *string                     `json:"brand_color,omitempty"`
	ShowBranding           *bool                       `json:"show_branding,omitempty"`
	ColorScheme            *string                     `json:"color_scheme,omitempty"`
	ButtonColor            *string                     `json:"button_color,omitempty"`
	ButtonIconColor        *string                     `json:"button_icon_color,omitempty"`
	LogoURL                *string                     `json:"logo_url,omitempty"`
	LauncherPosition       *string                     `json:"launcher_position,omitempty"`
	LauncherIcon           *string                     `json:"launcher_icon,omitempty"`
	CSATEnabled            *bool                       `json:"csat_enabled,omitempty"`
}

// WidgetToken is the token format expected by the events-pipeline rust-capture service.
type WidgetToken struct {
	ID           string   `json:"id"`
	ClientSecret string   `json:"client_secret"`
	ServerSecret string   `json:"server_secret"`
	Origins      []string `json:"origins"`
}

// WidgetTokensResponse wraps the token list for the HTTP response.
type WidgetTokensResponse struct {
	Tokens []WidgetToken `json:"tokens"`
}

// WidgetConfigBranding matches the widget-core WidgetConfig.branding shape.
type WidgetConfigBranding struct {
	PrimaryColor    string `json:"primaryColor"`
	LogoURL         string `json:"logoUrl,omitempty"`
	WelcomeMessage  string `json:"welcomeMessage"`
	WidgetPosition  string `json:"widgetPosition"`
	ShowBranding    bool   `json:"showBranding"`
	LauncherIcon    string `json:"launcherIcon,omitempty"`
	ColorScheme     string `json:"colorScheme,omitempty"`
	ButtonColor     string `json:"buttonColor,omitempty"`
	ButtonIconColor string `json:"buttonIconColor,omitempty"`
}

// WidgetConfigFeatures matches the widget-core WidgetConfig.features shape.
type WidgetConfigFeatures struct {
	AIEnabled   bool `json:"aiEnabled"`
	FileUploads bool `json:"fileUploads"`
	PreChatForm bool `json:"preChatForm"`
	RequirePhone bool `json:"requirePhone"`
	CSATRating  bool `json:"csatRating"`
}

// WidgetHelpSpace is an external-capable docs space exposed to the widget help tab.
type WidgetHelpSpace struct {
	ID   string  `json:"id"`
	Name string  `json:"name"`
	Slug string  `json:"slug"`
	Icon *string `json:"icon,omitempty"`
}

// WidgetHelpCollection is a widget help collection row.
type WidgetHelpCollection struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Slug         string  `json:"slug"`
	Icon         *string `json:"icon,omitempty"`
	ArticleCount int     `json:"article_count"`
}

// WidgetHelpArticleSummary is a widget help article list row.
type WidgetHelpArticleSummary struct {
	ID      string  `json:"id"`
	Title   string  `json:"title"`
	Slug    string  `json:"slug"`
	Excerpt *string `json:"excerpt,omitempty"`
	Icon    *string `json:"icon,omitempty"`
}

// WidgetHelpArticle is a widget help article detail response.
type WidgetHelpArticle struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Slug        string  `json:"slug"`
	Excerpt     *string `json:"excerpt,omitempty"`
	Icon        *string `json:"icon,omitempty"`
	ContentHTML *string `json:"content_html"`
}

// WidgetConfigResponse is the public-facing widget config matching the
// TypeScript WidgetConfig interface in packages/shared/src/types/widget-config.ts.
type WidgetConfigResponse struct {
	WorkspaceID   string               `json:"workspaceId"`
	WorkspaceName string               `json:"workspaceName,omitempty"`
	Branding      WidgetConfigBranding `json:"branding"`
	Features      WidgetConfigFeatures `json:"features"`
	HelpSpaces    []WidgetHelpSpace    `json:"helpSpaces"`
}

// ── Visitor Context DTOs ─────────────────────────────────────────────

// VisitorDeviceInfo holds parsed user-agent data for visitor context.
type VisitorDeviceInfo struct {
	Browser        string `json:"browser"`
	BrowserVersion string `json:"browser_version"`
	OS             string `json:"os"`
	OSVersion      string `json:"os_version"`
	DeviceType     string `json:"device_type"` // desktop, mobile, tablet
}

// VisitorLocation holds geographic/locale data for visitor context.
type VisitorLocation struct {
	Timezone    *string `json:"timezone"`
	Locale      *string `json:"locale"`
	LastPageURL *string `json:"last_page_url"`
}

// VisitorContactData holds CRM contact details for visitor context.
type VisitorContactData struct {
	ID               string            `json:"id"`
	Name             *string           `json:"name"`
	Email            *string           `json:"email"`
	Phone            *string           `json:"phone"`
	JobTitle         *string           `json:"job_title"`
	LifecycleStage   string            `json:"lifecycle_stage"`
	LeadStatus       string            `json:"lead_status"`
	Source           string            `json:"source"`
	CustomProperties map[string]string `json:"custom_properties,omitempty"`
}

// VisitorOtherConversation is a compact summary for other conversations in visitor context.
type VisitorOtherConversation struct {
	ID        string `json:"id"`
	DisplayID int    `json:"display_id"`
	Subject   string `json:"subject"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}

// VisitorContextResponse assembles all visitor intelligence for a conversation.
type VisitorContextResponse struct {
	Device             *VisitorDeviceInfo         `json:"device,omitempty"`
	Location           *VisitorLocation           `json:"location,omitempty"`
	Contact            *VisitorContactData        `json:"contact,omitempty"`
	OtherConversations []VisitorOtherConversation `json:"other_conversations"`
	TotalConversations int                        `json:"total_conversations"`
	SessionCreatedAt   *string                    `json:"session_created_at,omitempty"`
}

// InstallationSettingsResponse wraps installation + parsed settings for the admin API.
type InstallationSettingsResponse struct {
	ID          string               `json:"id"`
	WorkspaceID string               `json:"workspace_id"`
	WidgetKey   string               `json:"widget_key"`
	Settings    SupportInboxSettings `json:"settings"`
	Active      bool                 `json:"active"`
	CreatedAt   string               `json:"created_at"`
	UpdatedAt   string               `json:"updated_at"`
}
