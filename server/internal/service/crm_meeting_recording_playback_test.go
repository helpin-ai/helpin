package service

import (
	"context"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

type fakeMeetingPlaybackStore struct {
	url string
}

func (s fakeMeetingPlaybackStore) GeneratePresignedInlineGetURL(string) (string, error) {
	return s.url, nil
}
func (fakeMeetingPlaybackStore) DeleteObject(context.Context, string) error { return nil }

func TestMeetingRecordingPlaybackContractSupportsVideoAndLegacyAudio(t *testing.T) {
	tests := []struct {
		name        string
		key         string
		contentType *string
		wantType    string
		wantMedia   string
	}{
		{name: "video", key: "recording.mp4", contentType: meetingStringPointer("video/mp4"), wantType: "video/mp4", wantMedia: "video"},
		{name: "legacy audio", key: "recording.mp3", wantType: "audio/mpeg", wantMedia: "audio"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db := setupMeetingLifecycleDB(t)
			repo, meeting := createMeetingLifecycleFixture(t, db, model.CRMMeetingStatusReady)
			meeting.RecordingObjectKey = &test.key
			meeting.RecordingContentType = test.contentType
			if err := repo.Update(context.Background(), meeting); err != nil {
				t.Fatal(err)
			}
			meetingService := NewCRMMeetingService(repo, nil, nil).SetRecordingStore(fakeMeetingPlaybackStore{url: "https://signed.example/recording"})
			recording, err := meetingService.GetRecording(context.Background(), meeting.WorkspaceID, meeting.ID)
			if err != nil {
				t.Fatalf("get recording: %v", err)
			}
			if recording.URL == "" || recording.ContentType != test.wantType || recording.MediaType != test.wantMedia {
				t.Fatalf("recording = %#v", recording)
			}
		})
	}
}
