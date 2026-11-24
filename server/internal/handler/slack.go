package handler

import (
	"errors"
	"github.com/go-chi/chi/v5"
	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/service"
	"io"
	"log/slog"
	"net/http"
)

// SlackHandler handles signed Slack events and workspace-scoped settings.
type SlackHandler struct{ s *service.SlackService }

func NewSlackHandler(s *service.SlackService) *SlackHandler { return &SlackHandler{s: s} }

func slackResponse(w http.ResponseWriter, r *http.Request, out any, err error) {
	if err == nil {
		writeJSON(w, 200, out)
		return
	}
	switch {
	case errors.Is(err, service.ErrChannelWebhookUnauthorized):
		writeError(w, 401, "Invalid Slack signature")
	case errors.Is(err, service.ErrChannelInvalid), errors.Is(err, service.ErrChannelWebhookInvalid):
		writeError(w, 400, err.Error())
	case errors.Is(err, repository.ErrSupportChannelAccountNotFound):
		writeError(w, 404, "Connect Slack first")
	case errors.Is(err, service.ErrChannelAlreadyConnected):
		writeError(w, 409, "Slack workspace is already connected")
	default:
		slog.ErrorContext(r.Context(), "Slack request failed", "error", err)
		writeError(w, 503, "Could not complete the Slack request")
	}
}
func (h *SlackHandler) Event(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		writeError(w, 413, "Payload too large")
		return
	}
	challenge, err := h.s.AcceptEvent(r.Context(), r.Header.Get("X-Slack-Request-Timestamp"), r.Header.Get("X-Slack-Signature"), body)
	slackResponse(w, r, map[string]string{"challenge": challenge}, err)
}
func (h *SlackHandler) MessageSource(w http.ResponseWriter, r *http.Request) {
	link, err := h.s.MessageSource(r.Context(), getWorkspaceID(r), chi.URLParam(r, "id"))
	slackResponse(w, r, map[string]string{"permalink": link}, err)
}

func (h *SlackHandler) Settings(w http.ResponseWriter, r *http.Request) {
	data, err := h.s.Settings(r.Context(), getWorkspaceID(r))
	slackResponse(w, r, data, err)
}
func (h *SlackHandler) Start(w http.ResponseWriter, r *http.Request) {
	data, err := h.s.StartInstall(r.Context(), getWorkspaceID(r), middleware.GetUserID(r.Context()))
	slackResponse(w, r, map[string]string{"url": data}, err)
}
func (h *SlackHandler) Finish(w http.ResponseWriter, r *http.Request) {
	var v struct{ State, Code string }
	if err := decodeJSON(r, &v); err != nil {
		writeError(w, 400, "Invalid authorization")
		return
	}
	err := h.s.CompleteInstall(r.Context(), getWorkspaceID(r), middleware.GetUserID(r.Context()), v.State, v.Code)
	slackResponse(w, r, map[string]bool{"connected": err == nil}, err)
}
func (h *SlackHandler) Channels(w http.ResponseWriter, r *http.Request) {
	data, err := h.s.AvailableChannels(r.Context(), getWorkspaceID(r))
	slackResponse(w, r, data, err)
}
func (h *SlackHandler) Configure(w http.ResponseWriter, r *http.Request) {
	var v model.SlackRoute
	if err := decodeJSON(r, &v); err != nil {
		writeError(w, 400, "Invalid channel")
		return
	}
	slackResponse(w, r, true, h.s.Configure(r.Context(), getWorkspaceID(r), v))
}
func (h *SlackHandler) Remove(w http.ResponseWriter, r *http.Request) {
	slackResponse(w, r, true, h.s.Remove(r.Context(), getWorkspaceID(r), chi.URLParam(r, "channel")))
}
func (h *SlackHandler) Disconnect(w http.ResponseWriter, r *http.Request) {
	slackResponse(w, r, true, h.s.Disconnect(r.Context(), getWorkspaceID(r)))
}

func (h *SlackHandler) Link(w http.ResponseWriter, r *http.Request) {
	var v struct{ Token string }
	if err := decodeJSON(r, &v); err != nil {
		writeError(w, 400, "Invalid link")
		return
	}
	slackResponse(w, r, true, h.s.LinkMember(r.Context(), getWorkspaceID(r), middleware.GetUserID(r.Context()), v.Token))
}

func (h *SlackHandler) Action(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		writeError(w, 413, "Payload too large")
		return
	}
	out, err := h.s.Action(r.Context(), r.Header.Get("X-Slack-Request-Timestamp"), r.Header.Get("X-Slack-Signature"), body)
	slackResponse(w, r, out, err)
}

func (h *SlackHandler) SetAI(w http.ResponseWriter, r *http.Request) {
	var v struct{ Enabled bool }
	if err := decodeJSON(r, &v); err != nil {
		writeError(w, 400, "Invalid settings")
		return
	}
	slackResponse(w, r, true, h.s.SetAI(r.Context(), getWorkspaceID(r), v.Enabled))
}
