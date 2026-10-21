package service

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/url"
	"strings"

	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// hydrateSupportSourceLinks updates only citation URLs, preserving all other
// metadata. Resolve a whole page together, including citations saved before
// public URLs were supported. Missing or no-longer-public targets stay unlinked.
func hydrateSupportSourceLinks(ctx context.Context, repo publicLinkTargetRepository, workspaceID string, messages []model.SupportMessage) {
	if repo == nil {
		return
	}
	type citationMessage struct {
		index    int
		metadata map[string]json.RawMessage
		sources  []map[string]json.RawMessage
	}
	var pending []citationMessage
	ids := map[string]struct{}{}
	for i := range messages {
		var metadata map[string]json.RawMessage
		if json.Unmarshal([]byte(messages[i].Metadata), &metadata) != nil {
			continue
		}
		var sources []map[string]json.RawMessage
		if json.Unmarshal(metadata["ai_sources"], &sources) != nil {
			continue
		}
		hasDocs := false
		for _, source := range sources {
			if id := supportCitationDocumentID(source); id != "" {
				ids[id] = struct{}{}
				hasDocs = true
			}
		}
		if hasDocs {
			pending = append(pending, citationMessage{i, metadata, sources})
		}
	}
	if len(ids) == 0 {
		return
	}
	documentIDs := make([]string, 0, len(ids))
	for id := range ids {
		documentIDs = append(documentIDs, id)
	}
	urls, err := supportPublicArticleURLs(ctx, repo, workspaceID, documentIDs)
	if err != nil {
		slog.WarnContext(ctx, "resolve support citation links", "workspace_id", workspaceID, "error", err)
	}
	for _, item := range pending {
		for _, source := range item.sources {
			if id := supportCitationDocumentID(source); id != "" {
				delete(source, "url")
				if link := urls[id]; link != "" {
					encoded, err := json.Marshal(link)
					if err != nil {
						slog.ErrorContext(ctx, "encode support citation URL", "error", err)
						continue
					}
					source["url"] = encoded
				}
			}
		}
		encodedSources, err := json.Marshal(item.sources)
		if err != nil {
			slog.ErrorContext(ctx, "encode support citations", "error", err)
			continue
		}
		item.metadata["ai_sources"] = encodedSources
		encoded, err := json.Marshal(item.metadata)
		if err != nil {
			slog.ErrorContext(ctx, "encode support citation metadata", "error", err)
			continue
		}
		messages[item.index].Metadata = string(encoded)
	}
}

func supportCitationDocumentID(source map[string]json.RawMessage) string {
	var id, sourceType string
	if json.Unmarshal(source["docId"], &id) != nil {
		return ""
	}
	if raw, ok := source["sourceType"]; ok && json.Unmarshal(raw, &sourceType) != nil {
		return ""
	}
	if sourceType != "" && sourceType != knowledgeSourceTypeDocs {
		return ""
	}
	if strings.HasPrefix(id, "docs:") {
		return strings.TrimPrefix(id, "docs:")
	}
	// Older citations used the bare document UUID without a source type.
	if _, err := uuid.Parse(id); err == nil {
		return id
	}
	return ""
}

func supportPublicArticleURLs(ctx context.Context, repo publicLinkTargetRepository, workspaceID string, ids []string) (map[string]string, error) {
	cfg, err := repo.GetConfig(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	if cfg == nil || !cfg.IsPublished {
		return nil, nil
	}
	base := supportHelpcenterPublicBaseURL(cfg)
	if base == "" {
		return nil, nil
	}
	targets, err := repo.ListLiveArticleLinkTargets(ctx, workspaceID, defaultHelpcenterLocale(cfg), ids)
	if err != nil {
		return nil, err
	}
	urls := make(map[string]string, len(targets))
	for _, target := range targets {
		if target.Slug == "" || target.PublicID == "" {
			continue
		}
		urls[target.DocumentID] = base + buildDocsHelpcenterArticleCanonicalPath(cfg, target.Locale, target.Slug, target.PublicID)
	}
	return urls, nil
}

// Match the Help Center's canonical origin and reverse-proxy path settings.
func supportHelpcenterPublicBaseURL(cfg *model.DocsHelpcenterConfig) string {
	host, path := "", ""
	switch cfg.PublicURLMode {
	case model.HelpcenterPublicURLModeCustomDomain:
		host = stringPtrTrimmed(cfg.CustomDomain)
	case model.HelpcenterPublicURLModeReverseProxy:
		host = stringPtrTrimmed(cfg.ReverseProxyHost)
		normalized, err := normalizeHelpcenterBasePath(cfg.ReverseProxyBasePath)
		if err != nil || normalized == nil {
			return ""
		}
		path = *normalized
	default:
		if strings.TrimSpace(cfg.Subdomain) == "" {
			return ""
		}
		host = strings.TrimSpace(cfg.Subdomain) + ".helpin.center"
	}
	host = strings.TrimPrefix(strings.TrimPrefix(host, "https://"), "http://")
	parsed, err := url.Parse("https://" + host)
	if err != nil || parsed.Hostname() == "" || parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return ""
	}
	return parsed.String() + path
}

func (s *SupportInboxService) hydrateSourceLinks(ctx context.Context, workspaceID string, messages []model.SupportMessage) {
	if s.docsHelpcenterRepo != nil {
		hydrateSupportSourceLinks(ctx, s.docsHelpcenterRepo, workspaceID, messages)
	}
}

func (s *SupportAIService) hydrateSourceLinks(ctx context.Context, message *model.SupportMessage) {
	if s.db == nil {
		return
	}
	messages := []model.SupportMessage{*message}
	hydrateSupportSourceLinks(ctx, repository.NewDocsHelpcenterRepository(s.db), message.WorkspaceID, messages)
	message.Metadata = messages[0].Metadata
}
