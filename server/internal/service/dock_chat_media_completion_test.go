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
		if !ok || time.Until(deadline) < 3*time.Minute+50*time.Second || time.Until(deadline) > 4*time.Minute {
			t.Fatalf("media provider deadline = %v, want approximately four minutes", deadline)
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
