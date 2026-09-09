package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/meetingcapture"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

const (
	meetingIntelligenceGenerationVersion = "v5"
	meetingIntelligenceMaxTokens         = 8192
)

type meetingArtifactStore interface {
	PutObject(ctx context.Context, key, contentType string, size int64, body io.Reader, publicRead bool) error
}

type meetingSignalDetector interface {
	DetectSignals(ctx context.Context, payloads []model.SignalSourcePayload) ([]model.CRMSignal, error)
}

type meetingActivityCreator interface {
	Create(ctx context.Context, req model.CreateCRMActivityRequest) (*model.CRMActivity, error)
}

type meetingSuggestionManager interface {
	List(ctx context.Context, workspaceID string, filters model.CRMSuggestionListFilters, pagination model.PMPagination) ([]model.CRMSuggestion, int64, error)
	Create(ctx context.Context, req model.CreateCRMSuggestionRequest) (*model.CRMSuggestion, error)
}

// CRMMeetingProcessingService owns provider-neutral transcript persistence and intelligence generation.
type CRMMeetingProcessingService struct {
	repo            *repository.CRMMeetingRepository
	associationRepo *repository.CRMAssociationRepository
	llmProvider     llm.Provider
	signalDetector  meetingSignalDetector
	activityCreator meetingActivityCreator
	suggestions     meetingSuggestionManager
	followUpRouting meetingFollowUpRoutingStore
	artifactStore   meetingArtifactStore
	httpClient      *http.Client
	providers       map[string]meetingCaptureProvider
}

// NewCRMMeetingProcessingService creates the fixed product-owned meeting processor.
func NewCRMMeetingProcessingService(
	repo *repository.CRMMeetingRepository,
	associationRepo *repository.CRMAssociationRepository,
	llmProvider llm.Provider,
	artifactStore meetingArtifactStore,
	httpClient *http.Client,
	providers ...meetingCaptureProvider,
) *CRMMeetingProcessingService {
	providerMap := make(map[string]meetingCaptureProvider, len(providers))
	for _, provider := range providers {
		if provider != nil {
			providerMap[provider.Name()] = provider
		}
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Minute}
	}
	return &CRMMeetingProcessingService{
		repo:            repo,
		associationRepo: associationRepo,
		llmProvider:     llmProvider,
		artifactStore:   artifactStore,
		httpClient:      httpClient,
		providers:       providerMap,
	}
}

// SetCRMOutputs injects existing CRM projections used after generation.
func (s *CRMMeetingProcessingService) SetCRMOutputs(
	signals meetingSignalDetector,
	activities meetingActivityCreator,
	suggestions meetingSuggestionManager,
) *CRMMeetingProcessingService {
	s.signalDetector = signals
	s.activityCreator = activities
	s.suggestions = suggestions
	return s
}

// Process copies the canonical transcript, generates intelligence, and projects CRM outputs.
func (s *CRMMeetingProcessingService) Process(ctx context.Context, workspaceID, meetingID string) error {
	meeting, capture, provider, err := s.loadCapture(ctx, workspaceID, meetingID)
	if err != nil {
		return err
	}
	transcript, err := s.repo.GetTranscript(ctx, workspaceID, meetingID)
	if err != nil {
		return err
	}
	if transcript == nil || transcript.CaptureID != capture.ID {
		providerTranscript, transcriptErr := provider.GetTranscript(ctx, capture.ProviderCaptureID)
		if transcriptErr != nil {
			return fmt.Errorf("get %s meeting transcript: %w", capture.Provider, transcriptErr)
		}
		transcript, transcriptErr = canonicalMeetingTranscript(workspaceID, meetingID, capture.ID, capture.Provider, providerTranscript)
		if transcriptErr != nil {
			return transcriptErr
		}
		if transcriptErr := s.repo.UpsertTranscript(ctx, transcript); transcriptErr != nil {
			return transcriptErr
		}
		capture.ProviderTranscriptID = trimStringPtr(&providerTranscript.ProviderTranscriptID)
		if transcriptErr := s.repo.UpdateCapture(ctx, capture); transcriptErr != nil {
			return transcriptErr
		}
	}
	meeting.Participants = meetingParticipants(transcript)
	meeting.Status = model.CRMMeetingStatusProcessing
	meeting.SummaryStatus = model.CRMMeetingSummaryProcessing
	meeting.FailureCode = nil
	meeting.FailureMessage = nil
	if err := s.repo.Update(ctx, meeting); err != nil {
		return err
	}
	if err := s.copyRecording(ctx, meeting, capture, provider); err != nil {
		return s.failProcessing(ctx, meeting, "recording_copy_failed", err)
	}
	output, err := s.generateIntelligence(ctx, meeting, transcript)
	if err != nil {
		if isMeetingUsageBlocked(err) {
			meeting.SummaryStatus = model.CRMMeetingSummaryBlockedUsage
			meeting.FailureCode = trimStringPtr(meetingStringPointer("ai_usage_blocked"))
			meeting.FailureMessage = trimStringPtr(meetingStringPointer(err.Error()))
			if updateErr := s.repo.Update(ctx, meeting); updateErr != nil {
				return updateErr
			}
			return err
		}
		return s.failProcessing(ctx, meeting, "intelligence_failed", err)
	}
	if err := s.persistIntelligence(ctx, meeting, transcript, output); err != nil {
		return s.failProcessing(ctx, meeting, "intelligence_persist_failed", err)
	}
	if err := s.projectActivity(ctx, meeting, output.SummaryMarkdown); err != nil {
		slog.WarnContext(ctx, "meeting CRM activity projection failed", "workspace_id", workspaceID, "meeting_id", meetingID, "error", err)
	}
	if err := s.projectSignals(ctx, meeting, transcript); err != nil {
		slog.WarnContext(ctx, "meeting signal detection failed", "workspace_id", workspaceID, "meeting_id", meetingID, "error", err)
	}
	if err := s.projectFollowUp(ctx, meeting, transcript, output); err != nil {
		slog.WarnContext(ctx, "meeting follow-up projection failed", "workspace_id", workspaceID, "meeting_id", meetingID, "error", err)
	}
	meeting.Status = model.CRMMeetingStatusReady
	meeting.SummaryStatus = model.CRMMeetingSummaryReady
	meeting.FailureCode = nil
	meeting.FailureMessage = nil
	if err := s.repo.Update(ctx, meeting); err != nil {
		return err
	}
	if provider != nil {
		if err := provider.DeleteArtifacts(ctx, capture.ProviderCaptureID); err != nil {
			slog.WarnContext(ctx, "provider artifact cleanup failed", "workspace_id", workspaceID, "meeting_id", meetingID, "provider", capture.Provider, "error", err)
		}
	}
	return nil
}

func (s *CRMMeetingProcessingService) loadCapture(
	ctx context.Context,
	workspaceID, meetingID string,
) (*model.CRMMeeting, *model.CRMMeetingCapture, meetingCaptureProvider, error) {
	meeting, err := s.repo.GetByID(ctx, workspaceID, meetingID)
	if err != nil {
		return nil, nil, nil, err
	}
	if meeting == nil {
		return nil, nil, nil, fmt.Errorf("meeting not found")
	}
	capture, err := s.repo.GetLatestCapture(ctx, workspaceID, meetingID)
	if err != nil {
		return nil, nil, nil, err
	}
	if capture == nil {
		return nil, nil, nil, fmt.Errorf("meeting capture not found")
	}
	provider := s.providers[capture.Provider]
	if provider == nil || !provider.Configured() {
		return nil, nil, nil, fmt.Errorf("%s meeting provider is not configured", capture.Provider)
	}
	return meeting, capture, provider, nil
}

func canonicalMeetingTranscript(
	workspaceID, meetingID, captureID, provider string,
	input *meetingcapture.Transcript,
) (*model.CRMMeetingTranscript, error) {
	if input == nil || len(input.Segments) == 0 {
		return nil, fmt.Errorf("meeting transcript is empty")
	}
	segments := make(model.CRMMeetingTranscriptSegments, 0, len(input.Segments))
	lines := make([]string, 0, len(input.Segments))
	for index, segment := range input.Segments {
		text := strings.TrimSpace(segment.Text)
		if text == "" {
			continue
		}
		speaker := strings.TrimSpace(segment.SpeakerName)
		if speaker == "" {
			speaker = "Unknown speaker"
		}
		segmentID := strings.TrimSpace(segment.ID)
		if segmentID == "" {
			segmentID = fmt.Sprintf("segment-%d", index)
		}
		segments = append(segments, model.CRMMeetingTranscriptSegment{
			ID:           segmentID,
			SpeakerID:    segment.SpeakerID,
			SpeakerName:  speaker,
			Text:         text,
			StartSeconds: segment.StartSeconds,
			EndSeconds:   segment.EndSeconds,
			Language:     segment.Language,
			Confidence:   segment.Confidence,
		})
		lines = append(lines, speaker+": "+text)
	}
	if len(segments) == 0 {
		return nil, fmt.Errorf("meeting transcript is empty")
	}
	plainText := strings.Join(lines, "\n")
	checksum := sha256.Sum256([]byte(plainText))
	return &model.CRMMeetingTranscript{
		WorkspaceID:    workspaceID,
		MeetingID:      meetingID,
		CaptureID:      captureID,
		SourceProvider: provider,
		Language:       trimStringPtr(&input.Language),
		PlainText:      plainText,
		Segments:       segments,
		Checksum:       hex.EncodeToString(checksum[:]),
	}, nil
}

type meetingIntelligenceOutput struct {
	SummaryMarkdown     string                    `json:"summary_markdown"`
	ParticipantsContext []string                  `json:"participants_context"`
	KeyPoints           []string                  `json:"key_points"`
	Decisions           []string                  `json:"decisions"`
	Objections          []string                  `json:"objections"`
	Risks               []string                  `json:"risks"`
	OpenQuestions       []string                  `json:"open_questions"`
	NextSteps           []string                  `json:"next_steps"`
	Rapport             []string                  `json:"rapport"`
	ActionItems         []meetingActionItemOutput `json:"action_items"`
	FollowUpDraft       meetingFollowUpOutput     `json:"follow_up_draft"`
}

type meetingActionItemOutput struct {
	Title        string `json:"title"`
	Details      string `json:"details"`
	AssigneeName string `json:"assignee_name"`
	DueDate      string `json:"due_date"`
	Evidence     string `json:"evidence"`
}

type meetingFollowUpOutput struct {
	Subject       string `json:"subject"`
	Body          string `json:"body"`
	Scope         string `json:"scope"`
	ScopeEvidence string `json:"scope_evidence"`
}

func (s *CRMMeetingProcessingService) generateIntelligence(
	ctx context.Context,
	meeting *model.CRMMeeting,
	transcript *model.CRMMeetingTranscript,
) (*meetingIntelligenceOutput, error) {
	if s.llmProvider == nil {
		return nil, fmt.Errorf("meeting intelligence LLM is not configured")
	}
	content := transcript.PlainText
	if len(content) > 120000 {
		content = content[:120000]
	}

	var output *meetingIntelligenceOutput
	_, err := completeAI(ctx, s.llmProvider, AICompletionRequest{
		WorkspaceID:    meeting.WorkspaceID,
		FeatureKey:     BillingFeatureMeetingIntelligence,
		IdempotencyKey: aiUsageIdempotencyKey(meeting.WorkspaceID, meeting.ID, transcript.Checksum, meetingIntelligenceGenerationVersion),
		Metadata: map[string]interface{}{
			"meeting_id": meeting.ID,
			"provider":   transcript.SourceProvider,
		},
		RequireComplete:    true,
		RetryInvalidOutput: true,
		ValidateResponse: func(response *llm.ChatResponse) error {
			parsed, parseErr := parseMeetingIntelligenceResponse(response)
			if parseErr == nil {
				output = parsed
			}
			return parseErr
		},
		Chat: llm.ChatRequest{
			SystemPrompt:     meetingIntelligenceSystemPrompt,
			Messages:         []llm.Message{{Role: "user", Content: "Meeting title: " + meeting.Title + "\n\nTranscript:\n" + content}},
			Temperature:      0.1,
			MaxTokens:        meetingIntelligenceMaxTokens,
			JSONMode:         true,
			JSONSchema:       meetingIntelligenceJSONSchema,
			JSONSchemaStrict: true,
			Reasoning:        &llm.ReasoningConfig{Effort: "low"},
			ProviderOptions:  json.RawMessage(`{"require_parameters":true}`),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("meeting intelligence generation failed after trying a backup model")
	}
	return output, nil
}

func parseMeetingIntelligenceResponse(response *llm.ChatResponse) (*meetingIntelligenceOutput, error) {
	if response == nil {
		return nil, fmt.Errorf("model returned no response")
	}
	if strings.TrimSpace(response.Content) == "" {
		finishReason := strings.TrimSpace(response.FinishReason)
		if finishReason == "" {
			finishReason = "unknown"
		}
		return nil, fmt.Errorf(
			"model returned no structured output (finish_reason=%s, reasoning_tokens=%d, completion_tokens=%d)",
			finishReason,
			response.TokensUsed.ReasoningTokens,
			response.TokensUsed.CompletionTokensTotal,
		)
	}
	var output meetingIntelligenceOutput
	if err := llm.UnmarshalResponse(response.Content, &output); err != nil {
		return nil, fmt.Errorf("model returned malformed structured output")
	}
	if strings.TrimSpace(output.SummaryMarkdown) == "" {
		return nil, fmt.Errorf("model response did not include a summary")
	}
	return &output, nil
}

func (s *CRMMeetingProcessingService) persistIntelligence(
	ctx context.Context,
	meeting *model.CRMMeeting,
	transcript *model.CRMMeetingTranscript,
	output *meetingIntelligenceOutput,
) error {
	intelligence := &model.CRMMeetingIntelligence{
		WorkspaceID:         meeting.WorkspaceID,
		MeetingID:           meeting.ID,
		GenerationVersion:   meetingIntelligenceGenerationVersion,
		TranscriptChecksum:  transcript.Checksum,
		SummaryMarkdown:     strings.TrimSpace(output.SummaryMarkdown),
		ParticipantsContext: jsonBlob(output.ParticipantsContext),
		KeyPoints:           jsonBlob(output.KeyPoints),
		Decisions:           jsonBlob(output.Decisions),
		Objections:          jsonBlob(output.Objections),
		Risks:               jsonBlob(output.Risks),
		OpenQuestions:       jsonBlob(output.OpenQuestions),
		NextSteps:           jsonBlob(output.NextSteps),
		Rapport:             jsonBlob(output.Rapport),
		FollowUpDraft:       model.JSONB{"subject": output.FollowUpDraft.Subject, "body": output.FollowUpDraft.Body},
	}
	if err := s.repo.UpsertIntelligence(ctx, intelligence); err != nil {
		return err
	}
	items := make([]model.CRMMeetingActionItem, 0, len(output.ActionItems))
	for index, outputItem := range output.ActionItems {
		title := strings.TrimSpace(outputItem.Title)
		if title == "" {
			continue
		}
		item := model.CRMMeetingActionItem{
			WorkspaceID:  meeting.WorkspaceID,
			MeetingID:    meeting.ID,
			Position:     index,
			Title:        title,
			Details:      trimStringPtr(&outputItem.Details),
			AssigneeName: trimStringPtr(&outputItem.AssigneeName),
			Evidence:     model.JSONB{"excerpt": strings.TrimSpace(outputItem.Evidence)},
			Status:       model.CRMMeetingActionPending,
		}
		if dueDate, err := time.Parse("2006-01-02", strings.TrimSpace(outputItem.DueDate)); err == nil {
			item.DueDate = &dueDate
		}
		items = append(items, item)
	}
	return s.repo.ReplaceActionItems(ctx, meeting.WorkspaceID, meeting.ID, items)
}

func (s *CRMMeetingProcessingService) copyRecording(
	ctx context.Context,
	meeting *model.CRMMeeting,
	capture *model.CRMMeetingCapture,
	provider meetingCaptureProvider,
) error {
	if !meeting.RecordAudio || meeting.RecordingObjectKey != nil {
		return nil
	}
	if s.artifactStore == nil {
		return fmt.Errorf("meeting recording storage is not configured")
	}
	if provider == nil {
		return fmt.Errorf("%s meeting provider is not configured", capture.Provider)
	}
	recording, err := provider.GetRecording(ctx, capture.ProviderCaptureID)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, recording.DownloadURL, nil)
	if err != nil {
		return fmt.Errorf("create recording download request: %w", err)
	}
	for name, value := range recording.Headers {
		request.Header.Set(name, value)
	}
	response, err := s.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("download meeting recording: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("download meeting recording: provider returned HTTP %d", response.StatusCode)
	}
	contentType, extension, err := canonicalMeetingRecordingFormat(recording.ContentType, response.Header.Get("Content-Type"))
	if err != nil {
		return err
	}
	key := "crm-meetings/" + meeting.WorkspaceID + "/" + meeting.ID + "/recording" + extension
	if err := s.artifactStore.PutObject(ctx, key, contentType, response.ContentLength, response.Body, false); err != nil {
		return err
	}
	meeting.RecordingObjectKey = &key
	meeting.RecordingContentType = &contentType
	capture.ProviderRecordingID = trimStringPtr(&recording.ProviderRecordingID)
	if err := s.repo.UpdateCapture(ctx, capture); err != nil {
		return err
	}
	return s.repo.Update(ctx, meeting)
}

func canonicalMeetingRecordingFormat(values ...string) (string, string, error) {
	for _, value := range values {
		contentType := strings.ToLower(strings.TrimSpace(strings.SplitN(value, ";", 2)[0]))
		switch contentType {
		case "video/mp4":
			return "video/mp4", ".mp4", nil
		case "video/webm":
			return "video/webm", ".webm", nil
		case "audio/mpeg", "audio/mp3":
			return "audio/mpeg", ".mp3", nil
		case "audio/webm":
			return "audio/webm", ".webm", nil
		case "audio/mp4", "audio/x-m4a":
			return "audio/mp4", ".m4a", nil
		}
	}
	return "", "", fmt.Errorf("meeting recording has an unsupported content type")
}

func (s *CRMMeetingProcessingService) projectActivity(
	ctx context.Context,
	meeting *model.CRMMeeting,
	summary string,
) error {
	if s.activityCreator == nil || meeting.ActivityID != nil {
		return nil
	}
	contactID, companyID, dealID, err := s.associatedCRMObjects(ctx, meeting)
	if err != nil {
		return err
	}
	occurredAt := meeting.ActualEndAt
	if occurredAt == nil {
		occurredAt = meeting.ScheduledStartAt
	}
	subject := meeting.Title
	body := strings.TrimSpace(summary)
	activity, err := s.activityCreator.Create(ctx, model.CreateCRMActivityRequest{
		WorkspaceID:   meeting.WorkspaceID,
		ActivityType:  model.CRMActivityMeeting,
		ContactID:     contactID,
		CompanyID:     companyID,
		DealID:        dealID,
		OwnerMemberID: meeting.OwnerMemberID,
		Subject:       &subject,
		Body:          &body,
		OccurredAt:    occurredAt,
		Metadata: map[string]interface{}{
			"meeting_id": meeting.ID,
			"platform":   meeting.Platform,
			"event_type": "meeting.captured",
			"immutable":  true,
		},
	})
	if err != nil {
		return err
	}
	meeting.ActivityID = &activity.ID
	return s.repo.Update(ctx, meeting)
}

func (s *CRMMeetingProcessingService) projectSignals(
	ctx context.Context,
	meeting *model.CRMMeeting,
	transcript *model.CRMMeetingTranscript,
) error {
	if s.signalDetector == nil {
		return nil
	}
	contactID, _, dealID, err := s.associatedCRMObjects(ctx, meeting)
	if err != nil {
		return err
	}
	body := transcript.PlainText
	if len(body) > 12000 {
		body = body[:12000]
	}
	occurredAt := meeting.CreatedAt
	if meeting.ActualEndAt != nil {
		occurredAt = *meeting.ActualEndAt
	} else if meeting.ScheduledStartAt != nil {
		occurredAt = *meeting.ScheduledStartAt
	}
	_, err = s.signalDetector.DetectSignals(ctx, []model.SignalSourcePayload{{
		SourceType:  model.CRMSignalSourceMeeting,
		SourceID:    meeting.ID,
		WorkspaceID: meeting.WorkspaceID,
		ContactID:   contactID,
		DealID:      dealID,
		Subject:     meeting.Title,
		Body:        body,
		Direction:   "bilateral",
		OccurredAt:  occurredAt,
	}})
	return err
}

func (s *CRMMeetingProcessingService) projectFollowUp(
	ctx context.Context,
	meeting *model.CRMMeeting,
	transcript *model.CRMMeetingTranscript,
	output *meetingIntelligenceOutput,
) error {
	if s.suggestions == nil || strings.TrimSpace(output.FollowUpDraft.Body) == "" {
		return nil
	}
	objectType := model.CRMObjectMeeting
	existing, _, err := s.suggestions.List(ctx, meeting.WorkspaceID, model.CRMSuggestionListFilters{
		IncludeInternalMeetingFollowUps: true,
		SuggestionType:                  meetingStringPointer(model.CRMSuggestionFollowUp),
		ObjectType:                      &objectType,
		ObjectID:                        &meeting.ID,
	}, model.PMPagination{Page: 1, PerPage: 1})
	if err != nil || len(existing) > 0 {
		return err
	}
	title := strings.TrimSpace(output.FollowUpDraft.Subject)
	if title == "" {
		title = "Follow up after " + meeting.Title
	}
	description := strings.TrimSpace(output.FollowUpDraft.Body)
	scope := validatedMeetingFollowUpScope(output.FollowUpDraft.Scope, output.FollowUpDraft.ScopeEvidence, transcript.PlainText, output.FollowUpDraft.Subject, output.FollowUpDraft.Body)
	routingContext := map[string]interface{}{
		"meeting_id":                  meeting.ID,
		"draft_subject":               output.FollowUpDraft.Subject,
		"draft_body":                  output.FollowUpDraft.Body,
		model.MeetingFollowUpScopeKey: scope,
	}
	// An unsupported quote is an incomplete classification, not a final abstention.
	// The automatic backfill retries it using the saved draft and source transcript.
	if output.FollowUpDraft.Scope == "uncertain" || scope == output.FollowUpDraft.Scope {
		routingContext[model.MeetingFollowUpRoutingVersionKey] = model.MeetingFollowUpRoutingVersion
	}
	confidence := 0.85
	_, err = s.suggestions.Create(ctx, model.CreateCRMSuggestionRequest{
		WorkspaceID:    meeting.WorkspaceID,
		SuggestionType: model.CRMSuggestionFollowUp,
		ObjectType:     &objectType,
		ObjectID:       &meeting.ID,
		Title:          title,
		Description:    &description,
		Context:        routingContext,
		Confidence:     &confidence,
	})
	return err
}

func (s *CRMMeetingProcessingService) associatedCRMObjects(
	ctx context.Context,
	meeting *model.CRMMeeting,
) (*string, *string, *string, error) {
	associations, err := s.associationRepo.ListByObject(ctx, meeting.WorkspaceID, model.CRMObjectMeeting, meeting.ID)
	if err != nil {
		return nil, nil, nil, err
	}
	var contactID, companyID, dealID *string
	for _, association := range associations {
		objectType, objectID := association.ToObjectType, association.ToObjectID
		if objectType == model.CRMObjectMeeting && objectID == meeting.ID {
			objectType, objectID = association.FromObjectType, association.FromObjectID
		}
		switch objectType {
		case model.CRMObjectContact:
			contactID = meetingStringPointer(objectID)
		case model.CRMObjectCompany:
			companyID = meetingStringPointer(objectID)
		case model.CRMObjectDeal:
			dealID = meetingStringPointer(objectID)
		}
	}
	return contactID, companyID, dealID, nil
}

func (s *CRMMeetingProcessingService) failProcessing(ctx context.Context, meeting *model.CRMMeeting, code string, cause error) error {
	meeting.Status = model.CRMMeetingStatusFailed
	meeting.SummaryStatus = model.CRMMeetingSummaryFailed
	meeting.FailureCode = meetingStringPointer(code)
	meeting.FailureMessage = meetingStringPointer(cause.Error())
	if err := s.repo.Update(ctx, meeting); err != nil {
		return err
	}
	return cause
}

func meetingParticipants(transcript *model.CRMMeetingTranscript) model.JSONBlob {
	participants := make([]map[string]string, 0)
	seen := make(map[string]struct{})
	if transcript != nil {
		for _, segment := range transcript.Segments {
			name := strings.TrimSpace(segment.SpeakerName)
			id := strings.TrimSpace(segment.SpeakerID)
			key := strings.ToLower(name)
			if id != "" {
				key = "id:" + id
			}
			if key == "" {
				continue
			}
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			participants = append(participants, map[string]string{"id": id, "name": name})
		}
	}
	return jsonBlob(participants)
}

func jsonBlob(value interface{}) model.JSONBlob {
	payload, err := json.Marshal(value)
	if err != nil {
		return model.JSONBlob("[]")
	}
	return model.JSONBlob(payload)
}

func meetingStringPointer(value string) *string {
	return &value
}

var meetingIntelligenceJSONSchema = map[string]any{
	"type":                 "object",
	"additionalProperties": false,
	"required": []string{
		"summary_markdown",
		"participants_context",
		"key_points",
		"decisions",
		"objections",
		"risks",
		"open_questions",
		"next_steps",
		"rapport",
		"action_items",
		"follow_up_draft",
	},
	"properties": map[string]any{
		"summary_markdown":     map[string]any{"type": "string"},
		"participants_context": meetingIntelligenceStringArraySchema(),
		"key_points":           meetingIntelligenceStringArraySchema(),
		"decisions":            meetingIntelligenceStringArraySchema(),
		"objections":           meetingIntelligenceStringArraySchema(),
		"risks":                meetingIntelligenceStringArraySchema(),
		"open_questions":       meetingIntelligenceStringArraySchema(),
		"next_steps":           meetingIntelligenceStringArraySchema(),
		"rapport":              meetingIntelligenceStringArraySchema(),
		"action_items": map[string]any{
			"type": "array",
			"items": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"required": []string{
					"title",
					"details",
					"assignee_name",
					"due_date",
					"evidence",
				},
				"properties": map[string]any{
					"title":         map[string]any{"type": "string"},
					"details":       map[string]any{"type": "string"},
					"assignee_name": map[string]any{"type": "string"},
					"due_date":      map[string]any{"type": "string"},
					"evidence":      map[string]any{"type": "string"},
				},
			},
		},
		"follow_up_draft": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"subject", "body", "scope", "scope_evidence"},
			"properties": map[string]any{
				"scope":          map[string]any{"type": "string", "enum": []string{"internal", "customer", "uncertain"}},
				"scope_evidence": map[string]any{"type": "string"},
				"subject":        map[string]any{"type": "string"},
				"body":           map[string]any{"type": "string"},
			},
		},
	},
}

func meetingIntelligenceStringArraySchema() map[string]any {
	return map[string]any{
		"type":  "array",
		"items": map[string]any{"type": "string"},
	}
}

const meetingIntelligenceSystemPrompt = `You are Helpin's meeting intelligence processor. Convert the transcript into a concise, reliable meeting record that reads in a clear linear order.

Return one JSON object with exactly these keys:
- summary_markdown: one concise factual overview paragraph in Markdown, without a heading
- participants_context: participant roles and meeting purpose/context as concise strings
- key_points: the most important discussion points, without repeating the overview
- decisions: confirmed decisions only
- open_questions: unresolved questions, issues, or blockers
- next_steps: concrete next steps, without duplicating action_items
- rapport: useful relationship, engagement, or sentiment observations supported by the transcript
- objections and risks: concise internal CRM signals retained for automation
- action_items: array of objects with title, details, assignee_name, due_date (YYYY-MM-DD or empty), and a short evidence excerpt
- follow_up_draft: object with subject, body, scope (internal/customer/uncertain), and scope_evidence (an exact excerpt from the draft or transcript supporting scope). Apply the following routing rules:
` + meetingFollowUpScopeRules + `

Prioritize signal over completeness. Never invent identities, roles, facts, owners, dates, commitments, sentiment, or rapport. Use empty arrays or empty strings when the transcript does not support a field. Do not include prose outside JSON.`
