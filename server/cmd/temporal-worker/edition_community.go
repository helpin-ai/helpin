//go:build !ee

package main

import (
	"github.com/helpin-ai/helpin/server/internal/config"
	"github.com/helpin-ai/helpin/server/internal/edition"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/service"
	"gorm.io/gorm"
)

func newEditionServices(db *gorm.DB, cfg *config.Config, workspace *repository.WorkspaceRepository) (*edition.Services, error) {
	profiles, err := service.NewAIStandardProfiles(repository.NewAIStandardProfileRepository(db), cfg.AIConnectionEncryptionKey, "customer", map[string]string{"openai": cfg.OpenAIAPIKey, "anthropic": cfg.AnthropicAPIKey, "openrouter": cfg.OpenRouterAPIKey})
	if err != nil {
		return nil, err
	}
	services := edition.Community(db)
	services.InitializeAIProfiles = profiles.EnsureAll
	services.WorkspaceLifecycle = profiles
	return services, nil
}
