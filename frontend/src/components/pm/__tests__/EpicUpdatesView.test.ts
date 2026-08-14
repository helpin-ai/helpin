import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

import { buildEpicUpdateEntries } from "@/components/pm/epic-detail/EpicUpdatesView";
import {
  epicActivityPresentation,
  filterRedundantEpicActivity,
} from "@/components/pm/epic-detail/epicUpdateEventLabel";
import type { ActivityLogEntry, CommentWithAuthor } from "@/lib/pmTypes";

const __dirname = dirname(fileURLToPath(import.meta.url));

function comment(id: string, createdAt: string): CommentWithAuthor {
  return {
    comment: {
      id,
      entity_type: "epic",
      entity_id: "epic-1",
      author_id: "user-1",
      body: id,
      created_at: createdAt,
      updated_at: createdAt,
    },
    author: {
      id: "user-1",
      email: "user@example.com",
      full_name: "User",
      created_at: createdAt,
      updated_at: createdAt,
    },
  };
}

function activity(
  id: string,
  action: string,
  createdAt: string,
  overrides: Partial<ActivityLogEntry["activity"]> = {},
): ActivityLogEntry {
  return {
    activity: {
      id,
      workspace_id: "workspace-1",
      entity_type: "epic",
      entity_id: "epic-1",
      action,
      created_at: createdAt,
      ...overrides,
    },
  };
}

describe("Epic updates view", () => {
  const comments = [
    comment("comment-new", "2026-08-14T12:00:00Z"),
    comment("comment-old", "2026-08-14T10:00:00Z"),
  ];
  const activityEntries = [
    activity("change", "updated", "2026-08-14T13:00:00Z", {
      field_name: "name",
      old_value: "Old title",
      new_value: "New title",
    }),
    activity("comment-activity", "comment_added", "2026-08-14T14:00:00Z"),
  ];

  it("merges comments and changes newest first without duplicate comment activity", () => {
    expect(
      buildEpicUpdateEntries(comments, activityEntries, "all").map(
        (entry) => entry.id,
      ),
    ).toEqual([
      "activity:change",
      "comment:comment-new",
      "comment:comment-old",
    ]);
  });

  it("supports discussion-only and changes-only filters", () => {
    expect(
      buildEpicUpdateEntries(comments, activityEntries, "discussion").map(
        (entry) => entry.kind,
      ),
    ).toEqual(["comment", "comment"]);
    expect(
      buildEpicUpdateEntries(comments, activityEntries, "changes").map(
        (entry) => entry.id,
      ),
    ).toEqual(["activity:change"]);
  });

  it("hides legacy fieldless updates while retaining identifiable activity", () => {
    const entries = [
      activity("empty", "updated", "2026-08-14T13:00:00Z"),
      activity("state", "updated", "2026-08-14T12:00:00Z", {
        field_name: "epic_state_id",
        old_value: "state-1",
        new_value: "state-2",
        metadata: { old_label: "To Do", new_label: "In Progress" },
      }),
    ];

    expect(buildEpicUpdateEntries([], entries, "changes").map((entry) => entry.id)).toEqual([
      "activity:state",
    ]);
  });

  it("describes the exact Epic field change without exposing description content", () => {
    const stateChange = activity("state", "updated", "2026-08-14T12:00:00Z", {
      field_name: "epic_state_id",
      old_value: "state-1",
      new_value: "state-2",
      metadata: { old_label: "To Do", new_label: "In Progress" },
    });
    stateChange.actor = {
      id: "user-1",
      email: "amad@example.com",
      full_name: "Amad",
      created_at: "2026-01-01T00:00:00Z",
      updated_at: "2026-01-01T00:00:00Z",
    };
    const presentation = epicActivityPresentation(stateChange);
    expect(presentation.title).toBe(
      "Amad moved this epic from To Do to In Progress",
    );
    expect(presentation.emphasizedValues).toEqual(["Amad", "To Do", "In Progress"]);

    const descriptionChange = activity("description", "updated", "2026-08-14T12:00:00Z", {
      field_name: "description",
    });
    expect(epicActivityPresentation(descriptionChange).title).toBe(
      "System updated the description",
    );
  });

  it("deduplicates matching terminal agent activity but preserves distinct runs", () => {
    const terminal = (id: string, runId: string) => activity(id, "updated", "2026-08-14T12:00:00Z", {
      field_name: "agent_run",
      new_value: "completed",
      metadata: { run_id: runId, agent_name: "Forge" },
    });
    const first = terminal("first", "run-1");
    const duplicate = terminal("duplicate", "run-1");
    const otherRun = terminal("other", "run-2");

    expect(filterRedundantEpicActivity([first, duplicate, otherRun]).map((entry) => entry.activity.id)).toEqual([
      "first",
      "other",
    ]);
  });

  it("humanizes legacy agent starts and approved specification versions", () => {
    const legacyRun = activity("run", "updated", "2026-08-14T12:00:00Z", {
      field_name: "agent_run",
      new_value: "started",
    });
    legacyRun.actor = {
      id: "user-1",
      email: "amad@example.com",
      full_name: "Amad",
      created_at: "2026-01-01T00:00:00Z",
      updated_at: "2026-01-01T00:00:00Z",
    };
    expect(epicActivityPresentation(legacyRun).title).toBe("Amad started the agent run");

    const approvedSpec = activity("spec", "updated", "2026-08-14T12:00:00Z", {
      field_name: "approved_spec_version_id",
      new_value: "raw-version-id",
    });
    expect(epicActivityPresentation(approvedSpec).title).toBe(
      "System approved a new specification version",
    );
  });

  it("keeps Overview and Updates together while isolating delivery and planner content", () => {
    const detailSource = readFileSync(
      resolve(__dirname, "../../../pages/pm/EpicDetail.tsx"),
      "utf8",
    );
    const routeSource = readFileSync(
      resolve(
        __dirname,
        "../../../routes/_authenticated/w/$slug/pm/epics/$epicId.tsx",
      ),
      "utf8",
    );

    expect(detailSource).toContain("(['overview', 'delivery'] as const)");
    expect(detailSource).toContain("<EpicUpdatesView");
    expect(detailSource).not.toContain("setTasksView");
    expect(routeSource).toContain("epic_view?: 'delivery'");

    const tasksIndex = detailSource.indexOf(
      "title={`Tasks (${tasks.length})`}",
    );
    const updatesIndex = detailSource.indexOf('title="Updates"');
    const pipelineIndex = detailSource.indexOf('title="Delivery pipeline"');
    const plannerIndex = detailSource.indexOf(
      "<EpicPlannerPanel",
      pipelineIndex,
    );
    expect(tasksIndex).toBeGreaterThan(-1);
    expect(updatesIndex).toBeGreaterThan(tasksIndex);
    expect(pipelineIndex).toBeGreaterThan(updatesIndex);
    expect(plannerIndex).toBeGreaterThan(pipelineIndex);
  });

  it("uses a borderless task empty state with two ghost actions", () => {
    const detailSource = readFileSync(
      resolve(__dirname, "../../../pages/pm/EpicDetail.tsx"),
      "utf8",
    );

    expect(detailSource).toContain('meta={tasks.length > 0 ? (');
    expect(detailSource).toContain('text-sm italic text-muted-foreground">No tasks linked yet.');
    expect(detailSource).toContain('<span>Link existing tasks</span>');
    expect(detailSource).toContain('<span>Create task</span>');
    expect(detailSource).toContain('const renderEmptyTaskActions = () => (');
    expect(detailSource).toContain('variant="ghost"');
    expect(detailSource).toContain('size="sm"');
    expect(detailSource).not.toContain('h-7 gap-1.5 px-2 text-xs text-muted-foreground');
  });
});
