package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

const docsAPIReferenceSpecV1 = `openapi: 3.1.0
info:
  title: Product API
  version: 1.0.0
paths:
  /users:
    get:
      operationId: listUsers
      responses:
        "200":
          description: OK
components:
  schemas:
    User:
      type: object
`

func TestNormalizeDocsOpenAPISpec(t *testing.T) {
	normalized, document, err := normalizeDocsOpenAPISpec(docsAPIReferenceSpecV1)
	if err != nil {
		t.Fatalf("normalize valid YAML: %v", err)
	}
	if len(normalized) == 0 {
		t.Fatal("expected normalized specification")
	}
	operations, schemas, warnings := inspectDocsOpenAPISpec(document)
	if operations != 1 || schemas != 1 {
		t.Fatalf("unexpected counts: operations=%d schemas=%d", operations, schemas)
	}
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
}

func TestNormalizeDocsOpenAPISpecRejectsUnsafeOrInvalidDocuments(t *testing.T) {
	tests := map[string]string{
		"swagger 2":     `{"swagger":"2.0","info":{"title":"Old"},"paths":{}}`,
		"external ref":  `{"openapi":"3.0.3","info":{"title":"API"},"paths":{},"components":{"schemas":{"User":{"$ref":"https://example.com/user.json"}}}}`,
		"missing title": `{"openapi":"3.0.3","info":{},"paths":{}}`,
		"trailing json": `{"openapi":"3.0.3","info":{"title":"API"},"paths":{}} {"extra":true}`,
	}
	for name, specification := range tests {
		t.Run(name, func(t *testing.T) {
			if _, _, err := normalizeDocsOpenAPISpec(specification); !errors.Is(err, ErrDocsAPIReferenceInvalidSpec) {
				t.Fatalf("expected invalid spec error, got %v", err)
			}
		})
	}
}

func TestValidateDocsOpenAPIURL(t *testing.T) {
	valid, err := validateDocsOpenAPIURL("https://api.example.com/openapi.json#fragment")
	if err != nil {
		t.Fatalf("validate public URL: %v", err)
	}
	if valid.Fragment != "" {
		t.Fatalf("expected fragment to be removed, got %q", valid.Fragment)
	}

	invalid := []string{
		"http://api.example.com/openapi.json",
		"https://127.0.0.1/openapi.json",
		"https://10.0.0.2/openapi.json",
		"https://user:pass@api.example.com/openapi.json",
		"https://api.example.com:8443/openapi.json",
	}
	for _, rawURL := range invalid {
		t.Run(rawURL, func(t *testing.T) {
			if _, err := validateDocsOpenAPIURL(rawURL); !errors.Is(err, ErrDocsAPIReferenceInvalidSource) {
				t.Fatalf("expected invalid source error, got %v", err)
			}
		})
	}
}

func TestDocsAPIReferenceLifecycleKeepsPublishedSnapshotStable(t *testing.T) {
	db := setupDocsAPIReferenceTestDB(t)
	ctx := context.Background()
	workspaceID := "workspace-api-reference"
	externalSpaceID := "space-external"
	internalSpaceID := "space-internal"
	for _, values := range []struct {
		id, slug, spaceType string
	}{
		{externalSpaceID, "developers", model.SpaceTypeExternalCapable},
		{internalSpaceID, "internal", model.SpaceTypeInternal},
	} {
		if err := db.Exec(
			`INSERT INTO docs_spaces (id, workspace_id, name, slug, visibility, type, created_by, created_at, updated_at)
			 VALUES (?, ?, ?, ?, 'workspace_wide', ?, 'user-1', ?, ?)`,
			values.id, workspaceID, values.slug, values.slug, values.spaceType, time.Now(), time.Now(),
		).Error; err != nil {
			t.Fatalf("insert space: %v", err)
		}
	}

	service := NewDocsAPIReferenceService(
		repository.NewDocsAPIReferenceRepository(db),
		repository.NewDocsSpaceRepository(db),
	)
	created, err := service.Create(ctx, workspaceID, externalSpaceID, "user-1", model.CreateDocsAPIReferenceRequest{
		Name:              "Product API",
		SourceType:        model.DocsAPIReferenceSourceUpload,
		SpecificationText: docsAPIReferenceSpecV1,
	})
	if err != nil {
		t.Fatalf("create API reference: %v", err)
	}
	if created.DraftRevision == nil || created.DraftRevision.OperationCount != 1 {
		t.Fatalf("unexpected draft: %#v", created.DraftRevision)
	}

	if _, err := service.Create(ctx, workspaceID, internalSpaceID, "user-1", model.CreateDocsAPIReferenceRequest{
		Name:              "Private API",
		SourceType:        model.DocsAPIReferenceSourceUpload,
		SpecificationText: docsAPIReferenceSpecV1,
	}); !errors.Is(err, ErrDocsAPIReferenceExternalSpace) {
		t.Fatalf("expected external-space error, got %v", err)
	}

	published, err := service.Publish(ctx, workspaceID, created.ID)
	if err != nil {
		t.Fatalf("publish API reference: %v", err)
	}
	if published.PublishedRevisionID == nil {
		t.Fatal("expected a published revision")
	}

	publicV1, err := service.GetPublic(ctx, workspaceID, externalSpaceID, created.Slug)
	if err != nil {
		t.Fatalf("get public API reference: %v", err)
	}
	if publicV1.OperationCount != 1 || publicV1.APIVersion != "1.0.0" {
		t.Fatalf("unexpected public snapshot: %#v", publicV1)
	}

	specV2 := strings.Replace(docsAPIReferenceSpecV1, "version: 1.0.0", "version: 2.0.0", 1)
	specV2 = strings.Replace(
		specV2,
		"components:",
		"  /teams:\n    get:\n      operationId: listTeams\n      responses:\n        \"200\":\n          description: OK\ncomponents:",
		1,
	)
	if _, err := service.Update(ctx, workspaceID, created.ID, "user-1", model.UpdateDocsAPIReferenceRequest{
		SpecificationText: &specV2,
	}); err != nil {
		t.Fatalf("create second draft: %v", err)
	}
	stillV1, err := service.GetPublic(ctx, workspaceID, externalSpaceID, created.Slug)
	if err != nil {
		t.Fatalf("get stable public snapshot: %v", err)
	}
	if stillV1.APIVersion != "1.0.0" {
		t.Fatalf("draft leaked publicly: got version %q", stillV1.APIVersion)
	}

	if _, err := service.Publish(ctx, workspaceID, created.ID); err != nil {
		t.Fatalf("publish second draft: %v", err)
	}
	publicV2, err := service.GetPublic(ctx, workspaceID, externalSpaceID, created.Slug)
	if err != nil {
		t.Fatalf("get second public snapshot: %v", err)
	}
	if publicV2.APIVersion != "2.0.0" || publicV2.OperationCount != 2 {
		t.Fatalf("unexpected second snapshot: %#v", publicV2)
	}

	if _, err := service.Unpublish(ctx, workspaceID, created.ID); err != nil {
		t.Fatalf("unpublish API reference: %v", err)
	}
	if _, err := service.GetPublic(ctx, workspaceID, externalSpaceID, created.Slug); !errors.Is(err, ErrDocsAPIReferenceNotFound) {
		t.Fatalf("expected unpublished reference to be hidden, got %v", err)
	}
}

func setupDocsAPIReferenceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(
		sqlite.Open(fmt.Sprintf("file:docs-api-reference-%d?mode=memory&cache=shared", time.Now().UnixNano())),
		&gorm.Config{},
	)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	statements := []string{
		`CREATE TABLE docs_spaces (
			id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, team_id TEXT, name TEXT NOT NULL,
			slug TEXT NOT NULL, icon TEXT, visibility TEXT NOT NULL, type TEXT NOT NULL,
			default_review_days INTEGER, is_system BOOLEAN NOT NULL DEFAULT 0,
			position INTEGER NOT NULL DEFAULT 0, created_by TEXT NOT NULL,
			created_at DATETIME, updated_at DATETIME, deleted_at DATETIME
		)`,
		`CREATE TABLE docs_api_references (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			workspace_id TEXT NOT NULL, space_id TEXT NOT NULL, name TEXT NOT NULL, slug TEXT NOT NULL,
			source_type TEXT NOT NULL, source_url TEXT, sync_enabled BOOLEAN NOT NULL DEFAULT 0,
			sync_status TEXT NOT NULL DEFAULT 'ready', last_sync_error TEXT, last_synced_at DATETIME,
			draft_revision_id TEXT, published_revision_id TEXT, published_at DATETIME,
			position INTEGER NOT NULL DEFAULT 0, created_by TEXT NOT NULL,
			created_at DATETIME, updated_at DATETIME, deleted_at DATETIME
		)`,
		`CREATE UNIQUE INDEX idx_docs_api_reference_space_slug ON docs_api_references(space_id, slug)`,
		`CREATE TABLE docs_api_reference_revisions (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			api_reference_id TEXT NOT NULL, workspace_id TEXT NOT NULL, source_hash TEXT NOT NULL,
			openapi_version TEXT NOT NULL, api_version TEXT NOT NULL DEFAULT '',
			specification TEXT NOT NULL, warnings TEXT, operation_count INTEGER NOT NULL DEFAULT 0,
			schema_count INTEGER NOT NULL DEFAULT 0, created_by TEXT NOT NULL, created_at DATETIME
		)`,
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("create test table: %v", err)
		}
	}
	return db
}
