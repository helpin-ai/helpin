package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/meetingcapture"
	"github.com/helpin-ai/helpin/server/internal/model"
)

type fakeMeetingArtifactStore struct {
	key         string
	contentType string
	body        []byte
}

func (s *fakeMeetingArtifactStore) PutObject(_ context.Context, key, contentType string, _ int64, body io.Reader, _ bool) error {
	payload, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	s.key = key
	s.contentType = contentType
	s.body = payload
	return nil
}

func TestMeetingProcessingCopiesCanonicalVideoRecording(t *testing.T) {
	db := setupMeetingLifecycleDB(t)
	repo, meeting := createMeetingLifecycleFixture(t, db, model.CRMMeetingStatusProcessing)
	meeting.RecordAudio = true
	if err := repo.Update(context.Background(), meeting); err != nil {
		t.Fatal(err)
	}
	capture := &model.CRMMeetingCapture{
		ID: "capture-video", WorkspaceID: meeting.WorkspaceID, MeetingID: meeting.ID, Provider: model.CRMMeetingProviderRecall,
		ProviderCaptureID: "recall-video", ProviderStatus: "done", Status: model.CRMMeetingStatusProcessing,
		RequestIdempotencyKey: "video-key", Metadata: model.JSONB{},
	}
	if err := repo.CreateCapture(context.Background(), capture); err != nil {
		t.Fatal(err)
	}
	mediaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "video/mp4")
		_, _ = w.Write([]byte("mp4-recording"))
	}))
	defer mediaServer.Close()
	provider := &fakeMeetingProvider{name: model.CRMMeetingProviderRecall, recording: &meetingcapture.Recording{
		ProviderRecordingID: "provider-recording-1", DownloadURL: mediaServer.URL, ContentType: "video/mp4",
	}}
	store := &fakeMeetingArtifactStore{}
	processor := NewCRMMeetingProcessingService(repo, nil, nil, store, mediaServer.Client(), provider)
	if err := processor.copyRecording(context.Background(), meeting, capture, provider); err != nil {
		t.Fatalf("copy recording: %v", err)
	}
	if store.key != "crm-meetings/workspace-1/meeting-1/recording.mp4" || store.contentType != "video/mp4" || string(store.body) != "mp4-recording" {
		t.Fatalf("stored recording key=%q content_type=%q body=%q", store.key, store.contentType, store.body)
	}
	reloaded, err := repo.GetByID(context.Background(), meeting.WorkspaceID, meeting.ID)
	if err != nil || reloaded.RecordingContentType == nil || *reloaded.RecordingContentType != "video/mp4" {
		t.Fatalf("reloaded meeting = %#v, err=%v", reloaded, err)
	}
}
