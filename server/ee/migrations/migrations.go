//go:build ee

package migrations

import (
	"embed"

	"github.com/helpin-ai/helpin/server/internal/dbmigrate"
)

//go:embed sql/*.sql
var files embed.FS

func Source() dbmigrate.Source { return dbmigrate.Source{FS: files, Directory: "sql"} }
