package service

import (
	"context"
	"errors"
	"github.com/helpin-ai/helpin/server/internal/aipolicy"
	"github.com/helpin-ai/helpin/server/internal/llm"
	"testing"
	"time"
)

type dockMediaCompletionFunc func(context.Context, llm.ChatRequest) (*llm.ChatResponse, error)

func (f dockMediaCompletionFunc) ChatCompletion(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	return f(ctx, req)
}

func TestDockChatMediaCompletionAllowsLongAnalysis(t *testing.T) {
	provider := dockMediaCompletionFunc(func(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) < time.Minute+50*time.Second || time.Until(deadline) > 2*time.Minute {
			t.Fatalf("media provider deadline = %v, want approximately two minutes per attempt", deadline)
		}
		action, ok := aipolicy.DefaultRegistry().Lookup(aipolicy.ActionAskMediaEnrichment)
		if !ok || action.Timeout != 4*time.Minute {
			t.Fatalf("media policy timeout = %v, want four minutes", action.Timeout)
		}
		return &llm.ChatResponse{Content: "The screenshot shows an upload error."}, nil
	})
	s := (&DockChatService{}).SetMediaAnalyzer(&PMAttachmentService{}, provider)
	result, err := s.analyzeDockChatMedia(context.Background(), "ws", "user", "What went wrong?", []dockChatMediaAttachment{
		{ID: "image", FileType: "image/png", URL: "https://example.com/screenshot.png"},
	})
	if err != nil || result != "The screenshot shows an upload error." {
		t.Fatalf("media analysis = %q, %v", result, err)
	}
}

func TestDockChatMediaCompletionPreservesCallerCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	provider := dockMediaCompletionFunc(func(callCtx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
		cancel()
		return nil, callCtx.Err()
	})
	s := (&DockChatService{}).SetMediaAnalyzer(&PMAttachmentService{}, provider)
	_, err := s.analyzeDockChatMedia(ctx, "ws", "user", "Describe this", []dockChatMediaAttachment{
		{ID: "image", FileType: "image/png", URL: "https://example.com/screenshot.png"},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("media analysis error = %v, want caller cancellation", err)
	}
}

func TestDockChatMediaFallbackOnlyForTransientFailures(t *testing.T) {
	for _, tc := range []struct {
		name      string
		status    int
		wantCalls int
		wantError bool
	}{
		{"rate_limit", 429, 2, false}, {"unavailable", 503, 2, false},
		{"invalid_media", 400, 1, true}, {"unauthorized", 401, 1, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			provider := dockMediaCompletionFunc(func(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
				calls++
				if calls == 1 {
					return nil, &llm.ProviderError{StatusCode: tc.status}
				}
				if req.Model != "qwen/qwen3.8-omni-flash" {
					t.Fatalf("fallback model=%s", req.Model)
				}
				return &llm.ChatResponse{Content: "Visible upload error"}, nil
			})
			s := (&DockChatService{}).SetMediaAnalyzer(&PMAttachmentService{}, provider)
			_, err := s.analyzeDockChatMedia(context.Background(), "ws", "user", "Describe", []dockChatMediaAttachment{{ID: "image", FileType: "image/png", URL: "https://example.com/image.png"}})
			if calls != tc.wantCalls || (err != nil) != tc.wantError {
				t.Fatalf("calls=%d error=%v", calls, err)
			}
		})
	}
}
