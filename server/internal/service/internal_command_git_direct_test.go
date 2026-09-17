package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestDirectPullRequestUsesWorkspaceRepositoryWithoutTask(t *testing.T) {
	db := newTestDB(t)
	ensureGitDeliveryStatusTables(t, db)
	seedGitDeliveryStatusFixture(t, db)
	app := &fakeGitHubAppClient{}
	svc := &InternalCommandService{definitions: map[string]InternalCommandDefinition{}, gitService: newGitDeliveryStatusService(db, app)}
	svc.registerDirectGitCommands()
	def := svc.definitions["git.open_pr"]
	if !def.Mutating || len(def.RequiredPermissionsAll) != 2 || def.RequiredPermissionsAll[0] != authorization.PermPMEdit {
		t.Fatal("publication permissions missing")
	}
	meta := model.InternalCommandContext{WorkspaceID: "ws-1", TargetType: "workspace", TargetID: "ws-1"}
	output, err := def.Execute(context.Background(), meta, json.RawMessage(`{"repository_id":"repo-1","head":"requested-fix","base":"main","title":"Requested fix","body":"Verified tests"}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(app.ensurePRs) != 1 || app.ensurePRs[0].Head != "requested-fix" || !strings.Contains(string(output), "github.test/pr/1") {
		t.Fatalf("incorrect PR: %s %+v", output, app.ensurePRs)
	}
	meta.WorkspaceID = "other-workspace"
	if _, err := def.Execute(context.Background(), meta, json.RawMessage(`{"repository_id":"repo-1","head":"requested-fix","base":"main","title":"Requested fix"}`)); err == nil {
		t.Fatal("cross-workspace repository accepted")
	}
	if len(app.ensurePRs) != 1 {
		t.Fatal("unauthorized publication executed")
	}
}
