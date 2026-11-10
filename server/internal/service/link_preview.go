package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/url"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/deployment"
	"github.com/helpin-ai/helpin/server/internal/linkpreview"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// LinkPreviewService resolves anonymous projections without invoking
// authenticated services with a missing actor or caching authorization results.
type LinkPreviewService struct {
	repo    *repository.LinkPreviewRepository
	modules map[model.ModuleID]bool
}

// NewLinkPreviewService creates a resolver using the deployment's module policy.
func NewLinkPreviewService(repo *repository.LinkPreviewRepository, modules []model.ModuleID) *LinkPreviewService {
	s := &LinkPreviewService{repo: repo, modules: map[model.ModuleID]bool{}}
	for _, module := range modules {
		s.modules[module] = true
	}
	return s
}

// Resolve always returns a usable card. Failed or ineligible lookups use the same
// generic presentation and never echo query text or private error messages.
func (s *LinkPreviewService) Resolve(ctx context.Context, path, rawQuery string) model.LinkPreview {
	t := linkpreview.Resolve(path, rawQuery)
	card := s.resolve(ctx, t)
	card = linkpreview.Normalize(card)
	// Cache revisions describe only public card contents. Settings/permissions are
	// re-read on every image request, including requests carrying an old revision.
	encoded, err := json.Marshal(card)
	if err != nil {
		slog.ErrorContext(ctx, "encode link preview revision", "error", err)
	}
	digest := sha256.Sum256(encoded)
	identity := t.Path
	canonical := identity
	if t.Slug == "" && t.Token == "" {
		parts := strings.Split(strings.Trim(identity, "/"), "/")
		if len(parts) > 1 && (parts[0] == "join" || parts[0] == "team-reply" || parts[0] == "email-preferences") {
			identity = "/" + parts[0] + "/preview"
			canonical = "/"
		}
	}
	if params := t.Query.Encode(); params != "" {
		canonical += "?" + params
	}
	card.URL = canonical
	params := url.Values{"path": {identity}, "v": {linkpreview.TemplateVersion + "-" + hex.EncodeToString(digest[:12])}}
	if query := t.Query.Encode(); query != "" {
		params.Set("query", query)
	}
	card.ImageURL = "/api/link-previews/image?" + params.Encode()
	return card
}

func (s *LinkPreviewService) resolve(ctx context.Context, t linkpreview.Target) model.LinkPreview {
	if t.Token != "" {
		var record *model.LinkPreviewRecord
		var err error
		if t.Kind == "document" {
			record, err = s.repo.GetPublicDocument(ctx, t.Token)
		} else {
			record, err = s.repo.GetPublicResource(ctx, t.Token)
		}
		return s.recordOrFallback(ctx, record, err, t.Fallback)
	}
	if t.Slug == "" {
		return t.Fallback
	}
	if !s.enabled(t.Module) {
		return linkpreview.TypeOnly("")
	}
	if deployment.EditionName != "community" && (strings.HasSuffix(t.Path, "/settings/server") || strings.HasSuffix(t.Path, "/settings/system-status")) {
		return linkpreview.Resolve("/w/placeholder/settings/general", "").Fallback
	}
	ws, err := s.repo.GetWorkspace(ctx, t.Slug)
	if err != nil {
		slog.WarnContext(ctx, "resolve link preview workspace", "error", err)
		return t.Fallback
	}
	if ws == nil || !ws.LinkPreviewsEnabled || t.Kind == "" {
		return t.Fallback
	}
	record, err := s.repo.GetRecord(ctx, ws, t)
	return s.recordOrFallback(ctx, record, err, t.Fallback)
}

func (s *LinkPreviewService) recordOrFallback(ctx context.Context, record *model.LinkPreviewRecord, err error, fallback model.LinkPreview) model.LinkPreview {
	if err != nil {
		slog.WarnContext(ctx, "resolve link preview record", "error", err)
		return fallback
	}
	if record == nil {
		return fallback
	}
	for _, module := range record.Modules {
		if !s.enabled(module) {
			return linkpreview.TypeOnly("")
		}
	}
	return record.Preview
}

func (s *LinkPreviewService) enabled(module model.ModuleID) bool {
	return module == "" || s.modules[module]
}
