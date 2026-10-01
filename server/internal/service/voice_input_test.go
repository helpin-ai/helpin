package service

import (
	"context"
	"encoding/binary"
	"errors"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/aipolicy"
	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func voiceTestWAV(ms int) []byte {
	b := make([]byte, 44+ms*32)
	copy(b, "RIFF")
	binary.LittleEndian.PutUint32(b[4:], uint32(len(b)-8))
	copy(b[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(b[16:], 16)
	binary.LittleEndian.PutUint16(b[20:], 1)
	binary.LittleEndian.PutUint16(b[22:], 1)
	binary.LittleEndian.PutUint32(b[24:], 16000)
	binary.LittleEndian.PutUint32(b[28:], 32000)
	binary.LittleEndian.PutUint16(b[32:], 2)
	binary.LittleEndian.PutUint16(b[34:], 16)
	copy(b[36:], "data")
	binary.LittleEndian.PutUint32(b[40:], uint32(len(b)-44))
	return b
}
func TestVoiceWAVBounds(t *testing.T) {
	for _, ms := range []int{250, 1000, 120000, 300000} {
		got, err := voiceWAVDuration(voiceTestWAV(ms))
		if err != nil || got != int64(ms) {
			t.Fatalf("duration %d: %d %v", ms, got, err)
		}
	}
	for _, audio := range [][]byte{nil, voiceTestWAV(0), voiceTestWAV(300001), voiceTestWAV(1000)[:50]} {
		if _, err := voiceWAVDuration(audio); !errors.Is(err, ErrVoiceInvalidAudio) {
			t.Fatalf("accepted invalid audio: %v", err)
		}
	}
	b := voiceTestWAV(1000)
	b[22] = 2
	if _, err := voiceWAVDuration(b); err == nil {
		t.Fatal("accepted invalid channel count")
	}
}

type voiceTestProvider struct {
	result *llm.TranscriptionResult
	calls  int
	err    error
}

func (p *voiceTestProvider) Transcribe(context.Context, []byte) (llm.TranscriptionResult, error) {
	p.calls++
	if p.result != nil {
		return *p.result, p.err
	}
	return llm.TranscriptionResult{Text: "Hello", AudioMilliseconds: 1000}, p.err
}

type voiceTestAudit struct {
	finished  string
	execution *model.AIActionExecution
}

func (a *voiceTestAudit) Start(_ context.Context, e *model.AIActionExecution) (*model.AIActionExecution, error) {
	a.execution = e
	e.ID = "execution"
	return e, nil
}
func (a *voiceTestAudit) Finish(_ context.Context, _ string, r aipolicy.ExecutionResult) error {
	a.finished = r.Status
	return nil
}
func TestVoiceTranscriptionRecordsDurationAndValidatesBeforeProvider(t *testing.T) {
	store := &recordingCommunityUsage{}
	provider := &voiceTestProvider{}
	audit := &voiceTestAudit{}
	svc := NewVoiceInputService(provider, NewCommunityAIUsage(store), aipolicy.DefaultRegistry(), audit)
	result, err := svc.Transcribe(context.Background(), "ws", "user", voiceTestWAV(1000))
	if err != nil || result.Text != "Hello" {
		t.Fatalf("result %+v %v", result, err)
	}
	if len(store.entries) != 1 || store.entries[0].AudioMilliseconds != 1000 || store.entries[0].InputTokens != 0 {
		t.Fatalf("usage: %+v", store.entries)
	}
	if audit.finished != model.AIActionExecutionSucceeded {
		t.Fatalf("audit: %s", audit.finished)
	}
	_, err = svc.Transcribe(context.Background(), "ws", "user", nil)
	if !errors.Is(err, ErrVoiceInvalidAudio) || provider.calls != 1 {
		t.Fatal("invalid input reached provider")
	}
	provider.err = errors.New("private upstream details")
	_, err = svc.Transcribe(context.Background(), "ws", "user", voiceTestWAV(1000))
	if !errors.Is(err, ErrVoiceProvider) || audit.finished != model.AIActionExecutionFailed {
		t.Fatalf("failure: %v %s", err, audit.finished)
	}
}

type voiceAdmissionFailure struct{ *CommunityAIUsage }

func (voiceAdmissionFailure) Preflight(context.Context, PreflightRequest) (*MeteringContext, error) {
	return nil, model.ErrAIAllowanceExhausted
}
func TestVoiceAdmissionFailureNeverCallsProvider(t *testing.T) {
	provider := &voiceTestProvider{}
	svc := NewVoiceInputService(provider, voiceAdmissionFailure{}, aipolicy.DefaultRegistry(), &voiceTestAudit{})
	_, err := svc.Transcribe(context.Background(), "ws", "user", voiceTestWAV(1000))
	if !errors.Is(err, model.ErrAIAllowanceExhausted) || provider.calls != 0 {
		t.Fatalf("admission: %v, calls %d", err, provider.calls)
	}
}

func TestVoiceTranscriptionMeasuresInputWhenOpenRouterOmitsDuration(t *testing.T) {
	store := &recordingCommunityUsage{}
	provider := &voiceTestProvider{result: &llm.TranscriptionResult{Text: "Hello"}}
	audit := &voiceTestAudit{}
	svc := NewVoiceInputService(provider, NewCommunityAIUsage(store), aipolicy.DefaultRegistry(), audit)
	result, err := svc.Transcribe(context.Background(), "ws", "user", voiceTestWAV(1250))
	if err != nil || result.AudioMilliseconds != 1250 {
		t.Fatalf("result: %+v %v", result, err)
	}
	if len(store.entries) != 1 || store.entries[0].AudioMilliseconds != 1250 || store.entries[0].Provider != "openrouter" || store.entries[0].Model != "openai/gpt-transcribe" {
		t.Fatalf("usage: %+v", store.entries)
	}
	if audit.execution.Provider != "openrouter" || audit.execution.Model != "openai/gpt-transcribe" {
		t.Fatalf("audit: %+v", audit.execution)
	}
}
