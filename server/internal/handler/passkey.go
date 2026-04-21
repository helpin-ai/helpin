package handler

import (
	"errors"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

type PasskeyHandler struct {
	passkeyService *service.PasskeyService
}

func NewPasskeyHandler(passkeyService *service.PasskeyService) *PasskeyHandler {
	return &PasskeyHandler{passkeyService: passkeyService}
}

func (h *PasskeyHandler) RegistrationOptions(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	resp, err := h.passkeyService.BeginRegistration(r.Context(), userID)
	if err != nil {
		writePasskeyError(w, err, http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *PasskeyHandler) Register(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	var req model.PasskeyRegisterRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	resp, err := h.passkeyService.FinishRegistration(r.Context(), userID, req, r.UserAgent())
	if err != nil {
		writePasskeyError(w, err, http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusCreated, resp)
}

func (h *PasskeyHandler) AuthenticationOptions(w http.ResponseWriter, r *http.Request) {
	var req model.PasskeyAuthenticationOptionsRequest
	if err := decodeJSON(r, &req); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	resp, err := h.passkeyService.BeginAuthentication(r.Context(), req)
	if err != nil {
		if errors.Is(err, service.ErrNoPasskeyForAccount) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writePasskeyError(w, err, http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *PasskeyHandler) Authenticate(w http.ResponseWriter, r *http.Request) {
	var req model.PasskeyAuthenticateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	resp, err := h.passkeyService.FinishAuthentication(r.Context(), req)
	if err != nil {
		writePasskeyError(w, err, http.StatusUnauthorized)
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *PasskeyHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	resp, err := h.passkeyService.ListPasskeys(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *PasskeyHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	passkeyID := chi.URLParam(r, "id")

	if err := h.passkeyService.DeletePasskey(r.Context(), userID, passkeyID); err != nil {
		if errors.Is(err, service.ErrPasskeyNotFound) {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writePasskeyError(w, err, http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "passkey deleted"})
}

func writePasskeyError(w http.ResponseWriter, err error, fallbackStatus int) {
	switch {
	case errors.Is(err, service.ErrPasskeyNotConfigured):
		writeError(w, http.StatusServiceUnavailable, err.Error())
	case errors.Is(err, service.ErrPasskeyNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, service.ErrNoPasskeyForAccount):
		writeError(w, fallbackStatus, err.Error())
	default:
		writeError(w, fallbackStatus, err.Error())
	}
}
