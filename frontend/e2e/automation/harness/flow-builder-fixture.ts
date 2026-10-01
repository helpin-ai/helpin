// Browser-only fixture. All application API requests stay in memory.
import type { FlowBuilderState } from "@/lib/flowBuilderTypes";
import type { AgentRunMessage, CodingSessionEvent } from "@/lib/pmTypes";
const now = () => new Date().toISOString();
export const fixtureFlow = {
  id: "sample-flow",
  workspace_id: "preview-workspace",
  name: "Review pull requests",
  enabled: true,
  trigger_type: "github.pull_request_opened",
  trigger_config: { repo_full_name: "acme/web-app", base_branch: "main" },
  action_type: "start_agent_run",
  action_config: {
    agent_id: "reviewer",
    target_type: "repository",
    target_id: "repo",
  },
  created_at: now(),
  updated_at: now(),
};
export function installFlowBuilderFixture() {
  const nativeFetch = window.fetch.bind(window);
  let serial = 0;
  const chats = new Map<
    string,
    {
      chat: Record<string, unknown>;
      state: FlowBuilderState;
      messages: AgentRunMessage[];
      events: CodingSessionEvent[];
      pause: string;
      turn: number;
      editing: boolean;
      runId: string;
    }
  >();
  window.fetch = async (input, init) => {
    const url = new URL(
      typeof input === "string"
        ? input
        : input instanceof URL
          ? input.href
          : input.url,
      location.href,
    );
    if (!url.pathname.includes("/api/")) return nativeFetch(input, init);
    const path = url.pathname.replace(/^.*\/api/, "");
    const body = init?.body ? JSON.parse(String(init.body)) : {};
    const send = (value: unknown) =>
      Promise.resolve(
        new Response(JSON.stringify(value), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        }),
      );
    if (path === "/dock/ai-defaults") return send({ ai_profile_id: "default" });
    if (path.startsWith("/ai-profiles"))
      return send([
        {
          id: "default",
          name: "Workspace AI",
          scope: "workspace",
          primary: {
            connection_id: "sample",
            model: {
              provider: "anthropic",
              model: "Workspace default",
              controls: {},
            },
          },
        },
      ]);
    if (path.startsWith("/ai-settings"))
      return send({ default_profile_id: "default" });
    if (path === "/dock/chats" && init?.method === "POST") {
      const id = `preview-chat-${++serial}`;
      const state: FlowBuilderState = {
        labels: { repo: "acme/web-app", reviewer: "Code Reviewer" },
        ...(body.flow_id
          ? { source_rule: fixtureFlow as never, draft: { ...fixtureFlow } }
          : {}),
        ...(body.flow_template_key
          ? { template_key: body.flow_template_key }
          : {}),
      };
      const chat = {
        id,
        workspace_id: "preview-workspace",
        user_id: "preview-user",
        title: "Flow builder",
        visibility: "private",
        created_at: now(),
        updated_at: now(),
        flow_builder: state,
      };
      chats.set(id, {
        chat,
        state,
        messages: [],
        events: [],
        pause: "awaiting_user_message",
        turn: 0,
        editing: !!body.flow_id,
        runId: `run-${serial}`,
      });
      return send(chat);
    }
    const id = path.match(/\/dock\/chats\/([^/]+)/)?.[1];
    const c = id ? chats.get(id) : undefined;
    if (!c) return send([]);
    const run = () => ({
      id: c.runId,
      run_id: c.runId,
      workspace_id: "preview-workspace",
      dock_chat_id: id,
      agent_id: "ask-agent",
      target_type: "workspace",
      target_id: "preview-workspace",
      runtime_kind: "native_sdk",
      invocation_mode: "interactive",
      status: "paused",
      pause_reason: c.pause,
      created_at: now(),
      updated_at: now(),
      input: {},
      tokens_used: 0,
    });
    const detail = () => ({
      chat: c.chat,
      run: c.turn ? run() : null,
      plan_ids: [],
      artifacts: [],
    });
    const message = (role: string, content: string, clientId?: string) => {
      const seq = c.messages.length + 1;
      const value: AgentRunMessage = {
        id: `${id}-message-${seq}`,
        workspace_id: "preview-workspace",
        run_id: c.runId,
        dock_chat_id: id,
        dock_chat_sequence: seq,
        sequence_no: seq,
        client_message_id: clientId,
        delivery_status: "sent",
        actor_user_id: role === "user" ? "preview-user" : undefined,
        role,
        content,
        message_type: role === "user" ? "prompt" : "assistant_final",
        created_at: now(),
      };
      c.messages.push(value);
      return value;
    };
    const event = (type: string, payload: Record<string, unknown>) =>
      c.events.push({
        id: `event-${c.events.length + 1}`,
        session_id: c.runId,
        run_id: c.runId,
        sequence_no: c.events.length + 1,
        timestamp: now(),
        type,
        runtime_kind: "native_sdk",
        payload,
      });
    const propose = (change: string) => {
      c.state.draft ??= { ...fixtureFlow };
      if (/only if|authentication/i.test(change))
        c.state.draft.trigger_config = {
          ...c.state.draft.trigger_config,
          semantic_condition: {
            text: "The pull request concerns authentication",
          },
        };
      if (/remove.*condition/i.test(change))
        c.state.draft.trigger_config = { repo_full_name: "acme/web-app" };
      if (/develop/i.test(change))
        c.state.draft.trigger_config = {
          ...c.state.draft.trigger_config,
          base_branch: "develop",
        };
      c.state.revision = `revision-${c.turn}`;
      message(
        "assistant",
        c.editing
          ? "Here’s the updated flow. Everything else stays the same."
          : "Here’s the flow I’ll create.",
      );
      c.pause = "human_approval";
      event("interaction.requested", {
        interaction_id: `approval-${c.turn}`,
        interaction_kind: "approval_request",
        status: "pending",
        request_schema_version: "helpin.v1",
        title: c.editing ? "Save these changes?" : "Create this flow?",
        request_payload: {
          phase: "flow_confirm",
          action: { revision: c.state.revision },
        },
      });
    };
    if (path.endsWith("/messages") && init?.method === "POST") {
      c.turn++;
      c.chat.active_run_id = c.runId;
      const accepted = message("user", body.content, body.client_message_id);
      if (c.turn === 1 && c.editing)
        message(
          "assistant",
          "This flow runs **Code Reviewer** when **a pull request opens** in **acme/web-app** targeting **main**. It is currently **active**.\n\nWhat would you like to change?",
        );
      else if (c.turn === 1)
        message("assistant", "Which repository should this flow use?");
      else propose(body.content);
      return send({ ...detail(), accepted_message: accepted });
    }
    if (path.endsWith("/messages"))
      return send({ messages: c.messages, next_before: null });
    if (path.endsWith("/title")) return send(c.chat);
    if (path.endsWith("/run/events"))
      return send({ events: c.events, next_sequence_no: c.events.length });
    if (path.endsWith("/resolve")) {
      const pending = c.events
        .filter((e) => e.type === "interaction.requested")
        .at(-1)!;
      const payload = {
        ...pending.payload,
        status: "resolved",
        response_payload: body.response_payload,
      };
      event("interaction.resolved", payload);
      c.pause = "awaiting_user_message";
      if (body.response_payload.decision === "approve") {
        c.state.created_flow_id = c.editing
          ? "sample-flow"
          : `new-flow-${serial}`;
        message(
          "assistant",
          c.editing
            ? "Flow updated."
            : "Flow created. It’s active and ready for the next pull request.",
        );
      } else {
        c.turn++;
        message("user", body.followup_message || "Make changes");
        propose(body.followup_message || "");
      }
      return send(payload);
    }
    if (path.endsWith("/run"))
      return send({
        ...run(),
        title: "Flow builder",
        approval_state: "none",
        capabilities: {},
        repo: { files: [] },
        input_tokens: 0,
        output_tokens: 0,
        cached_input_tokens: 0,
      });
    return send(detail());
  };
}
