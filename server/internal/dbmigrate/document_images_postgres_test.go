//go:build integration

package dbmigrate

import (
	"context"
	"database/sql"
	"fmt"
	_ "github.com/jackc/pgx/v5/stdlib"
	"os"
	"testing"
	"time"
)

func TestDocumentImageRecoveryPostgres(t *testing.T) {
	dsn := os.Getenv("DOCS_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set DOCS_TEST_DATABASE_URL to a disposable PostgreSQL database")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := tx.ExecContext(ctx, q, args...); err != nil {
			t.Fatal(err)
		}
	}
	// Roll back the entire isolated schema and migration after testing.
	schema := fmt.Sprintf("image_recovery_%d", time.Now().UnixNano())
	exec("CREATE SCHEMA " + schema)
	exec("SET LOCAL search_path TO " + schema + ", public")
	exec(`CREATE TABLE docs_documents(id uuid PRIMARY KEY, created_by uuid NOT NULL, deleted_at timestamptz, updated_at timestamptz);
 CREATE TABLE docs_contents(document_id uuid PRIMARY KEY, content jsonb, content_text text, word_count int, updated_at timestamptz);
 CREATE TABLE docs_versions(id uuid PRIMARY KEY, document_id uuid, content jsonb, content_text text, snapshot_label text, version_type text, word_count int, created_by uuid, created_at timestamptz);
 CREATE TABLE docs_blocks(id uuid PRIMARY KEY, document_id uuid, content jsonb, type text, revision int, updated_at timestamptz, deleted_at timestamptz);`)
	const doc = "10000000-0000-0000-0000-000000000001"
	const original = `{"type":"doc","content":[{"type":"image","attrs":{"src":"https://example.test/a.png","metadata":{"type":"image"},"number":9007199254740993}},{"type":"bulletList","content":[{"type":"listItem","content":[{"type":"paragraph"},{"type":"image","attrs":{"alt":"Keep this"}}]}]}]}`
	exec(`INSERT INTO docs_documents(id,created_by) VALUES($1,$1)`, doc)
	exec(`INSERT INTO docs_contents VALUES($1,$2,'Original text',2,now())`, doc, original)
	exec(`INSERT INTO docs_blocks VALUES($1,$1,($2::jsonb)->'content'->0,'image',3,now(),NULL)`, doc, original)
	migration, err := migrationFiles.ReadFile("sql/202611100001_normalize_document_image_nodes.sql")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		exec(string(migration))
		var snapshots int
		var preserved, normalized, blockOK bool
		err = tx.QueryRowContext(ctx, `SELECT (SELECT count(*) FROM docs_versions),
  (SELECT content=$2::jsonb AND content_text='Original text' AND word_count=2 FROM docs_versions WHERE document_id=$1),
  (SELECT content->'content'->0->>'type'='resizableImage' AND content->'content'->1->'content'->0->'content'->1->>'type'='resizableImage'
   AND content->'content'->0->'attrs'=($2::jsonb)->'content'->0->'attrs' FROM docs_contents WHERE document_id=$1),
  (SELECT type='resizableImage' AND content->>'type'='resizableImage' AND revision=4 FROM docs_blocks WHERE document_id=$1)`, doc, original).Scan(&snapshots, &preserved, &normalized, &blockOK)
		if err != nil {
			t.Fatal(err)
		}
		if snapshots != 1 || !preserved || !normalized || !blockOK {
			t.Fatalf("pass %d: snapshots=%d preserved=%v normalized=%v blocks=%v", i, snapshots, preserved, normalized, blockOK)
		}
	}
}
