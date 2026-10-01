import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { FlowBuilderPreview } from "../FlowBuilderPreview";
import type { FlowBuilderState } from "@/lib/flowBuilderTypes";

const state: FlowBuilderState = {
  revision: "v1",
  draft: {
    name: "Review changes",
    trigger_type: "github.pull_request_opened",
    trigger_config: {
      repo_full_name: "example/app",
      base_branch: "main",
      semantic_condition: { text: "The change concerns authentication" },
    },
    action_type: "start_agent_run",
    action_config: {
      agent_id: "reviewer",
      target_type: "repository",
      target_id: "repo",
    },
  },
};
describe("FlowBuilderPreview", () => {
  it("shows exact and semantic conditions without invented scope", () => {
    const html = renderToStaticMarkup(
      <FlowBuilderPreview
        state={state}
        agents={[{ id: "reviewer", name: "Code Reviewer" }]}
        workflows={[]}
      />,
    );
    expect(html).toContain("main");
    expect(html).toContain("the change concerns authentication");
    expect(html).toContain("It runs only when");
    expect(html).toContain("<strong>Code Reviewer</strong>");
    expect(html).not.toContain(">When<");
    expect(html).not.toContain(">Then<");
    expect(html).not.toContain("Engineering");
    expect(html).not.toContain("Advanced");
  });
  it("does not show a separate create button", () => {
    const html = renderToStaticMarkup(
      <FlowBuilderPreview state={state} agents={[]} workflows={[]} />,
    );
    expect(html).not.toContain("Create flow");
  });
});

describe("complete flow review", () => {
  it("describes consequential template agent settings and the flow description", () => {
    const review = {
      ...state,
      creates_agent: true,
      template_key: "release_notes_writer",
      draft: {
        ...state.draft!,
        description: "Prepare **customer-facing release notes**.",
        agent_overrides: {
          approval_mode: "never" as const,
          max_concurrent_runs: 3,
          allowed_targets: ["repository" as const],
          allowed_tools: ["read_file"],
          skills: [{ key: "release_notes_writing" }],
          system_prompt: "Write **clear** release notes.",
        },
      },
    };
    const html = renderToStaticMarkup(
      <FlowBuilderPreview state={review} agents={[]} workflows={[]} />,
    );
    expect(html).toContain("<strong>customer-facing release notes</strong>");
    expect(html).toContain("without approval");
    expect(html).toContain("3");
    expect(html).toContain("read_file");
    expect(html).toContain("release_notes_writing");
    expect(html).toContain("<strong>clear</strong>");
  });
  it("shows the actual next execution in the selected timezone", () => {
    const review = {
      ...state,
      timezone: "Asia/Kolkata",
      next_runs: ["2026-10-02T09:30:00+05:30"],
      draft: {
        ...state.draft!,
        trigger_type: "cron",
        trigger_config: { schedule: "0 4 * * 1-5" },
      },
    };
    const html = renderToStaticMarkup(
      <FlowBuilderPreview state={review} agents={[]} workflows={[]} />,
    );
    expect(html).toContain("Asia/Kolkata");
    expect(html).toContain("09:30");
  });
  it("does not claim a picked existing agent will be created", () => {
    const review = {
      ...state,
      template_key: "run_on_release",
      creates_agent: false,
      agent: { name: "Existing", is_system: false },
    } as FlowBuilderState;
    const html = renderToStaticMarkup(
      <FlowBuilderPreview state={review} agents={[]} workflows={[]} />,
    );
    expect(html).not.toContain("also creates an agent");
  });
});

it("shows user instructions without exposing generated template configuration", () => {
  const review: FlowBuilderState = {
    ...state,
    template_key: "review_merged_prs",
    draft: {
      ...state.draft!,
      template_inputs: { additional_instructions: "Focus on **security**." },
      action_config: {
        ...state.draft!.action_config,
        additional_context:
          "Internal template configuration: repository_id=repo",
      },
    },
  };
  const html = renderToStaticMarkup(
    <FlowBuilderPreview state={review} agents={[]} workflows={[]} />,
  );
  expect(html).toContain("<strong>security</strong>");
  expect(html).not.toContain("Internal template configuration");
});
