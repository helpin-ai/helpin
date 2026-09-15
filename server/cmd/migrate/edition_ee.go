//go:build ee

package main

import (
	"github.com/helpin-ai/helpin/server/ee/migrations"
	"github.com/helpin-ai/helpin/server/internal/dbmigrate"
)

func editionMigrationSources() []dbmigrate.Source {
	return []dbmigrate.Source{migrations.Source()}
}
