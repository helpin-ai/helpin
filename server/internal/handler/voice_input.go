package handler

import (
	"context"
	"errors"
	"io"
	"net/http"

	"github.com/helpin-ai/helpin/server/internal/aiusage"
	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

type voiceInputService interface {
	Available() bool
	Transcribe(context.Context, string, string, []byte) (llm.TranscriptionResult, error)
}
type VoiceInputHandler struct{ service voiceInputService }

func NewVoiceInputHandler(s voiceInputService) *VoiceInputHandler {
	return &VoiceInputHandler{service: s}
}
func (h *VoiceInputHandler) Capabilities(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{"enabled": h.service.Available(), "max_seconds": aiusage.MaxVoiceMilliseconds / 1000})
}
func (h *VoiceInputHandler) Transcribe(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	r.Body = http.MaxBytesReader(w, r.Body, int64(service.MaxVoiceUploadBytes))
	defer r.Body.Close()
	audio, err := io.ReadAll(r.Body)
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeError(w, http.StatusRequestEntityTooLarge, "Recording is too large")
			return
		}
		writeError(w, http.StatusBadRequest, "Unable to read recording")
		return
	}
	result, err := h.service.Transcribe(r.Context(), getWorkspaceID(r), middleware.GetUserID(r.Context()), audio)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrVoiceInvalidAudio):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, service.ErrVoiceUnavailable):
			writeError(w, http.StatusServiceUnavailable, service.ErrVoiceUnavailable.Error())
		case errors.Is(err, service.ErrVoiceBusy):
			writeError(w, http.StatusTooManyRequests, service.ErrVoiceBusy.Error())
		case errors.Is(err, model.ErrAIAllowanceExhausted), errors.Is(err, model.ErrExtraAIUsageDisabled):
			writeError(w, http.StatusPaymentRequired, "AI usage exhausted")
		case errors.Is(err, model.ErrExtraAIUsageUnavailable), errors.Is(err, model.ErrExtraAIUsageBillingUnconfigured):
			writeError(w, http.StatusPaymentRequired, "extra AI usage is not available")
		case errors.Is(err, model.ErrBillingWorkspaceLocked), errors.Is(err, model.ErrUsageSettlementFailed):
			writeError(w, http.StatusPaymentRequired, err.Error())
		default:
			writeError(w, http.StatusBadGateway, service.ErrVoiceProvider.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"text": result.Text})
}
