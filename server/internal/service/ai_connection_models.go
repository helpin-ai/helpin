package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

const aiModelsCacheTTL = 6 * time.Hour

// ConnectionModels refreshes lazily, keeping successful results when a provider
// fails. It never mutates routes, visibility, defaults, or commercial catalogs.
func (s *AIConnectionService) ConnectionModels(ctx context.Context, workspace, user, id string, refresh bool) (*model.AIConnectionModels, error) {
	if !s.Enabled() {
		return nil, ErrAIConnection
	}
	member, err := s.repo.ActiveMember(ctx, workspace, user)
	if err != nil || !member {
		return nil, ErrAIConnection
	}
	c, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if c == nil || c.SupersededBy != nil {
		return nil, ErrAIConnection
	}
	if err := accessAIConnection(c, workspace, user); err != nil {
		return nil, err
	}
	// Shared readers see the cache/curated list; only managers contact providers.
	canRefresh := c.Scope == "personal" || s.requireConnectionManager(ctx, workspace, user) == nil
	if refresh && !canRefresh {
		return nil, ErrAIConnection
	}
	result := &model.AIConnectionModels{Models: []model.DiscoveredAIModel{}, Source: "catalog"}
	for _, m := range s.Models() {
		if m.Provider == c.Provider {
			result.Models = append(result.Models, model.DiscoveredAIModel{ID: m.SelectionModel, Name: m.Label})
		}
	}
	if c.Funding == "managed" || c.Provider == "openai_chatgpt" {
		return result, nil
	}
	if c.Provider == "openai_compatible" {
		result.Source = "manual"
		return result, nil // Approved runtime endpoints aren't arbitrary API-server destinations.
	}
	if c.ModelsFetchedAt != nil {
		result.Models = c.DiscoveredModels
		if result.Models == nil {
			result.Models = []model.DiscoveredAIModel{}
		}
		result.Source = "provider"
		result.FetchedAt = c.ModelsFetchedAt
		result.Stale = time.Since(*c.ModelsFetchedAt) >= aiModelsCacheTTL
		if !refresh && !result.Stale {
			return result, nil
		}
	}
	if !canRefresh || c.Status != "connected" {
		return result, nil
	}
	_, credential, err := s.Credential(ctx, workspace, user, id, false)
	if err != nil {
		return nil, err
	}
	models, err := discoverProviderModels(ctx, s.modelHTTP, c.Provider, credential.APIKey)
	if err != nil {
		// Provider bodies and transport errors can contain secrets: don't expose them.
		slog.WarnContext(ctx, "AI model discovery failed", "workspace_id", workspace, "connection_id", id, "provider", c.Provider)
		result.Stale = true
		result.Warning = "Could not refresh models. Your saved choices are unchanged."
		return result, nil
	}
	at := time.Now().UTC()
	if err := s.repo.SaveDiscoveredModels(ctx, id, models, at); err != nil {
		return nil, err
	}
	return &model.AIConnectionModels{Models: models, Source: "provider", FetchedAt: &at}, nil
}

type providerModelPage struct {
	Data []struct {
		ID                  string   `json:"id"`
		Name                string   `json:"name"`
		DisplayName         string   `json:"display_name"`
		SupportedParameters []string `json:"supported_parameters"`
		Architecture        struct {
			OutputModalities []string `json:"output_modalities"`
		} `json:"architecture"`
	} `json:"data"`
	HasMore bool   `json:"has_more"`
	LastID  string `json:"last_id"`
}

// Fixed provider URLs and disabled redirects prevent discovery from forwarding
// credentials to user-controlled endpoints. No paid completion is performed.
func discoverProviderModels(ctx context.Context, client *http.Client, provider, key string) ([]model.DiscoveredAIModel, error) {
	endpoint := ""
	switch provider {
	case "openai":
		endpoint = "https://api.openai.com/v1/models"
	case "anthropic":
		endpoint = "https://api.anthropic.com/v1/models?limit=1000"
	case "openrouter":
		endpoint = "https://openrouter.ai/api/v1/models"
	default:
		return nil, errors.New("model discovery is unsupported")
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	models := []model.DiscoveredAIModel{}
	seen := map[string]bool{}
	cursor := ""
	for page := 0; page < 50; page++ {
		address := endpoint
		if cursor != "" {
			address += "&after_id=" + url.QueryEscape(cursor)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
		if err != nil {
			return nil, err
		}
		if provider == "anthropic" {
			req.Header.Set("x-api-key", key)
			req.Header.Set("anthropic-version", "2023-06-01")
		} else {
			req.Header.Set("Authorization", "Bearer "+key)
		}
		response, err := client.Do(req)
		if err != nil {
			return nil, errors.New("model discovery request failed")
		}
		var payload providerModelPage
		decodeErr := func() error {
			defer response.Body.Close()
			if response.StatusCode != http.StatusOK {
				return errors.New("model discovery rejected")
			}
			body, err := io.ReadAll(io.LimitReader(response.Body, 8<<20+1))
			if err != nil || len(body) > 8<<20 {
				return errors.New("invalid model discovery response")
			}
			if err := json.Unmarshal(body, &payload); err != nil {
				return err
			}
			if payload.Data == nil {
				return errors.New("missing model list")
			}
			return nil
		}()
		if decodeErr != nil {
			return nil, decodeErr
		}
		for _, m := range payload.Data {
			if m.ID == "" || len(m.ID) > 200 || strings.ContainsAny(m.ID, "\r\n") || seen[m.ID] {
				continue
			}
			if provider == "openai" && !openAIChatModel(m.ID) {
				continue
			}
			if provider == "openrouter" && (!slices.Contains(m.SupportedParameters, "tools") || !slices.Contains(m.Architecture.OutputModalities, "text")) {
				continue
			}
			name := m.DisplayName
			if name == "" {
				name = m.Name
			}
			if name == "" {
				name = m.ID
			}
			seen[m.ID] = true
			models = append(models, model.DiscoveredAIModel{ID: m.ID, Name: name})
		}
		if provider != "anthropic" || !payload.HasMore {
			sort.Slice(models, func(i, j int) bool { return models[i].ID < models[j].ID })
			return models, nil
		}
		if payload.LastID == "" || payload.LastID == cursor {
			return nil, errors.New("invalid model discovery cursor")
		}
		cursor = payload.LastID
	}
	return nil, errors.New("model discovery pagination limit exceeded")
}

// OpenAI's list has no tool-capability metadata. Exclude known non-agent
// families; runtime validation still applies and exact-ID entry remains available.
func openAIChatModel(id string) bool {
	for _, part := range []string{"embedding", "moderation", "whisper", "tts", "dall-e", "image", "audio", "realtime", "transcribe", "search", "instruct", "deep-research"} {
		if strings.Contains(id, part) {
			return false
		}
	}
	return strings.HasPrefix(id, "gpt-") || strings.HasPrefix(id, "chatgpt-") || strings.HasPrefix(id, "o1") || strings.HasPrefix(id, "o3") || strings.HasPrefix(id, "o4") || strings.HasPrefix(id, "ft:")
}
