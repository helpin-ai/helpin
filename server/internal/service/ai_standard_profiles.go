package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/aimodel"
	"github.com/helpin-ai/helpin/server/internal/crypto"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// AIStandardProfiles provisions shared sizes without modifying agent selections.
type AIStandardProfiles struct {
	store       *repository.AIStandardProfileRepository
	key         []byte
	funding     string
	credentials map[string]string
}

func NewAIStandardProfiles(store *repository.AIStandardProfileRepository, encryptionKey, funding string, credentials map[string]string) (*AIStandardProfiles, error) {
	if funding != "managed" && funding != "customer" {
		return nil, errors.New("invalid standard connection funding")
	}
	var key []byte
	if encryptionKey != "" {
		var err error
		key, err = parseExternalMCPEncryptionKey(encryptionKey)
		if err != nil {
			return nil, errors.New("invalid AI_CONNECTION_ENCRYPTION_KEY")
		}
	}
	if funding == "managed" && len(key) != 32 {
		return nil, errors.New("AI_CONNECTION_ENCRYPTION_KEY is required for managed connections")
	}
	return &AIStandardProfiles{store: store, key: key, funding: funding, credentials: credentials}, nil
}

func (s *AIStandardProfiles) EnsureAll(ctx context.Context) error {
	ids, err := s.store.Workspaces(ctx)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err := s.EnsureWorkspace(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

func (s *AIStandardProfiles) EnsureWorkspace(ctx context.Context, workspace string) error {
	return s.store.Transaction(ctx, workspace, func(store *repository.AIStandardProfileRepository) error {
		connections := map[string]*model.AIConnection{}
		for _, tier := range []aimodel.Tier{aimodel.TierSmall, aimodel.TierMedium, aimodel.TierLarge, aimodel.TierFlagship} {
			route, err := standardModelForTier(string(tier))
			if err != nil {
				return err
			}
			c := connections[route.Provider]
			if c == nil {
				var err error
				c, err = s.ensureConnection(ctx, store, workspace, route.Provider)
				if err != nil {
					return err
				}
				connections[route.Provider] = c
			}
			now := time.Now().UTC()
			p := &model.AIProfile{ID: model.StandardAIProfileID(workspace, string(tier)), WorkspaceID: workspace, Scope: "workspace", Name: strings.ToUpper(string(tier[:1])) + string(tier[1:]), Revision: 1,
				Primary: model.AIProfileRoute{ConnectionID: c.ID, Model: route}, CreatedAt: now, UpdatedAt: now}
			if _, err := store.EnsureProfile(ctx, p); err != nil {
				return err
			}
		}
		small := model.StandardAIProfileID(workspace, "small")
		return store.EnsureDefault(ctx, &model.AIWorkspaceSettings{WorkspaceID: workspace, DefaultProfileID: &small, UpdatedAt: time.Now().UTC()})
	})
}

func (s *AIStandardProfiles) ensureConnection(ctx context.Context, store *repository.AIStandardProfileRepository, workspace, provider string) (*model.AIConnection, error) {
	id := model.StandardAIConnectionID(workspace, provider)
	c, err := store.Connection(ctx, id)
	if err != nil {
		return nil, err
	}
	fresh := c == nil
	if fresh {
		c = &model.AIConnection{ID: id, WorkspaceID: workspace, Scope: "workspace", Funding: s.funding, Name: "Standard " + provider, Provider: provider, Status: "unconfigured", CreatedAt: time.Now().UTC()}
	}
	if c.WorkspaceID != workspace || c.Scope != "workspace" || c.UserID != nil || c.Provider != provider {
		return nil, errors.New("standard connection identity conflicts with existing configuration")
	}
	// Only an untouched placeholder may change funding. Customer keys and
	// explicitly disconnected connections are never adopted or reconnected.
	placeholder := c.Status == "unconfigured" && len(c.EncryptedSecret) == 0
	if !fresh && !placeholder && (c.Funding != "managed" || s.funding != "managed" || c.Status != "connected") {
		return c, nil
	}
	changed := fresh
	if placeholder && c.Funding != s.funding && s.funding == "managed" {
		c.Funding = s.funding
		changed = true
	}
	if apiKey := strings.TrimSpace(s.credentials[provider]); apiKey != "" && len(s.key) == 32 {
		if len(apiKey) > 65536 || strings.ContainsAny(apiKey, "\r\n") {
			return nil, errors.New("invalid managed provider credential")
		}
		oldKey := ""
		if len(c.EncryptedSecret) > 0 {
			secret, err := (&AIConnectionService{key: s.key}).open(c)
			if err != nil {
				return nil, errors.New("cannot decrypt standard managed connection")
			}
			oldKey = secret.APIKey
		}
		if oldKey != apiKey {
			raw, err := json.Marshal(aiConnectionSecret{APIKey: apiKey})
			if err != nil {
				return nil, err
			}
			c.EncryptedSecret, err = crypto.EncryptWithAAD(raw, s.key, aiConnectionAAD(c))
			if err != nil {
				return nil, err
			}
			c.Status = "connected"
			changed = true
		}
	}
	if changed {
		c.UpdatedAt = time.Now().UTC()
		if err := store.SaveConnection(ctx, c); err != nil {
			return nil, err
		}
	}
	return c, nil
}

func (s *AIStandardProfiles) WorkspaceCreated(ctx context.Context, workspace string) error {
	return s.EnsureWorkspace(ctx, workspace)
}
func (s *AIStandardProfiles) WorkspaceDeleting(context.Context, string) error { return nil }
