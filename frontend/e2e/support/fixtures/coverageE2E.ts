import { coverageFindings } from "../../../src/components/support/coverage/coverageFindings";
import type { SupportCoverageGapDetail } from "../../../src/lib/supportCoverageTypes";
import type { Page } from "@playwright/test";
import { installSupportAppMocks, WORKSPACE_ID } from "./supportE2E";

export const COVERAGE_GAP = {
  id: "gap-1",
  workspace_id: WORKSPACE_ID,
  title: "Customers cannot find the invoice correction process",
  canonical_title: "",
  topic_title: "",
  status: "open",
  gap_kind: "content",
  v1_gap_type: "missing_article",
  issue_key: "invoice_correction",
  failure_mode: "missing_content",
  confidence: 0.85,
  evidence_count: 9,
  evidence_all: 9,
  evidence_records_30d: 7,
  evidence_30d: 3,
  conversations_30d: 3,
  conversations_all: 4,
  distinct_customers_30d: 2,
  distinct_customers_all: 3,
  impact_explanation: "3 conversations · 2 known customers in the last 30 days",
  first_seen_at: "2026-09-20T10:00:00Z",
  last_seen_at: "2026-09-30T10:00:00Z",
  analysis_explanation: {
    customer_need:
      "Customers need to correct the billing address on an issued invoice.",
    ai_failure:
      "The help center does not explain who can correct an issued invoice.",
    human_resolution:
      "The support team confirmed that billing reviews correction requests.",
    decision_reason:
      "The same question required a human answer in several conversations.",
  },
  evidence: [
    {
      id: "ev-1",
      gap_id: "gap-1",
      evidence_type: "daily_conversation_analysis",
      source_signal: "daily_conversation_analysis",
      conversation_id: "conv-1",
      message_id: null,
      document_id: null,
      excerpt: "How can I correct the billing address on my invoice?",
      sender_role: "",
      created_at: "2026-09-30T10:00:00Z",
    },
  ],
  recommendations: [
    {
      id: "rec-1",
      priority: "primary",
      status: "open",
      recommendation_type: "create_article",
      suggested_change:
        "Prepare invoice correction guidance using the support team’s confirmed process.",
      rationale: "The answer is currently available only from support.",
    },
  ],
  suggestions: [] as Record<string, unknown>[],
  related_articles: [{ id: "gap-article-1", gap_id: "gap-1", document_id: "doc-billing", article_title: "Billing overview" }],
};

export const COVERAGE_PROPOSAL = {
  id: "proposal-1",
  gap_id: "gap-1",
  suggestion_type: "create_article",
  status: "draft",
  is_active: true,
  superseded_at: null,
  title: "Correcting an invoice",
  evidence_summary: "The support team’s confirmed resolution.",
  target_space_id: "space-1",
  target_collection_id: null,
  target_document_id: null,
  result_document_id: null,
  content: {
    type: "doc",
    content: [
      {
        type: "heading",
        attrs: { level: 2 },
        content: [{ type: "text", text: "Request a correction" }],
      },
      {
        type: "paragraph",
        content: [
          {
            type: "text",
            text: "Contact support so the billing team can review your request.",
          },
        ],
      },
    ],
  },
  created_at: "2026-09-30T11:00:00Z",
};

export async function installCoverageMocks(
  page: Page,
  {
    proposal = false,
    readOnly = false,
    existingRun = false,
    runError = false,
    question = false,
    prepared = false,
  } = {},
) {
  await installSupportAppMocks(page);
  const gap = {
    ...COVERAGE_GAP,
    suggestions: proposal ? [{ ...COVERAGE_PROPOSAL }] : [],
  };
  const applied: Record<string, unknown>[] = [];
  let started = 0;
  const sent: Array<Record<string, unknown>> = [];
  const created: Array<Record<string, unknown>> = [];
  let chatCreated = existingRun;
  const answers: Record<string, unknown>[] = [];
  let resolved = false;
  const interaction = {
    interaction_id: "question-1",
    interaction_kind: "request_user_input",
    status: "pending",
    request_schema_version: "v1",
    request_payload: {
      questions: [
        {
          id: "owner",
          header: "Owner",
          question: "Who reviews invoice corrections?",
          options: [
            {
              label: "Billing team",
              description: "Billing reviews correction requests.",
            },
            {
              label: "Finance team",
              description: "Finance reviews correction requests.",
            },
          ],
          isOther: true,
        },
      ],
    },
  };
  const run = {
    id: "run-1",
    workspace_id: WORKSPACE_ID,
    target_type: "workspace",
    target_id: WORKSPACE_ID,
    dock_chat_id: "coverage-chat",
    agent_id: "ask-agent",
    status: question ? "paused" : "completed",
    pause_reason: question ? "human_input" : "none",
    created_at: new Date(Date.now() - 20_000).toISOString(),
    updated_at: new Date().toISOString(),
    input: {},
    output: { summary: "I found the missing invoice correction guidance." },
    output_summary: prepared
      ? {
          support_coverage_gap_outcome: {
            outcome: "review_ready",
            document_id: "doc-ready",
            proposal_id: "change-ready",
          },
        }
      : {},
  };
  const chat = {
    id: "coverage-chat",
    workspace_id: WORKSPACE_ID,
    user_id: "user-b",
    title: gap.title,
    visibility: "module",
    module_id: "support",
    coverage_gap_id: gap.id,
    active_run_id: "run-1",
    created_at: "2026-09-30T10:00:00Z",
    updated_at: "2026-09-30T11:00:00Z",
    initial_context: coverageFindings(gap as unknown as SupportCoverageGapDetail),
  };
  const answer = prepared
    ? "I prepared invoice correction guidance for review. [Review prepared fix](/w/workspace/docs/documents/doc-ready?proposal=change-ready). The gap stays open until you verify the fix."
    : "Billing owns the invoice correction process. I can prepare an article for review using the [customer conversation](/w/workspace/support?conversation=conv-1).";
  const messages = () =>
    chatCreated && (existingRun || started)
      ? [
          {
            id: "user-msg",
            run_id: run.id,
            workspace_id: WORKSPACE_ID,
            dock_chat_id: chat.id,
            actor_user_id: "user-b",
            role: "user",
            content: sent.at(-1)?.content ?? "Help me fix this gap",
            sequence_no: 1,
            chat_sequence: 1,
            created_at: run.created_at,
            delivery_status: "sent",
            client_message_id: sent.at(-1)?.client_message_id,
          },
          {
            id: "answer-1",
            run_id: run.id,
            workspace_id: WORKSPACE_ID,
            dock_chat_id: chat.id,
            role: "assistant",
            content: answer,
            sequence_no: 2,
            chat_sequence: 2,
            created_at: run.updated_at,
            delivery_status: "sent",
          },
        ]
      : [];
  const detail = () => ({
    chat,
    run: chatCreated && (existingRun || started) ? run : null,
    plan_ids: [],
    plans: [],
  });
  await page.route("**/api/**", async (route) => {
    const path = new URL(route.request().url()).pathname;
    const method = route.request().method();
    const json = (value: unknown) => route.fulfill({ json: value });
    if (path === "/api/dock/ai-defaults")
      return json({ ai_profile_id: "workspace-default" });
    if (path === "/api/ai-profiles/" || path === "/api/ai-profiles")
      return json([
        {
          id: "workspace-default",
          workspace_id: WORKSPACE_ID,
          scope: "workspace",
          name: "Workspace default",
          revision: 1,
          primary: {
            connection_id: "connection-1",
            model: { provider: "openai", model: "gpt-5.4", controls: {} },
          },
          fallback: null,
        },
      ]);
    if (path === "/api/dock/runs")
      return json({ runs: [], attention_count: 0 });
    if (path === "/api/dock/chats" && method === "GET")
      return json({ chats: chatCreated && !runError ? [chat] : [] });
    if (path === "/api/dock/chats/coverage-gap") {
      if (runError)
        return route.fulfill({
          status: 503,
          json: { error: "Conversation unavailable" },
        });
      return json(chatCreated ? chat : null);
    }
    if (path === "/api/dock/chats" && method === "POST") {
      created.push(route.request().postDataJSON());
      chatCreated = true;
      return json(chat);
    }
    if (path === "/api/dock/chats/coverage-chat") return json(detail());
    if (path === "/api/dock/chats/coverage-chat/messages" && method === "GET")
      return json({ messages: messages(), next_before: null });
    if (
      path === "/api/dock/chats/coverage-chat/messages" &&
      method === "POST"
    ) {
      started++;
      sent.push(route.request().postDataJSON());
      return json({ ...detail(), accepted_message: messages()[0] });
    }
    if (path === `/api/workspaces/${WORKSPACE_ID}/me`)
      return json({
        workspace_id: WORKSPACE_ID,
        user_id: "user-b",
        role: readOnly ? "member" : "admin",
        modules: ["support", "docs", "pm"],
        permissions: [
          "workspace.read",
          "support.read",
          "docs.read",
          "pm.read",
          "settings.read",
          "ws.connect",
          ...(!readOnly
            ? ["support.edit", "docs.edit", "pm.edit", "settings.manage"]
            : []),
        ],
        team_memberships: [],
      });
    if (path === "/api/automation/agents")
      return json([
        {
          id: "quill",
          name: "Quill",
          is_system: true,
          preset_key: "documentation_agent",
          status: "active",
          is_active: true,
          allowed_targets: ["support_coverage_gap"],
          visibility: "workspace",
          agent_type: "llm",
        },
      ]);
    if (path === "/api/docs/spaces")
      return json([
        {
          id: "space-1",
          workspace_id: WORKSPACE_ID,
          name: "Help center",
          space_type: "external_capable",
          type: "external_capable",
          visibility: "workspace_wide",
        },
      ]);
    if (path === "/api/support/coverage/summary")
      return json({
        new_gaps_this_week: 1,
        top_recurring_gaps: 1,
        gaps_fixed_this_week: 0,
        total_open_gaps: 1,
        total_evidence_count: 9,
        handoffs_after_fixes: 0,
      });
    if (path === "/api/support/coverage/gaps")
      return json({
        items: [gap],
        total: 1,
        page: 1,
        per_page: 50,
        total_pages: 1,
      });
    if (path === "/api/support/coverage/gaps/gap-1") return json(gap);
    if (path === "/api/support/coverage/gaps/gap-1/merge-suggestions")
      return json([]);
    if (path === "/api/dock/chats/coverage-chat/run")
      return json({
        ...run,
        stream_state_snapshot: {
          messages: [],
          artifacts: [],
          activities: [],
          work_plans: [],
          last_seq: 0,
          pending_interactions: [],
        },
      });
    if (path === "/api/dock/chats/coverage-chat/run/events")
      return json({
        events: [
          {
            id: "answer-1",
            run_id: run.id,
            session_id: run.id,
            sequence_no: 1,
            timestamp: run.updated_at,
            runtime_kind: "native_sdk",
            type: "assistant.message.completed",
            payload: {
              message_id: "answer-1",
              role: "assistant",
              sequence_no: 1,
              content: answer,
            },
            runtime_metadata: { source: "agent_run_message" },
          },
        ],
        next_sequence_no: 1,
      });
    if (path === "/api/dock/chats/coverage-chat/run/interactions")
      return json({ interactions: question && !resolved ? [interaction] : [] });
    if (
      path === "/api/dock/chats/coverage-chat/interactions/question-1/resolve"
    ) {
      answers.push(route.request().postDataJSON());
      resolved = true;
      run.status = "running";
      run.pause_reason = "none";
      return json({ ...interaction, status: "resolved" });
    }
    if (path === "/api/support/coverage/suggestions/proposal-1/apply") {
      applied.push(route.request().postDataJSON());
      gap.suggestions = [
        {
          ...COVERAGE_PROPOSAL,
          ...applied[0],
          status: "applied",
          result_document_id: "saved-doc",
        },
      ];
      return json({ status: "ok" });
    }
    return route.fallback();
  });
  return { gap, applied, answers, sent, created, started: () => started };
}
