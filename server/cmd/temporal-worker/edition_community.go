//go:build !ee

package main

import (
	"github.com/helpin-ai/helpin/server/internal/config"
	"github.com/helpin-ai/helpin/server/internal/edition"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"gorm.io/gorm"
)

func newEditionServices(db *gorm.DB, cfg *config.Config, workspace *repository.WorkspaceRepository) (*edition.Services, error) {
	return edition.Community(db), nil
}
