package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/meetingcapture"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

type meetingCaptureProvider interface {
	Name() string
	Configured() bool
	Supports(platform string) bool
	StartCapture(context.Context, meetingcapture.StartCaptureInput) (*meetingcapture.Capture, error)
	StopCapture(context.Context, string) error
	GetStatus(context.Context, string) (*meetingcapture.Capture, error)
	GetTranscript(context.Context, string) (*meetingcapture.Transcript, error)
	GetRecording(context.Context, string) (*meetingcapture.Recording, error)
	DeleteArtifacts(context.Context, string) error
	VerifyWebhook(http.Header, []byte) error
	NormalizeWebhook(http.Header, []byte) (*meetingcapture.ProviderEvent, error)
}

type meetingProcessingRunner interface {
	StartMeetingProcessing(ctx context.Context, workspaceID, meetingID string) error
}

type meetingRecordingStore interface {
	GeneratePresignedInlineGetURL(key string) (string, error)
	DeleteObject(ctx context.Context, key string) error
}

// CRMMeetingService owns meeting records, provider capture lifecycle, and review actions.
type CRMMeetingService struct {
	repo             *repository.CRMMeetingRepository
	associationRepo  *repository.CRMAssociationRepository
	calendarRepo     *repository.CRMCalendarRepository
	emailRepo        *repository.CRMEmailRepository
	taskService      *PMTaskService
	providers        map[string]meetingCaptureProvider
	captureProvider  string
	processing       meetingProcessingRunner
	recordingStore   meetingRecordingStore
	aiUsageMeter     *AIUsageMeter
	captureScheduler meetingCaptureScheduler
}

// NewCRMMeetingService creates the provider-neutral CRM meeting service.
func NewCRMMeetingService(
	repo *repository.CRMMeetingRepository,
	associationRepo *repository.CRMAssociationRepository,
	taskService *PMTaskService,
	providers ...meetingCaptureProvider,
) *CRMMeetingService {
	providerMap := make(map[string]meetingCaptureProvider, len(providers))
	for _, provider := range providers {
		if provider != nil {
			providerMap[provider.Name()] = provider
		}
	}
	return &CRMMeetingService{
		repo:            repo,
		associationRepo: associationRepo,
		taskService:     taskService,
		providers:       providerMap,
		captureProvider: model.CRMMeetingProviderRecall,
	}
}

// SetCaptureProvider selects the deployment-owned provider used for new capture attempts.
func (s *CRMMeetingService) SetCaptureProvider(name string) *CRMMeetingService {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		name = model.CRMMeetingProviderRecall
	}
	s.captureProvider = name
	return s
}

// SetProcessingRunner injects the product-owned Temporal processing launcher.
func (s *CRMMeetingService) SetProcessingRunner(runner meetingProcessingRunner) *CRMMeetingService {
	s.processing = runner
	return s
}

// SetRecordingStore injects canonical private recording storage.
func (s *CRMMeetingService) SetRecordingStore(store meetingRecordingStore) *CRMMeetingService {
	s.recordingStore = store
	return s
}

// SetAIUsageMeter enables launch-time billing preflight before a capture bot is created.
func (s *CRMMeetingService) SetAIUsageMeter(meter *AIUsageMeter) *CRMMeetingService {
	s.aiUsageMeter = meter
	return s
}

// List returns workspace-scoped meetings.
func (s *CRMMeetingService) List(
	ctx context.Context,
	workspaceID string,
	filters model.CRMMeetingListFilters,
	pagination model.PMPagination,
) ([]model.CRMMeeting, int64, error) {
	if strings.TrimSpace(workspaceID) == "" {
		return nil, 0, fmt.Errorf("workspace_id is required")
	}
	return s.repo.List(ctx, workspaceID, filters, pagination)
}

// Get returns a complete provider-neutral meeting detail.
func (s *CRMMeetingService) Get(ctx context.Context, workspaceID, meetingID string) (*model.CRMMeetingDetail, error) {
	meeting, err := s.repo.GetByID(ctx, workspaceID, meetingID)
	if err != nil {
		return nil, err
	}
	if meeting == nil {
		return nil, fmt.Errorf("meeting not found")
	}
	capture, err := s.repo.GetLatestCapture(ctx, workspaceID, meetingID)
	if err != nil {
		return nil, err
	}
	transcript, err := s.repo.GetTranscript(ctx, workspaceID, meetingID)
	if err != nil {
		return nil, err
	}
	intelligence, err := s.repo.GetIntelligence(ctx, workspaceID, meetingID)
	if err != nil {
		return nil, err
	}
	actionItems, err := s.repo.ListActionItems(ctx, workspaceID, meetingID)
	if err != nil {
		return nil, err
	}
	associations, err := s.associationRepo.ListByObjectEnriched(ctx, workspaceID, model.CRMObjectMeeting, meetingID)
	if err != nil {
		return nil, err
	}
	if actionItems == nil {
		actionItems = []model.CRMMeetingActionItem{}
	}
	if associations == nil {
		associations = []model.CRMAssociationEnriched{}
	}
	return &model.CRMMeetingDetail{
		Meeting:      *meeting,
		Capture:      capture,
		Transcript:   transcript,
		Intelligence: intelligence,
		ActionItems:  actionItems,
		Associations: associations,
	}, nil
}

// Create validates a meeting URL and creates a scheduled meeting record.
func (s *CRMMeetingService) Create(
	ctx context.Context,
	req model.CreateCRMMeetingRequest,
	actorID, idempotencyKey string,
) (*model.CRMMeetingDetail, error) {
	if strings.TrimSpace(req.WorkspaceID) == "" || strings.TrimSpace(req.MeetingURL) == "" {
		return nil, fmt.Errorf("workspace_id and meeting_url are required")
	}
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if req.StartNow {
		if idempotencyKey == "" {
			return nil, fmt.Errorf("Idempotency-Key header is required")
		}
		prior, lookupErr := s.repo.GetCaptureByIdempotencyKey(ctx, req.WorkspaceID, idempotencyKey)
		if lookupErr != nil {
			return nil, lookupErr
		}
		if prior != nil {
			return s.Get(ctx, req.WorkspaceID, prior.MeetingID)
		}
	}
	platform, nativeMeetingID, err := ParseMeetingURL(req.MeetingURL)
	if err != nil {
		return nil, err
	}
	settings, err := s.repo.GetSettings(ctx, req.WorkspaceID)
	if err != nil {
		return nil, err
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = "Untitled meeting"
	}
	visibility := settings.DefaultVisibility
	if req.Visibility != nil {
		visibility = strings.TrimSpace(*req.Visibility)
	}
	if !validMeetingVisibility(visibility) {
		return nil, fmt.Errorf("invalid visibility")
	}
	recordAudio := settings.RecordAudioByDefault
	if req.RecordAudio != nil {
		recordAudio = *req.RecordAudio
	}
	if req.StartNow {
		if !settings.Enabled {
			return nil, fmt.Errorf("meeting intelligence is disabled for this workspace")
		}
		provider, providerErr := s.provider(s.captureProvider, platform)
		if providerErr != nil {
			return nil, providerErr
		}
		if err := s.preflightCapture(ctx, req.WorkspaceID, nativeMeetingID, idempotencyKey, provider); err != nil {
			return nil, err
		}
	}
	meetingURL := strings.TrimSpace(req.MeetingURL)
	meeting := &model.CRMMeeting{
		WorkspaceID:      req.WorkspaceID,
		CalendarEventID:  trimStringPtr(req.CalendarEventID),
		OwnerMemberID:    trimStringPtr(req.OwnerMemberID),
		Title:            title,
		MeetingURL:       meetingURL,
		Platform:         platform,
		NativeMeetingID:  nativeMeetingID,
		Status:           model.CRMMeetingStatusScheduled,
		SummaryStatus:    model.CRMMeetingSummaryPending,
		Visibility:       visibility,
		RecordAudio:      recordAudio,
		ScheduledStartAt: req.ScheduledStartAt,
		ScheduledEndAt:   req.ScheduledEndAt,
		Participants:     model.JSONBlob("[]"),
		CreatedBy:        trimStringPtr(&actorID),
	}
	if err := validateMeetingSchedule(meeting); err != nil {
		return nil, err
	}
	if err := s.repo.Create(ctx, meeting); err != nil {
		return nil, err
	}
	if req.StartNow {
		if _, err := s.StartCapture(ctx, req.WorkspaceID, meeting.ID, idempotencyKey); err != nil {
			prior, lookupErr := s.repo.GetCaptureByIdempotencyKey(ctx, req.WorkspaceID, idempotencyKey)
			if lookupErr == nil && prior != nil && prior.MeetingID != meeting.ID {
				if deleteErr := s.repo.Delete(ctx, req.WorkspaceID, meeting.ID); deleteErr != nil {
					slog.ErrorContext(ctx, "duplicate meeting cleanup failed", "workspace_id", req.WorkspaceID, "meeting_id", meeting.ID, "error", deleteErr)
				}
				return s.Get(ctx, req.WorkspaceID, prior.MeetingID)
			}
			return nil, err
		}
	}
	return s.Get(ctx, req.WorkspaceID, meeting.ID)
}

// Update updates meeting metadata while preserving provider attempt identity.
func (s *CRMMeetingService) Update(
	ctx context.Context,
	workspaceID, meetingID string,
	req model.UpdateCRMMeetingRequest,
) (*model.CRMMeetingDetail, error) {
	meeting, err := s.repo.GetByID(ctx, workspaceID, meetingID)
	if err != nil {
		return nil, err
	}
	if meeting == nil {
		return nil, fmt.Errorf("meeting not found")
	}
	if req.Title != nil {
		meeting.Title = strings.TrimSpace(*req.Title)
		if meeting.Title == "" {
			return nil, fmt.Errorf("title is required")
		}
	}
	if req.CalendarEventID != nil {
		meeting.CalendarEventID = trimStringPtr(req.CalendarEventID)
	}
	if req.OwnerMemberID != nil {
		meeting.OwnerMemberID = trimStringPtr(req.OwnerMemberID)
	}
	if req.ScheduledStartAt != nil {
		meeting.ScheduledStartAt = req.ScheduledStartAt
	}
	if req.ScheduledEndAt != nil {
		meeting.ScheduledEndAt = req.ScheduledEndAt
	}
	if req.Visibility != nil {
		visibility := strings.TrimSpace(*req.Visibility)
		if !validMeetingVisibility(visibility) {
			return nil, fmt.Errorf("invalid visibility")
		}
		meeting.Visibility = visibility
	}
	if req.RecordAudio != nil {
		meeting.RecordAudio = *req.RecordAudio
	}
	if err := validateMeetingSchedule(meeting); err != nil {
		return nil, err
	}
	if err := s.repo.Update(ctx, meeting); err != nil {
		return nil, err
	}
	return s.Get(ctx, workspaceID, meetingID)
}

// StartCapture starts the workspace-selected provider exactly once per idempotency key.
func (s *CRMMeetingService) StartCapture(
	ctx context.Context,
	workspaceID, meetingID, idempotencyKey string,
) (*model.CRMMeetingCapture, error) {
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if idempotencyKey == "" {
		return nil, fmt.Errorf("Idempotency-Key header is required")
	}
	var capture *model.CRMMeetingCapture
	err := s.repo.WithCaptureLaunchLock(ctx, workspaceID, idempotencyKey, func(lockedRepo *repository.CRMMeetingRepository) error {
		var launchErr error
		capture, launchErr = s.startCaptureLocked(ctx, lockedRepo, workspaceID, meetingID, idempotencyKey)
		return launchErr
	})
	return capture, err
}

func (s *CRMMeetingService) startCaptureLocked(
	ctx context.Context,
	repo *repository.CRMMeetingRepository,
	workspaceID, meetingID, idempotencyKey string,
) (*model.CRMMeetingCapture, error) {
	if prior, err := repo.GetCaptureByIdempotencyKey(ctx, workspaceID, idempotencyKey); err != nil {
		return nil, err
	} else if prior != nil {
		if prior.MeetingID != meetingID {
			return nil, fmt.Errorf("Idempotency-Key was already used for another meeting")
		}
		return prior, nil
	}
	meeting, err := repo.GetByID(ctx, workspaceID, meetingID)
	if err != nil {
		return nil, err
	}
	if meeting == nil {
		return nil, fmt.Errorf("meeting not found")
	}
	if captureAlreadyActive(meeting.Status) {
		return nil, fmt.Errorf("meeting capture is already active")
	}
	if meeting.Status != model.CRMMeetingStatusScheduled && meeting.Status != model.CRMMeetingStatusFailed {
		return nil, fmt.Errorf("meeting capture cannot start from %s status", meeting.Status)
	}
	settings, err := repo.GetSettings(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	if !settings.Enabled {
		return nil, fmt.Errorf("meeting intelligence is disabled for this workspace")
	}
	provider, err := s.provider(s.captureProvider, meeting.Platform)
	if err != nil {
		return nil, err
	}
	if err := s.preflightCapture(ctx, workspaceID, meetingID, idempotencyKey, provider); err != nil {
		return nil, err
	}
	captureResult, err := provider.StartCapture(ctx, meetingcapture.StartCaptureInput{
		WorkspaceID:     workspaceID,
		MeetingID:       meetingID,
		MeetingURL:      meeting.MeetingURL,
		Platform:        meeting.Platform,
		NativeMeetingID: meeting.NativeMeetingID,
		BotName:         settings.BotName,
		RecordAudio:     meeting.RecordAudio,
	})
	if err != nil {
		return nil, fmt.Errorf("start %s meeting capture: %w", provider.Name(), err)
	}
	capture := &model.CRMMeetingCapture{
		WorkspaceID:           workspaceID,
		MeetingID:             meetingID,
		Provider:              provider.Name(),
		ProviderCaptureID:     captureResult.ProviderCaptureID,
		ProviderStatus:        captureResult.ProviderStatus,
		Status:                captureResult.Status,
		RequestIdempotencyKey: idempotencyKey,
		Metadata:              model.JSONB{},
	}
	stopLaunchedCapture := func(cause error) error {
		if stopErr := provider.StopCapture(ctx, captureResult.ProviderCaptureID); stopErr != nil {
			slog.ErrorContext(ctx, "meeting capture compensation failed", "workspace_id", workspaceID, "meeting_id", meetingID, "provider", provider.Name(), "provider_capture_id", captureResult.ProviderCaptureID, "error", stopErr)
		}
		return cause
	}
	if err := repo.CreateCapture(ctx, capture); err != nil {
		return nil, stopLaunchedCapture(err)
	}
	meeting.Status = capture.Status
	meeting.FailureCode = nil
	meeting.FailureMessage = nil
	if err := repo.Update(ctx, meeting); err != nil {
		return nil, stopLaunchedCapture(err)
	}
	slog.InfoContext(ctx, "meeting capture started", "workspace_id", workspaceID, "meeting_id", meetingID, "capture_id", capture.ID, "provider", capture.Provider)
	return capture, nil
}

// StopCapture stops the current provider bot without changing provider ownership.
func (s *CRMMeetingService) StopCapture(ctx context.Context, workspaceID, meetingID string) (*model.CRMMeetingCapture, error) {
	meeting, err := s.repo.GetByID(ctx, workspaceID, meetingID)
	if err != nil {
		return nil, err
	}
	if meeting == nil {
		return nil, fmt.Errorf("meeting not found")
	}
	capture, err := s.repo.GetLatestCapture(ctx, workspaceID, meetingID)
	if err != nil {
		return nil, err
	}
	if capture == nil {
		return nil, fmt.Errorf("meeting has no capture")
	}
	provider, err := s.provider(capture.Provider, meeting.Platform)
	if err != nil {
		return nil, err
	}
	if err := provider.StopCapture(ctx, capture.ProviderCaptureID); err != nil {
		return nil, fmt.Errorf("stop %s meeting capture: %w", capture.Provider, err)
	}
	capture.Status = model.CRMMeetingStatusFinalizing
	capture.ProviderStatus = "stop_requested"
	meeting.Status = model.CRMMeetingStatusFinalizing
	if err := s.repo.UpdateCapture(ctx, capture); err != nil {
		return nil, err
	}
	if err := s.repo.Update(ctx, meeting); err != nil {
		return nil, err
	}
	return capture, nil
}

// RetryProcessing restarts only transcript/intelligence processing, never capture.
func (s *CRMMeetingService) RetryProcessing(ctx context.Context, workspaceID, meetingID string) error {
	meeting, err := s.repo.GetByID(ctx, workspaceID, meetingID)
	if err != nil {
		return err
	}
	if meeting == nil {
		return fmt.Errorf("meeting not found")
	}
	if meeting.SummaryStatus != model.CRMMeetingSummaryFailed && meeting.SummaryStatus != model.CRMMeetingSummaryBlockedUsage {
		return fmt.Errorf("meeting processing can only be retried after a failure or usage block")
	}
	if s.processing == nil {
		return fmt.Errorf("meeting processing is unavailable")
	}
	meeting.SummaryStatus = model.CRMMeetingSummaryPending
	if err := s.repo.Update(ctx, meeting); err != nil {
		return err
	}
	return s.processing.StartMeetingProcessing(ctx, workspaceID, meetingID)
}

// GetRecording returns short-lived playback metadata for canonical media.
func (s *CRMMeetingService) GetRecording(ctx context.Context, workspaceID, meetingID string) (*model.CRMMeetingRecording, error) {
	meeting, err := s.repo.GetByID(ctx, workspaceID, meetingID)
	if err != nil {
		return nil, err
	}
	if meeting == nil || meeting.RecordingObjectKey == nil || strings.TrimSpace(*meeting.RecordingObjectKey) == "" {
		return nil, fmt.Errorf("recording not found")
	}
	if s.recordingStore == nil {
		return nil, fmt.Errorf("recording storage is unavailable")
	}
	url, err := s.recordingStore.GeneratePresignedInlineGetURL(*meeting.RecordingObjectKey)
	if err != nil {
		return nil, err
	}
	contentType := meetingRecordingContentType(meeting)
	mediaType := "file"
	if strings.HasPrefix(contentType, "video/") {
		mediaType = "video"
	} else if strings.HasPrefix(contentType, "audio/") {
		mediaType = "audio"
	}
	return &model.CRMMeetingRecording{URL: url, ContentType: contentType, MediaType: mediaType}, nil
}

func meetingRecordingContentType(meeting *model.CRMMeeting) string {
	if meeting != nil && meeting.RecordingContentType != nil && strings.TrimSpace(*meeting.RecordingContentType) != "" {
		return strings.ToLower(strings.TrimSpace(*meeting.RecordingContentType))
	}
	key := ""
	if meeting != nil && meeting.RecordingObjectKey != nil {
		key = strings.ToLower(strings.TrimSpace(*meeting.RecordingObjectKey))
	}
	switch {
	case strings.HasSuffix(key, ".mp4"):
		return "video/mp4"
	case strings.HasSuffix(key, ".webm"):
		return "audio/webm"
	default:
		return "audio/mpeg"
	}
}

// DeleteRecording removes only the canonical recording and keeps transcript intelligence.
func (s *CRMMeetingService) DeleteRecording(ctx context.Context, workspaceID, meetingID string) error {
	meeting, err := s.repo.GetByID(ctx, workspaceID, meetingID)
	if err != nil {
		return err
	}
	if meeting == nil {
		return fmt.Errorf("meeting not found")
	}
	if meeting.RecordingObjectKey == nil {
		return nil
	}
	if s.recordingStore == nil {
		return fmt.Errorf("recording storage is unavailable")
	}
	if err := s.recordingStore.DeleteObject(ctx, *meeting.RecordingObjectKey); err != nil {
		return err
	}
	meeting.RecordingObjectKey = nil
	meeting.RecordingContentType = nil
	return s.repo.Update(ctx, meeting)
}

// MeetingCaptureNotConfiguredMessage is the user-facing explanation returned
// when the server has no usable capture provider credentials.
const MeetingCaptureNotConfiguredMessage = "Meeting capture isn't set up on this server. Ask your server admin to configure a capture provider."

// IsMeetingCaptureNotConfigured reports whether err means the deployment has
// no usable capture provider credentials.
func IsMeetingCaptureNotConfigured(err error) bool {
	return errors.Is(err, meetingcapture.ErrNotConfigured)
}

// CaptureAvailability reports the deployment-selected capture provider and
// whether the server has the credentials it needs (API key and webhook secret).
func (s *CRMMeetingService) CaptureAvailability() (provider string, configured bool) {
	provider = s.captureProvider
	selected := s.providers[provider]
	return provider, selected != nil && selected.Configured()
}

// GetSettings returns user-manageable workspace meeting settings.
func (s *CRMMeetingService) GetSettings(ctx context.Context, workspaceID string) (*model.CRMMeetingSettings, error) {
	settings, err := s.repo.GetSettings(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	s.describeCapture(settings)
	return settings, nil
}

func (s *CRMMeetingService) describeCapture(settings *model.CRMMeetingSettings) {
	settings.CaptureProvider, settings.CaptureConfigured = s.CaptureAvailability()
}

// UpdateSettings validates and saves workspace meeting policy.
func (s *CRMMeetingService) UpdateSettings(
	ctx context.Context,
	workspaceID string,
	req model.UpdateCRMMeetingSettingsRequest,
) (*model.CRMMeetingSettings, error) {
	settings, err := s.repo.GetSettings(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	applyMeetingSettingsPatch(settings, req)
	settings.DefaultProvider = s.captureProvider
	if _, err := s.provider(s.captureProvider, model.CRMMeetingPlatformGoogleMeet); settings.Enabled && err != nil {
		return nil, err
	}
	if !validMeetingVisibility(settings.DefaultVisibility) {
		return nil, fmt.Errorf("invalid default_visibility")
	}
	if settings.BotName == "" || len(settings.BotName) > 100 {
		return nil, fmt.Errorf("bot_name must contain 1 to 100 characters")
	}
	if settings.AutoJoinMode != "manual" && settings.AutoJoinMode != "external" && settings.AutoJoinMode != "all" {
		return nil, fmt.Errorf("auto_join_mode must be manual, external, or all")
	}
	if settings.TranscriptRetentionDays < 1 || settings.TranscriptRetentionDays > 3650 || settings.AudioRetentionDays < 1 || settings.AudioRetentionDays > 3650 {
		return nil, fmt.Errorf("retention days must be between 1 and 3650")
	}
	if err := s.repo.UpsertSettings(ctx, settings); err != nil {
		return nil, err
	}
	s.describeCapture(settings)
	return settings, nil
}

// Delete removes provider artifacts when possible and then deletes the canonical meeting.
func (s *CRMMeetingService) Delete(ctx context.Context, workspaceID, meetingID string) error {
	meeting, err := s.repo.GetByID(ctx, workspaceID, meetingID)
	if err != nil {
		return err
	}
	if meeting == nil {
		return nil
	}
	if capture, captureErr := s.repo.GetLatestCapture(ctx, workspaceID, meetingID); captureErr != nil {
		return captureErr
	} else if capture != nil {
		if provider, providerErr := s.provider(capture.Provider, meeting.Platform); providerErr == nil {
			if deleteErr := provider.DeleteArtifacts(ctx, capture.ProviderCaptureID); deleteErr != nil {
				slog.WarnContext(ctx, "provider meeting artifact deletion failed", "workspace_id", workspaceID, "meeting_id", meetingID, "provider", capture.Provider, "error", deleteErr)
			}
		}
	}
	if meeting.RecordingObjectKey != nil && s.recordingStore != nil {
		if err := s.recordingStore.DeleteObject(ctx, *meeting.RecordingObjectKey); err != nil {
			return err
		}
	}
	return s.repo.Delete(ctx, workspaceID, meetingID)
}

func (s *CRMMeetingService) provider(name, platform string) (meetingCaptureProvider, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	provider := s.providers[name]
	if provider == nil || !provider.Configured() {
		return nil, fmt.Errorf("%s: %w", name, meetingcapture.ErrNotConfigured)
	}
	if !provider.Supports(platform) {
		return nil, fmt.Errorf("%s does not support %s meetings", name, platform)
	}
	return provider, nil
}

func (s *CRMMeetingService) preflightCapture(ctx context.Context, workspaceID, targetID, idempotencyKey string, provider meetingCaptureProvider) error {
	if s.aiUsageMeter == nil {
		return nil
	}
	return s.aiUsageMeter.Preflight(ctx, AIUsageMeterInput{
		WorkspaceID:    workspaceID,
		FeatureKey:     BillingFeatureMeetingIntelligence,
		IdempotencyKey: aiUsageIdempotencyKey(workspaceID, "meeting_capture", targetID, idempotencyKey, "preflight"),
		Metadata:       map[string]interface{}{"meeting_id": targetID, "provider": provider.Name()},
	})
}

// ParseMeetingURL validates supported providers and extracts a native meeting id.
func ParseMeetingURL(rawURL string) (string, string, error) {
	return meetingcapture.ParseMeetingURL(rawURL)
}

func validMeetingVisibility(value string) bool {
	// Participant/private policies remain reserved until participant identity
	// resolution can enforce them consistently across list, detail, and recording APIs.
	return value == model.CRMMeetingVisibilityWorkspace
}

func validateMeetingSchedule(meeting *model.CRMMeeting) error {
	if meeting.ScheduledStartAt != nil && meeting.ScheduledEndAt != nil && !meeting.ScheduledEndAt.After(*meeting.ScheduledStartAt) {
		return fmt.Errorf("scheduled_end_at must be after scheduled_start_at")
	}
	return nil
}

func captureAlreadyActive(status string) bool {
	switch status {
	case model.CRMMeetingStatusJoining, model.CRMMeetingStatusWaiting, model.CRMMeetingStatusRecording, model.CRMMeetingStatusFinalizing, model.CRMMeetingStatusProcessing:
		return true
	default:
		return false
	}
}

func trimStringPtr(value *string) *string {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	return &trimmed
}

func applyMeetingSettingsPatch(settings *model.CRMMeetingSettings, req model.UpdateCRMMeetingSettingsRequest) {
	if req.Enabled != nil {
		settings.Enabled = *req.Enabled
	}
	if req.BotName != nil {
		settings.BotName = strings.TrimSpace(*req.BotName)
	}
	if req.AutoJoinMode != nil {
		settings.AutoJoinMode = strings.ToLower(strings.TrimSpace(*req.AutoJoinMode))
	}
	if req.RecordAudioByDefault != nil {
		settings.RecordAudioByDefault = *req.RecordAudioByDefault
	}
}

func isMeetingUsageBlocked(err error) bool {
	return errors.Is(err, model.ErrAIUsageExhausted) ||
		errors.Is(err, model.ErrExtraAIUsageUnavailable) ||
		errors.Is(err, model.ErrExtraAIUsageDisabled) ||
		errors.Is(err, model.ErrBillingWorkspaceLocked)
}
