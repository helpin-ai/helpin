package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestDocsLifecycleCommands(t *testing.T) {
	for _, tc := range []struct {
		name, command, status, wantStatus, wantError     string
		locked, foreign, private, live, conflict, viewer bool
	}{
		{name: "archive", command: "archive_document", status: model.DocStatusDraft, wantStatus: model.DocStatusArchived},
		{name: "repeat archive", command: "archive_document", status: model.DocStatusArchived, wantStatus: model.DocStatusArchived},
		{name: "restore", command: "restore_document", status: model.DocStatusArchived, wantStatus: model.DocStatusDraft},
		{name: "restore draft", command: "restore_document", status: model.DocStatusDraft, wantError: "not archived"},
		{name: "locked archive", command: "archive_document", status: model.DocStatusDraft, locked: true, wantError: "locked"},
		{name: "locked restore", command: "restore_document", status: model.DocStatusArchived, locked: true, wantError: "locked"},
		{name: "foreign archive", command: "archive_document", status: model.DocStatusDraft, foreign: true, wantError: "not found"},
		{name: "foreign restore", command: "restore_document", status: model.DocStatusArchived, foreign: true, wantError: "not found"},
		{name: "private space", command: "archive_document", status: model.DocStatusDraft, private: true, wantError: "not found"},
		{name: "live draft", command: "archive_document", status: model.DocStatusDraft, live: true, wantError: "unpublish"},
		{name: "conflicting target", command: "archive_document", status: model.DocStatusDraft, conflict: true, wantError: "conflicts"},
		{name: "viewer", command: "archive_document", status: model.DocStatusDraft, viewer: true, wantError: "permission"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := setupDocsDeletionTestDB(t)
			space := model.DocsSpace{ID: "space", WorkspaceID: "ws", Name: "Docs", Slug: "docs", Type: model.SpaceTypeInternal, Visibility: model.SpaceVisibilityWorkspaceWide, CreatedBy: "owner"}
			if tc.private {
				space.Visibility = model.SpaceVisibilityTeamOnly
			}
			if err := db.Create(&space).Error; err != nil {
				t.Fatal(err)
			}
			if tc.private {
				if err := db.Exec("INSERT INTO docs_space_teams(space_id, team_id) VALUES('space','private-team')").Error; err != nil {
					t.Fatal(err)
				}
			}
			doc := model.DocsDocument{ID: "doc", WorkspaceID: "ws", SpaceID: "space", Title: "Keep content", Status: tc.status, Visibility: model.SpaceVisibilityWorkspaceWide, CreatedBy: "owner", IsLocked: tc.locked}
			if tc.foreign {
				doc.WorkspaceID = "other"
			}
			if err := db.Create(&doc).Error; err != nil {
				t.Fatal(err)
			}
			docRepo := repository.NewDocsDocumentRepository(db)
			spaceRepo := repository.NewDocsSpaceRepository(db)
			docSvc := NewDocsDocumentService(docRepo, spaceRepo, nil, false)
			if tc.live {
				if err := db.Exec("INSERT INTO docs_helpcenter_articles(id, document_id, workspace_id, space_id, slug, public_published_at) VALUES('article','doc','ws','space','doc',?)", time.Now()).Error; err != nil {
					t.Fatal(err)
				}
				docSvc.SetHelpcenterService(NewDocsHelpcenterService(repository.NewDocsHelpcenterRepository(db), nil, docRepo, nil, spaceRepo, nil, nil, nil, nil))
			}
			svc := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)
			svc.SetAuthorizationService(authorization.NewAuthzService(nil, nil, nil))
			svc.SetDocsCreateDependencies(docSvc, nil)
			svc.SetDocsOrganizationServices(NewDocsSpaceService(spaceRepo, nil), nil)
			meta := model.InternalCommandContext{WorkspaceID: "ws", ActorID: "owner", ActorRole: model.RoleOwner, TargetType: "workspace", TargetID: "ws"}
			if tc.private {
				meta.ActorRole = model.RoleMember
			}
			if tc.viewer {
				meta.ActorRole = model.RoleViewer
			}
			if tc.conflict {
				meta.TargetType = "document"
				meta.TargetID = "another"
			}
			output, err := svc.Execute(context.Background(), meta, "docs."+tc.command, json.RawMessage(`{"document_id":"doc"}`))
			if tc.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantError) {
					t.Fatalf("error = %v; want %q", err, tc.wantError)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				var state map[string]any
				if err := json.Unmarshal(output, &state); err != nil {
					t.Fatal(err)
				}
				if state["status"] != tc.wantStatus {
					t.Fatalf("result = %s", output)
				}
			}
			saved, err := docRepo.GetByID(context.Background(), "doc")
			if err != nil {
				t.Fatal(err)
			}
			expected := tc.wantStatus
			if tc.wantError != "" {
				expected = tc.status
			}
			if saved.Status != expected || saved.Title != doc.Title {
				t.Fatalf("unexpected document: %#v", saved)
			}
		})
	}
}
