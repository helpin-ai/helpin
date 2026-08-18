package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/prcontent"
)

func TestBuildDelegatedRunPullRequestContentIncludesReviewHandoffAndHelpinLinks(t *testing.T) {
	db := newTestDB(t)
	seedGitDeliveryStatusFixture(t, db)
	now := time.Now().UTC()
	mustExec(t, db, `INSERT INTO workspaces (
		id, name, slug, workspace_key, owner_id, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"ws-1", "Demo Workspace", "demo workspace", "HEL", "user-1", now, now)

	svc := newGitDeliveryStatusService(db, &fakeGitHubAppClient{})
	svc.appBaseURL = "https://stage.helpin.ai"
	run := newDelegatedDeliveryTestRun()
	run.OutputSummary = json.RawMessage(`{
		"repository": {
			"pull_request": {
				"summary": "Corrects delivery state synchronization.",
				"changes": ["Updates the merge status after delivery."],
				"validations": [{"name":"Go tests","command":"go test ./internal/service","result":"passed"}],
				"review_notes": ["Focus on the terminal status transition."],
				"risks": ["Low; scoped to delivery bookkeeping."],
				"unresolved_items": []
			}
		}
	}`)

	title, body := svc.buildDelegatedRunPullRequestContent(context.Background(), run, AgentRunRepositoryDelivery{
		AgentName: "Forge",
		CommitSHA: "abc1234",
	}, "hel-31-fix-merge-status", "main")

	if title != "HEL-31: Fix merge status" {
		t.Fatalf("title = %q", title)
	}
	for _, expected := range []string{
		prcontent.StartMarker,
		"## Summary",
		"Corrects delivery state synchronization.",
		"- Updates the merge status after delivery.",
		"## Validation",
		"✅ Go tests — `go test ./internal/service` (passed)",
		"## Review notes",
		"Focus on the terminal status transition.",
		"**Risk:** Low; scoped to delivery bookkeeping.",
		"[HEL-31 — Fix merge status](https://stage.helpin.ai/w/demo%20workspace/pm/tasks/task-1)",
		"[Forge delivery run](https://stage.helpin.ai/w/demo%20workspace/automation/activity?run_id=run-delivery-1)",
		"- Commit: `abc1234`",
		"Created by Helpin agent **Forge**.",
		prcontent.EndMarker,
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("body missing %q:\n%s", expected, body)
		}
	}
	if strings.Contains(body, "Run ID:") {
		t.Fatalf("body exposes a raw run ID:\n%s", body)
	}
}

func TestBuildDelegatedRunPullRequestContentLegacySummaryStillLinksStory(t *testing.T) {
	db := newTestDB(t)
	seedGitDeliveryStatusFixture(t, db)
	now := time.Now().UTC()
	mustExec(t, db, `INSERT INTO workspaces (
		id, name, slug, workspace_key, owner_id, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"ws-1", "Demo Workspace", "demo", "HEL", "user-1", now, now)

	svc := newGitDeliveryStatusService(db, nil)
	svc.appBaseURL = "https://helpin.ai"
	run := newDelegatedDeliveryTestRun()
	run.OutputSummary = json.RawMessage(`{"repository":{"changed":true,"pushed":true}}`)

	_, body := svc.buildDelegatedRunPullRequestContent(context.Background(), run, AgentRunRepositoryDelivery{}, "hel-31", "main")
	if !strings.Contains(body, "Implements [HEL-31 — Fix merge status](https://helpin.ai/w/demo/pm/tasks/task-1).") {
		t.Fatalf("legacy body should link the story:\n%s", body)
	}
	if strings.Contains(body, "## Validation") || strings.Contains(body, "passed") {
		t.Fatalf("legacy body must not fabricate validation:\n%s", body)
	}
}

func TestDelegatedRunPullRequestDetailsPrefersStructuredRepositoryMetadata(t *testing.T) {
	details := delegatedRunPullRequestDetailsFromSummary(json.RawMessage(`{
		"summary":"fallback",
		"repository":{"pull_request":{"summary":"preferred","validations":["Manual smoke test"]}}
	}`))
	if details.Summary != "preferred" {
		t.Fatalf("summary = %q", details.Summary)
	}
	if len(details.Validations) != 1 || details.Validations[0].Name != "Manual smoke test" {
		t.Fatalf("validations = %#v", details.Validations)
	}
}
