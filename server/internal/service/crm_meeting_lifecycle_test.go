package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/meetingcapture"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

type fakeMeetingProvider struct {
	name            string
	captureID       string
	event           *meetingcapture.ProviderEvent
	transcript      *meetingcapture.Transcript
	startCalls      int
	stopCalls       int
	deleteCalls     int
	transcriptCalls int
}

func (f *fakeMeetingProvider) Name() string                            { return f.name }
func (f *fakeMeetingProvider) Configured() bool                        { return true }
func (f *fakeMeetingProvider) Supports(string) bool                    { return true }
func (f *fakeMeetingProvider) VerifyWebhook(http.Header, []byte) error { return nil }
func (f *fakeMeetingProvider) GetRecording(context.Context, string) (*meetingcapture.Recording, error) {
	return nil, errors.New("recording unavailable")
}
func (f *fakeMeetingProvider) GetStatus(context.Context, string) (*meetingcapture.Capture, error) {
	return nil, nil
}
func (f *fakeMeetingProvider) StartCapture(_ context.Context, _ meetingcapture.StartCaptureInput) (*meetingcapture.Capture, error) {
	f.startCalls++
	return &meetingcapture.Capture{ProviderCaptureID: f.captureID, ProviderStatus: "joining", Status: model.CRMMeetingStatusJoining}, nil
}
func (f *fakeMeetingProvider) StopCapture(context.Context, string) error {
	f.stopCalls++
	return nil
}
func (f *fakeMeetingProvider) GetTranscript(context.Context, string) (*meetingcapture.Transcript, error) {
	f.transcriptCalls++
	if f.transcript == nil {
		return nil, errors.New("transcript unavailable")
	}
	return f.transcript, nil
}
func (f *fakeMeetingProvider) DeleteArtifacts(context.Context, string) error {
	f.deleteCalls++
	return nil
}
func (f *fakeMeetingProvider) NormalizeWebhook(http.Header, []byte) (*meetingcapture.ProviderEvent, error) {
	if f.event == nil {
		return nil, errors.New("event unavailable")
	}
	copy := *f.event
	return &copy, nil
}

type flakyMeetingProcessingRunner struct {
	calls     int
	failFirst bool
}

func (f *flakyMeetingProcessingRunner) StartMeetingProcessing(context.Context, string, string) error {
	f.calls++
	if f.failFirst && f.calls == 1 {
		return errors.New("temporal unavailable")
	}
	return nil
}

type fakeMeetingLLM struct{}

func (fakeMeetingLLM) ChatCompletion(context.Context, llm.ChatRequest) (*llm.ChatResponse, error) {
	return &llm.ChatResponse{Content: `{
		"summary_markdown":"## Summary\nThe team agreed to ship.",
		"key_points":["Release is ready"],
		"decisions":["Ship Friday"],
		"objections":[],
		"risks":["Migration timing"],
		"next_steps":["Azhar prepares release"],
		"action_items":[{"title":"Prepare release","details":"Publish the build","assignee_name":"Azhar","due_date":"2026-08-21","evidence":"I will prepare the release"}],
		"follow_up_draft":{"subject":"Release plan","body":"We will ship Friday."}
	}`}, nil
}

func setupMeetingLifecycleDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:crm-meeting-%s?mode=memory&cache=shared&_busy_timeout=5000", uuid.NewString())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open meeting test database: %v", err)
	}
	if err := db.Callback().Create().Before("gorm:create").Register("test:meeting_uuid", assignMeetingTestUUIDs); err != nil {
		t.Fatalf("register meeting UUID callback: %v", err)
	}
	statements := []string{
		`CREATE TABLE crm_meetings (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			calendar_event_id TEXT,
			activity_id TEXT UNIQUE,
			owner_member_id TEXT,
			title TEXT NOT NULL,
			meeting_url TEXT NOT NULL,
			platform TEXT NOT NULL,
			native_meeting_id TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'scheduled',
			summary_status TEXT NOT NULL DEFAULT 'pending',
			visibility TEXT NOT NULL DEFAULT 'workspace',
			record_audio BOOLEAN NOT NULL DEFAULT 0,
			scheduled_start_at DATETIME,
			scheduled_end_at DATETIME,
			actual_start_at DATETIME,
			actual_end_at DATETIME,
			duration_seconds INTEGER NOT NULL DEFAULT 0,
			participants BLOB NOT NULL DEFAULT x'5b5d',
			failure_code TEXT,
			failure_message TEXT,
			recording_object_key TEXT,
			created_by TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE crm_meeting_captures (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			meeting_id TEXT NOT NULL,
			provider TEXT NOT NULL,
			provider_capture_id TEXT NOT NULL,
			provider_status TEXT NOT NULL,
			status TEXT NOT NULL,
			request_idempotency_key TEXT NOT NULL,
			provider_recording_id TEXT,
			provider_transcript_id TEXT,
			started_at DATETIME,
			ended_at DATETIME,
			failure_code TEXT,
			failure_message TEXT,
			metadata BLOB NOT NULL DEFAULT x'7b7d',
			created_at DATETIME,
			updated_at DATETIME,
			UNIQUE(workspace_id, request_idempotency_key),
			UNIQUE(provider, provider_capture_id)
		)`,
		`CREATE TABLE crm_meeting_transcripts (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			meeting_id TEXT NOT NULL UNIQUE,
			capture_id TEXT NOT NULL,
			source_provider TEXT NOT NULL,
			language TEXT,
			plain_text TEXT NOT NULL,
			segments BLOB NOT NULL DEFAULT x'5b5d',
			checksum TEXT NOT NULL,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE crm_meeting_intelligence (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			meeting_id TEXT NOT NULL UNIQUE,
			generation_version TEXT NOT NULL,
			transcript_checksum TEXT NOT NULL,
			summary_markdown TEXT NOT NULL DEFAULT '',
			key_points BLOB NOT NULL DEFAULT x'5b5d',
			decisions BLOB NOT NULL DEFAULT x'5b5d',
			objections BLOB NOT NULL DEFAULT x'5b5d',
			risks BLOB NOT NULL DEFAULT x'5b5d',
			next_steps BLOB NOT NULL DEFAULT x'5b5d',
			follow_up_draft BLOB NOT NULL DEFAULT x'7b7d',
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE crm_meeting_action_items (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			meeting_id TEXT NOT NULL,
			position INTEGER NOT NULL,
			title TEXT NOT NULL,
			details TEXT,
			assignee_name TEXT,
			assignee_member_id TEXT,
			due_date DATE,
			evidence BLOB NOT NULL DEFAULT x'7b7d',
			status TEXT NOT NULL DEFAULT 'pending',
			task_id TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE crm_meeting_settings (
			workspace_id TEXT PRIMARY KEY,
			enabled BOOLEAN NOT NULL DEFAULT 0,
			default_provider TEXT NOT NULL DEFAULT 'recall',
			bot_name TEXT NOT NULL DEFAULT 'Helpin Notetaker',
			auto_join_mode TEXT NOT NULL DEFAULT 'manual',
			record_audio_by_default BOOLEAN NOT NULL DEFAULT 0,
			default_visibility TEXT NOT NULL DEFAULT 'workspace',
			include_internal BOOLEAN NOT NULL DEFAULT 0,
			include_private BOOLEAN NOT NULL DEFAULT 0,
			include_solo BOOLEAN NOT NULL DEFAULT 0,
			transcript_retention_days INTEGER NOT NULL DEFAULT 365,
			audio_retention_days INTEGER NOT NULL DEFAULT 30,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE crm_meeting_provider_events (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			provider TEXT NOT NULL,
			event_id TEXT NOT NULL,
			event_type TEXT NOT NULL,
			meeting_id TEXT,
			capture_id TEXT,
			payload BLOB NOT NULL,
			processed_at DATETIME,
			created_at DATETIME,
			UNIQUE(provider, event_id)
		)`,
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("create meeting test table: %v", err)
		}
	}
	return db
}

func assignMeetingTestUUIDs(tx *gorm.DB) {
	if tx.Statement == nil || tx.Statement.Schema == nil {
		return
	}
	idField := tx.Statement.Schema.LookUpField("ID")
	if idField == nil {
		return
	}
	value := tx.Statement.ReflectValue
	for value.IsValid() && (value.Kind() == reflect.Ptr || value.Kind() == reflect.Interface) {
		value = value.Elem()
	}
	setID := func(record reflect.Value) {
		for record.IsValid() && (record.Kind() == reflect.Ptr || record.Kind() == reflect.Interface) {
			record = record.Elem()
		}
		if !record.IsValid() || record.Kind() != reflect.Struct {
			return
		}
		_, zero := idField.ValueOf(tx.Statement.Context, record)
		if zero {
			_ = idField.Set(tx.Statement.Context, record, uuid.NewString())
		}
	}
	if value.Kind() == reflect.Slice || value.Kind() == reflect.Array {
		for index := 0; index < value.Len(); index++ {
			setID(value.Index(index))
		}
		return
	}
	setID(value)
}

func createMeetingLifecycleFixture(t *testing.T, db *gorm.DB, status string) (*repository.CRMMeetingRepository, *model.CRMMeeting) {
	t.Helper()
	repo := repository.NewCRMMeetingRepository(db)
	meeting := &model.CRMMeeting{
		ID: "meeting-1", WorkspaceID: "workspace-1", Title: "Product call",
		MeetingURL: "https://meet.google.com/abc-defg-hij", Platform: model.CRMMeetingPlatformGoogleMeet,
		NativeMeetingID: "abc-defg-hij", Status: status, SummaryStatus: model.CRMMeetingSummaryPending,
		Visibility: model.CRMMeetingVisibilityWorkspace, Participants: model.JSONBlob("[]"),
	}
	if err := repo.Create(context.Background(), meeting); err != nil {
		t.Fatalf("create meeting: %v", err)
	}
	return repo, meeting
}

func TestMeetingWebhookReplayRetriesUnprocessedEvent(t *testing.T) {
	db := setupMeetingLifecycleDB(t)
	repo, meeting := createMeetingLifecycleFixture(t, db, model.CRMMeetingStatusRecording)
	capture := &model.CRMMeetingCapture{
		ID: "capture-1", WorkspaceID: meeting.WorkspaceID, MeetingID: meeting.ID, Provider: model.CRMMeetingProviderRecall,
		ProviderCaptureID: "recall-bot-1", ProviderStatus: "in_call_recording", Status: model.CRMMeetingStatusRecording,
		RequestIdempotencyKey: "capture-key-1", Metadata: model.JSONB{},
	}
	if err := repo.CreateCapture(context.Background(), capture); err != nil {
		t.Fatalf("create capture: %v", err)
	}
	provider := &fakeMeetingProvider{name: model.CRMMeetingProviderRecall, event: &meetingcapture.ProviderEvent{
		EventID: "event-1", EventType: "transcript.done", ProviderCaptureID: capture.ProviderCaptureID,
		ProviderStatus: "done", Status: model.CRMMeetingStatusProcessing, TranscriptReady: true,
	}}
	runner := &flakyMeetingProcessingRunner{failFirst: true}
	service := NewCRMMeetingService(repo, nil, nil, provider).SetProcessingRunner(runner)

	if err := service.HandleWebhook(context.Background(), provider.Name(), nil, []byte(`{"event":"transcript.done"}`)); err == nil {
		t.Fatal("expected first workflow launch to fail")
	}
	event, err := repo.GetProviderEvent(context.Background(), provider.Name(), "event-1")
	if err != nil || event == nil || event.ProcessedAt != nil {
		t.Fatalf("unprocessed event = %#v, err=%v", event, err)
	}
	if err := service.HandleWebhook(context.Background(), provider.Name(), nil, []byte(`{"event":"transcript.done"}`)); err != nil {
		t.Fatalf("replay webhook: %v", err)
	}
	event, err = repo.GetProviderEvent(context.Background(), provider.Name(), "event-1")
	if err != nil || event == nil || event.ProcessedAt == nil || runner.calls != 2 {
		t.Fatalf("processed replay = %#v, calls=%d, err=%v", event, runner.calls, err)
	}
}

func TestMeetingWebhookFromOlderCaptureCannotOverwriteCurrentAttempt(t *testing.T) {
	db := setupMeetingLifecycleDB(t)
	repo, meeting := createMeetingLifecycleFixture(t, db, model.CRMMeetingStatusRecording)
	oldTime := time.Now().Add(-time.Hour)
	newTime := time.Now()
	oldCapture := &model.CRMMeetingCapture{
		ID: "capture-old", WorkspaceID: meeting.WorkspaceID, MeetingID: meeting.ID, Provider: model.CRMMeetingProviderRecall,
		ProviderCaptureID: "recall-old", ProviderStatus: "recording", Status: model.CRMMeetingStatusRecording,
		RequestIdempotencyKey: "old-key", Metadata: model.JSONB{}, CreatedAt: oldTime,
	}
	newCapture := &model.CRMMeetingCapture{
		ID: "capture-new", WorkspaceID: meeting.WorkspaceID, MeetingID: meeting.ID, Provider: model.CRMMeetingProviderRecall,
		ProviderCaptureID: "recall-new", ProviderStatus: "recording", Status: model.CRMMeetingStatusRecording,
		RequestIdempotencyKey: "new-key", Metadata: model.JSONB{}, CreatedAt: newTime,
	}
	if err := repo.CreateCapture(context.Background(), oldCapture); err != nil {
		t.Fatal(err)
	}
	if err := repo.CreateCapture(context.Background(), newCapture); err != nil {
		t.Fatal(err)
	}
	provider := &fakeMeetingProvider{name: model.CRMMeetingProviderRecall, event: &meetingcapture.ProviderEvent{
		EventID: "old-failure", EventType: "bot.call_ended", ProviderCaptureID: oldCapture.ProviderCaptureID,
		ProviderStatus: "fatal", Status: model.CRMMeetingStatusFailed, FailureMessage: "old bot failed",
	}}
	service := NewCRMMeetingService(repo, nil, nil, provider)
	if err := service.HandleWebhook(context.Background(), provider.Name(), nil, []byte(`{"event":"bot.call_ended"}`)); err != nil {
		t.Fatalf("handle stale webhook: %v", err)
	}
	reloaded, err := repo.GetByID(context.Background(), meeting.WorkspaceID, meeting.ID)
	if err != nil || reloaded.Status != model.CRMMeetingStatusRecording || reloaded.FailureMessage != nil {
		t.Fatalf("current meeting was overwritten: %#v, err=%v", reloaded, err)
	}
}

func TestMeetingProviderSwitchAffectsOnlyNewCaptureAttempts(t *testing.T) {
	db := setupMeetingLifecycleDB(t)
	repo, meeting := createMeetingLifecycleFixture(t, db, model.CRMMeetingStatusScheduled)
	recall := &fakeMeetingProvider{name: model.CRMMeetingProviderRecall, captureID: "recall-1"}
	vexa := &fakeMeetingProvider{name: model.CRMMeetingProviderVexa, captureID: "vexa-1"}
	settings := &model.CRMMeetingSettings{
		WorkspaceID: meeting.WorkspaceID, Enabled: true, DefaultProvider: model.CRMMeetingProviderRecall,
		BotName: "Helpin Notetaker", AutoJoinMode: "manual", DefaultVisibility: model.CRMMeetingVisibilityWorkspace,
		TranscriptRetentionDays: 365, AudioRetentionDays: 30,
	}
	if err := repo.UpsertSettings(context.Background(), settings); err != nil {
		t.Fatal(err)
	}
	service := NewCRMMeetingService(repo, nil, nil, recall, vexa)
	first, err := service.StartCapture(context.Background(), meeting.WorkspaceID, meeting.ID, "attempt-1")
	if err != nil || first.Provider != model.CRMMeetingProviderRecall {
		t.Fatalf("first capture = %#v, err=%v", first, err)
	}
	meeting, _ = repo.GetByID(context.Background(), meeting.WorkspaceID, meeting.ID)
	meeting.Status = model.CRMMeetingStatusFailed
	if err := repo.Update(context.Background(), meeting); err != nil {
		t.Fatal(err)
	}
	service.SetCaptureProvider(model.CRMMeetingProviderVexa)
	second, err := service.StartCapture(context.Background(), meeting.WorkspaceID, meeting.ID, "attempt-2")
	if err != nil || second.Provider != model.CRMMeetingProviderVexa {
		t.Fatalf("second capture = %#v, err=%v", second, err)
	}
	if _, err := service.StopCapture(context.Background(), meeting.WorkspaceID, meeting.ID); err != nil {
		t.Fatalf("stop latest capture: %v", err)
	}
	if recall.startCalls != 1 || recall.stopCalls != 0 || vexa.startCalls != 1 || vexa.stopCalls != 1 {
		t.Fatalf("provider calls recall(start=%d stop=%d), vexa(start=%d stop=%d)", recall.startCalls, recall.stopCalls, vexa.startCalls, vexa.stopCalls)
	}
}

func TestMeetingProcessingPersistsCanonicalArtifactsAndDeletesProviderCopy(t *testing.T) {
	db := setupMeetingLifecycleDB(t)
	repo, meeting := createMeetingLifecycleFixture(t, db, model.CRMMeetingStatusProcessing)
	provider := &fakeMeetingProvider{name: model.CRMMeetingProviderRecall, transcript: &meetingcapture.Transcript{
		ProviderTranscriptID: "transcript-1", Language: "en",
		Segments: []meetingcapture.TranscriptSegment{
			{ID: "one", SpeakerID: "p1", SpeakerName: "Azhar", Text: "I will prepare the release", StartSeconds: 1, EndSeconds: 4, Language: "en"},
			{ID: "two", SpeakerID: "p2", SpeakerName: "Maya", Text: "Let us ship Friday", StartSeconds: 5, EndSeconds: 8, Language: "en"},
		},
	}}
	capture := &model.CRMMeetingCapture{
		ID: "capture-process", WorkspaceID: meeting.WorkspaceID, MeetingID: meeting.ID, Provider: provider.Name(),
		ProviderCaptureID: "recall-process", ProviderStatus: "done", Status: model.CRMMeetingStatusProcessing,
		RequestIdempotencyKey: "process-key", Metadata: model.JSONB{},
	}
	if err := repo.CreateCapture(context.Background(), capture); err != nil {
		t.Fatal(err)
	}
	processor := NewCRMMeetingProcessingService(repo, nil, fakeMeetingLLM{}, nil, nil, provider)
	if err := processor.Process(context.Background(), meeting.WorkspaceID, meeting.ID); err != nil {
		t.Fatalf("process meeting: %v", err)
	}
	detailMeeting, err := repo.GetByID(context.Background(), meeting.WorkspaceID, meeting.ID)
	if err != nil || detailMeeting.Status != model.CRMMeetingStatusReady || detailMeeting.SummaryStatus != model.CRMMeetingSummaryReady {
		t.Fatalf("processed meeting = %#v, err=%v", detailMeeting, err)
	}
	var participants []map[string]interface{}
	if err := json.Unmarshal(detailMeeting.Participants, &participants); err != nil || len(participants) != 2 {
		t.Fatalf("participants = %s, err=%v", detailMeeting.Participants, err)
	}
	transcript, err := repo.GetTranscript(context.Background(), meeting.WorkspaceID, meeting.ID)
	if err != nil || transcript == nil || transcript.SourceProvider != provider.Name() || transcript.PlainText == "" {
		t.Fatalf("transcript = %#v, err=%v", transcript, err)
	}
	intelligence, err := repo.GetIntelligence(context.Background(), meeting.WorkspaceID, meeting.ID)
	if err != nil || intelligence == nil || intelligence.SummaryMarkdown == "" {
		t.Fatalf("intelligence = %#v, err=%v", intelligence, err)
	}
	actions, err := repo.ListActionItems(context.Background(), meeting.WorkspaceID, meeting.ID)
	if err != nil || len(actions) != 1 || actions[0].Title != "Prepare release" {
		t.Fatalf("actions = %#v, err=%v", actions, err)
	}
	if provider.deleteCalls != 1 {
		t.Fatalf("provider artifact deletes = %d", provider.deleteCalls)
	}
	provider.transcript = nil
	if err := processor.Process(context.Background(), meeting.WorkspaceID, meeting.ID); err != nil {
		t.Fatalf("retry from canonical transcript: %v", err)
	}
	if provider.transcriptCalls != 1 {
		t.Fatalf("provider transcript downloads = %d, want 1", provider.transcriptCalls)
	}
}
