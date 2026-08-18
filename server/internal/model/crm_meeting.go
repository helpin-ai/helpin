package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

const (
	CRMMeetingProviderRecall = "recall"
	CRMMeetingProviderVexa   = "vexa"

	CRMMeetingPlatformGoogleMeet = "google_meet"
	CRMMeetingPlatformZoom       = "zoom"
	CRMMeetingPlatformTeams      = "teams"
	CRMMeetingPlatformWebex      = "webex"

	CRMMeetingStatusScheduled  = "scheduled"
	CRMMeetingStatusJoining    = "joining"
	CRMMeetingStatusWaiting    = "waiting"
	CRMMeetingStatusRecording  = "recording"
	CRMMeetingStatusFinalizing = "finalizing"
	CRMMeetingStatusProcessing = "processing"
	CRMMeetingStatusReady      = "ready"
	CRMMeetingStatusFailed     = "failed"
	CRMMeetingStatusCancelled  = "cancelled"

	CRMMeetingSummaryPending      = "pending"
	CRMMeetingSummaryProcessing   = "processing"
	CRMMeetingSummaryReady        = "ready"
	CRMMeetingSummaryBlockedUsage = "blocked_usage"
	CRMMeetingSummaryFailed       = "failed"
	CRMMeetingSummaryNotRequested = "not_requested"

	CRMMeetingVisibilityWorkspace    = "workspace"
	CRMMeetingVisibilityParticipants = "participants"
	CRMMeetingVisibilityPrivate      = "private"

	CRMMeetingActionPending   = "pending"
	CRMMeetingActionAccepted  = "accepted"
	CRMMeetingActionDismissed = "dismissed"
)

// CRMMeeting is Helpin's provider-neutral meeting record.
type CRMMeeting struct {
	ID                   string     `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID          string     `json:"workspace_id" gorm:"type:uuid;not null;index"`
	CalendarEventID      *string    `json:"calendar_event_id" gorm:"type:uuid;index"`
	ActivityID           *string    `json:"activity_id" gorm:"type:uuid;uniqueIndex"`
	OwnerMemberID        *string    `json:"owner_member_id" gorm:"type:uuid;index"`
	Title                string     `json:"title" gorm:"not null"`
	MeetingURL           string     `json:"meeting_url" gorm:"not null"`
	Platform             string     `json:"platform" gorm:"not null;index"`
	NativeMeetingID      string     `json:"native_meeting_id" gorm:"not null"`
	Status               string     `json:"status" gorm:"not null;default:'scheduled';index"`
	SummaryStatus        string     `json:"summary_status" gorm:"not null;default:'pending';index"`
	Visibility           string     `json:"visibility" gorm:"not null;default:'workspace'"`
	RecordAudio          bool       `json:"record_audio" gorm:"not null;default:false"`
	ScheduledStartAt     *time.Time `json:"scheduled_start_at" gorm:"index"`
	ScheduledEndAt       *time.Time `json:"scheduled_end_at"`
	ActualStartAt        *time.Time `json:"actual_start_at"`
	ActualEndAt          *time.Time `json:"actual_end_at"`
	DurationSeconds      int        `json:"duration_seconds" gorm:"not null;default:0"`
	Participants         JSONBlob   `json:"participants" gorm:"type:jsonb;default:'[]'"`
	FailureCode          *string    `json:"failure_code"`
	FailureMessage       *string    `json:"failure_message"`
	RecordingObjectKey   *string    `json:"recording_object_key"`
	RecordingContentType *string    `json:"recording_content_type"`
	CreatedBy            *string    `json:"created_by" gorm:"type:uuid;index"`
	CreatedAt            time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt            time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (CRMMeeting) TableName() string { return "crm_meetings" }

// CRMMeetingCapture records one immutable provider attempt for a meeting.
type CRMMeetingCapture struct {
	ID                    string     `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID           string     `json:"workspace_id" gorm:"type:uuid;not null;index;uniqueIndex:idx_crm_meeting_capture_idempotency,priority:1"`
	MeetingID             string     `json:"meeting_id" gorm:"type:uuid;not null;index"`
	Provider              string     `json:"-" gorm:"not null;index;uniqueIndex:idx_crm_meeting_capture_provider_id,priority:1"`
	ProviderCaptureID     string     `json:"-" gorm:"not null;uniqueIndex:idx_crm_meeting_capture_provider_id,priority:2"`
	ProviderStatus        string     `json:"-" gorm:"not null"`
	Status                string     `json:"status" gorm:"not null;index"`
	RequestIdempotencyKey string     `json:"-" gorm:"not null;uniqueIndex:idx_crm_meeting_capture_idempotency,priority:2"`
	ProviderRecordingID   *string    `json:"-"`
	ProviderTranscriptID  *string    `json:"-"`
	StartedAt             *time.Time `json:"started_at"`
	EndedAt               *time.Time `json:"ended_at"`
	FailureCode           *string    `json:"failure_code"`
	FailureMessage        *string    `json:"failure_message"`
	Metadata              JSONB      `json:"-" gorm:"type:jsonb;default:'{}'"`
	CreatedAt             time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt             time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (CRMMeetingCapture) TableName() string { return "crm_meeting_captures" }

// CRMMeetingTranscriptSegment is one normalized speaker-attributed utterance.
type CRMMeetingTranscriptSegment struct {
	ID           string  `json:"id"`
	SpeakerID    string  `json:"speaker_id,omitempty"`
	SpeakerName  string  `json:"speaker_name"`
	Text         string  `json:"text"`
	StartSeconds float64 `json:"start_seconds"`
	EndSeconds   float64 `json:"end_seconds"`
	Language     string  `json:"language,omitempty"`
	Confidence   float64 `json:"confidence,omitempty"`
}

// CRMMeetingTranscriptSegments persists normalized transcript segments as JSON.
type CRMMeetingTranscriptSegments []CRMMeetingTranscriptSegment

// Value implements driver.Valuer.
func (s CRMMeetingTranscriptSegments) Value() (driver.Value, error) {
	if s == nil {
		return "[]", nil
	}
	payload, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	return string(payload), nil
}

// Scan implements sql.Scanner.
func (s *CRMMeetingTranscriptSegments) Scan(value interface{}) error {
	return scanJSONArray(value, s)
}

// CRMMeetingTranscript is the canonical transcript copied into Helpin.
type CRMMeetingTranscript struct {
	ID             string                       `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID    string                       `json:"workspace_id" gorm:"type:uuid;not null;index"`
	MeetingID      string                       `json:"meeting_id" gorm:"type:uuid;not null;uniqueIndex"`
	CaptureID      string                       `json:"capture_id" gorm:"type:uuid;not null;index"`
	SourceProvider string                       `json:"-" gorm:"not null"`
	Language       *string                      `json:"language"`
	PlainText      string                       `json:"plain_text" gorm:"type:text;not null"`
	Segments       CRMMeetingTranscriptSegments `json:"segments" gorm:"type:jsonb;default:'[]'"`
	Checksum       string                       `json:"checksum" gorm:"not null;index"`
	CreatedAt      time.Time                    `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt      time.Time                    `json:"updated_at" gorm:"autoUpdateTime"`
}

func (CRMMeetingTranscript) TableName() string { return "crm_meeting_transcripts" }

// CRMMeetingIntelligence contains fixed-schema post-meeting AI output.
type CRMMeetingIntelligence struct {
	ID                  string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID         string    `json:"workspace_id" gorm:"type:uuid;not null;index"`
	MeetingID           string    `json:"meeting_id" gorm:"type:uuid;not null;uniqueIndex"`
	GenerationVersion   string    `json:"generation_version" gorm:"not null"`
	TranscriptChecksum  string    `json:"transcript_checksum" gorm:"not null"`
	SummaryMarkdown     string    `json:"summary_markdown" gorm:"type:text;not null;default:''"`
	ParticipantsContext JSONBlob  `json:"participants_context" gorm:"type:jsonb;default:'[]'"`
	KeyPoints           JSONBlob  `json:"key_points" gorm:"type:jsonb;default:'[]'"`
	Decisions           JSONBlob  `json:"decisions" gorm:"type:jsonb;default:'[]'"`
	Objections          JSONBlob  `json:"objections" gorm:"type:jsonb;default:'[]'"`
	Risks               JSONBlob  `json:"risks" gorm:"type:jsonb;default:'[]'"`
	OpenQuestions       JSONBlob  `json:"open_questions" gorm:"type:jsonb;default:'[]'"`
	NextSteps           JSONBlob  `json:"next_steps" gorm:"type:jsonb;default:'[]'"`
	Rapport             JSONBlob  `json:"rapport" gorm:"type:jsonb;default:'[]'"`
	FollowUpDraft       JSONB     `json:"follow_up_draft" gorm:"type:jsonb;default:'{}'"`
	CreatedAt           time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt           time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (CRMMeetingIntelligence) TableName() string { return "crm_meeting_intelligence" }

// CRMMeetingActionItem is a reviewable action extracted from a meeting.
type CRMMeetingActionItem struct {
	ID               string     `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID      string     `json:"workspace_id" gorm:"type:uuid;not null;index"`
	MeetingID        string     `json:"meeting_id" gorm:"type:uuid;not null;index"`
	Position         int        `json:"position" gorm:"not null"`
	Title            string     `json:"title" gorm:"not null"`
	Details          *string    `json:"details"`
	AssigneeName     *string    `json:"assignee_name"`
	AssigneeMemberID *string    `json:"assignee_member_id" gorm:"type:uuid"`
	DueDate          *time.Time `json:"due_date" gorm:"type:date"`
	Evidence         JSONB      `json:"evidence" gorm:"type:jsonb;default:'{}'"`
	Status           string     `json:"status" gorm:"not null;default:'pending';index"`
	TaskID           *string    `json:"task_id" gorm:"type:uuid"`
	CreatedAt        time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt        time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (CRMMeetingActionItem) TableName() string { return "crm_meeting_action_items" }

// CRMMeetingSettings controls workspace meeting capture and retention defaults.
type CRMMeetingSettings struct {
	WorkspaceID             string    `json:"workspace_id" gorm:"type:uuid;primaryKey"`
	Enabled                 bool      `json:"enabled" gorm:"not null;default:false"`
	DefaultProvider         string    `json:"-" gorm:"not null;default:'recall'"`
	BotName                 string    `json:"bot_name" gorm:"not null;default:'Helpin Notetaker'"`
	AutoJoinMode            string    `json:"auto_join_mode" gorm:"not null;default:'manual'"`
	RecordAudioByDefault    bool      `json:"record_audio_by_default" gorm:"not null;default:false"`
	DefaultVisibility       string    `json:"default_visibility" gorm:"not null;default:'workspace'"`
	IncludeInternal         bool      `json:"include_internal" gorm:"not null;default:false"`
	IncludePrivate          bool      `json:"include_private" gorm:"not null;default:false"`
	IncludeSolo             bool      `json:"include_solo" gorm:"not null;default:false"`
	TranscriptRetentionDays int       `json:"transcript_retention_days" gorm:"not null;default:365"`
	AudioRetentionDays      int       `json:"audio_retention_days" gorm:"not null;default:30"`
	CreatedAt               time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt               time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (CRMMeetingSettings) TableName() string { return "crm_meeting_settings" }

// CRMMeetingProviderEvent stores authenticated provider webhooks idempotently.
type CRMMeetingProviderEvent struct {
	ID          string     `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID string     `json:"workspace_id" gorm:"type:uuid;not null;index"`
	Provider    string     `json:"provider" gorm:"not null;uniqueIndex:idx_crm_meeting_provider_event,priority:1"`
	EventID     string     `json:"event_id" gorm:"not null;uniqueIndex:idx_crm_meeting_provider_event,priority:2"`
	EventType   string     `json:"event_type" gorm:"not null;index"`
	MeetingID   *string    `json:"meeting_id" gorm:"type:uuid;index"`
	CaptureID   *string    `json:"capture_id" gorm:"type:uuid;index"`
	Payload     JSONBlob   `json:"payload" gorm:"type:jsonb;not null"`
	ProcessedAt *time.Time `json:"processed_at"`
	CreatedAt   time.Time  `json:"created_at" gorm:"autoCreateTime"`
}

func (CRMMeetingProviderEvent) TableName() string { return "crm_meeting_provider_events" }

// CreateCRMMeetingRequest creates a scheduled or immediate meeting record.
type CreateCRMMeetingRequest struct {
	WorkspaceID      string     `json:"workspace_id"`
	Title            string     `json:"title"`
	MeetingURL       string     `json:"meeting_url"`
	CalendarEventID  *string    `json:"calendar_event_id"`
	OwnerMemberID    *string    `json:"owner_member_id"`
	ScheduledStartAt *time.Time `json:"scheduled_start_at"`
	ScheduledEndAt   *time.Time `json:"scheduled_end_at"`
	Visibility       *string    `json:"visibility"`
	RecordAudio      *bool      `json:"record_audio"`
	StartNow         bool       `json:"start_now"`
}

// UpdateCRMMeetingRequest updates editable meeting metadata.
type UpdateCRMMeetingRequest struct {
	Title            *string    `json:"title"`
	CalendarEventID  *string    `json:"calendar_event_id"`
	OwnerMemberID    *string    `json:"owner_member_id"`
	ScheduledStartAt *time.Time `json:"scheduled_start_at"`
	ScheduledEndAt   *time.Time `json:"scheduled_end_at"`
	Visibility       *string    `json:"visibility"`
	RecordAudio      *bool      `json:"record_audio"`
}

// CRMMeetingListFilters controls meeting list queries.
type CRMMeetingListFilters struct {
	Status        *string
	OwnerMemberID *string
	CompanyID     *string
	ContactID     *string
	DealID        *string
	Search        *string
	StartAfter    *time.Time
	StartBefore   *time.Time
}

// UpdateCRMMeetingSettingsRequest updates workspace meeting policy.
type UpdateCRMMeetingSettingsRequest struct {
	Enabled              *bool   `json:"enabled"`
	BotName              *string `json:"bot_name"`
	AutoJoinMode         *string `json:"auto_join_mode"`
	RecordAudioByDefault *bool   `json:"record_audio_by_default"`
}

// AcceptCRMMeetingActionItemRequest configures the canonical PM task to create.
type AcceptCRMMeetingActionItemRequest struct {
	TeamID          string  `json:"team_id"`
	WorkflowID      string  `json:"workflow_id"`
	WorkflowStateID string  `json:"workflow_state_id"`
	OwnerMemberID   *string `json:"owner_member_id"`
}

// CRMMeetingDetail is the complete provider-neutral meeting response.
type CRMMeetingDetail struct {
	Meeting      CRMMeeting               `json:"meeting"`
	Capture      *CRMMeetingCapture       `json:"capture"`
	Transcript   *CRMMeetingTranscript    `json:"transcript"`
	Intelligence *CRMMeetingIntelligence  `json:"intelligence"`
	ActionItems  []CRMMeetingActionItem   `json:"action_items"`
	Associations []CRMAssociationEnriched `json:"associations"`
}

// CRMMeetingRecording is a short-lived playback contract for canonical media.
type CRMMeetingRecording struct {
	URL         string `json:"url"`
	ContentType string `json:"content_type"`
	MediaType   string `json:"media_type"`
}

func scanJSONArray(value interface{}, target interface{}) error {
	if value == nil {
		return json.Unmarshal([]byte("[]"), target)
	}

	var payload []byte
	switch typed := value.(type) {
	case []byte:
		payload = typed
	case string:
		payload = []byte(typed)
	default:
		return fmt.Errorf("unsupported JSON array scan type %T", value)
	}
	return json.Unmarshal(payload, target)
}
