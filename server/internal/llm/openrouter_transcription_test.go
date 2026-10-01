package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOpenRouterTranscriptionMultipartAndDuration(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/audio/transcriptions" || r.Header.Get("Authorization") != "Bearer test-key" {
			t.Error("incorrect route or authorization")
		}
		if err := r.ParseMultipartForm(1024); err != nil {
			t.Fatal(err)
		}
		if r.FormValue("model") != "openai/gpt-transcribe" {
			t.Error("incorrect model")
		}
		f, h, err := r.FormFile("file")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if h.Filename != "recording.wav" {
			t.Error("incorrect filename")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"text":"Hello world","usage":{"seconds":1.25}}`))
	}))
	defer server.Close()
	result, err := NewOpenRouterTranscriptionClient("test-key", server.URL, server.Client()).Transcribe(context.Background(), []byte("audio"))
	if err != nil {
		t.Fatal(err)
	}
	if result.Text != "Hello world" || result.AudioMilliseconds != 1250 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestOpenRouterTranscriptionRejectsInvalidUsageAndSanitizesErrors(t *testing.T) {
	for _, body := range []string{`{"usage":{"seconds":1}}`, `{"text":"private","usage":{"seconds":-1}}`, `{"text":"private","usage":{"seconds":1e99}}`, `not-json`} {
		t.Run(body, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(body)) }))
			defer server.Close()
			_, err := NewOpenRouterTranscriptionClient("key", server.URL, server.Client()).Transcribe(context.Background(), nil)
			if err == nil {
				t.Fatal("expected invalid response error")
			}
		})
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(429)
		w.Write([]byte("secret upstream details"))
	}))
	defer server.Close()
	_, err := NewOpenRouterTranscriptionClient("key", server.URL, server.Client()).Transcribe(context.Background(), nil)
	if err == nil || err.Error() != "transcription provider returned status 429" {
		t.Fatalf("unexpected error %v", err)
	}
}

func TestOpenRouterTranscriptionDefaultEndpointAndOptionalDuration(t *testing.T) {
	client := NewOpenRouterTranscriptionClient("key", "", nil)
	if client.baseURL != OpenRouterDefaultBaseURL {
		t.Fatalf("wrong endpoint: %s", client.baseURL)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{"text":"Hello"}`)) }))
	defer server.Close()
	result, err := NewOpenRouterTranscriptionClient("key", server.URL, server.Client()).Transcribe(context.Background(), nil)
	if err != nil || result.Text != "Hello" || result.AudioMilliseconds != 0 {
		t.Fatalf("optional usage: %+v %v", result, err)
	}
}
