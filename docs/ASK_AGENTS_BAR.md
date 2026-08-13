# The Dock (Ask Agents)

> The in-app command-bar chat layer this document used to describe (LLM
> classifier, inline read-only tool loop, plan proposals, `command_bar_threads`
> / `command_bar_messages`) was removed. Each dock chat is now an
> **agent-runtime chat-mode run**. This page is the current reference.

## Architecture

Each user has multiple **dock chats** per workspace (`dock_chats` table). A
chat is backed by an agent-runtime run of the **`ask_agent`** system preset
(`native_sdk`, `turn_policy.mode = pause_after_assistant`, 72h idle timeout):

```text
user message
  -> POST /api/dock/chats/{id}/messages
  -> DockChatService.SendMessage
       first message: start run (target=workspace, dock_chat_id stamped)
       otherwise:     resume run (intent=reply)
       run ended:     start successor run with carry-forward context
  -> agent-runtime executes the turn (NATS projection mirrors transcript)
  -> run pauses awaiting_user_message
```

The Ask Agent is the Dock's primary execution agent. Its product tools are
scoped per user at run start, and the command bridge re-checks permissions on
every call. It handles reads, planning, ordinary approved mutations, and
read-only repository inspection itself. It delegates only when work benefits
from parallelism, a long-running background run, specialist isolation, or
repository modification.

| Tool | Purpose |
|------|---------|
| `get_my_capabilities` | inspect the current Dock run's actual granted tools before deciding whether delegation is necessary |
| `list_agents` | discover saved agents (by id) |
| `get_agent_capabilities` | inspect a saved agent's complete tools, targets, skills, and runtime before launch |
| `prepare_dock_execution` | persist an immutable, bounded proposal for ordinary product mutations |
| `activate_dock_execution` / `finish_dock_execution` | activate an approved short-lived grant and close it after execution |
| repository read tools | check out the default branch and read/search files, symbols, and commit history directly |
| `start_agent_run` / `start_agent_plan` | selectively launch sub-agent runs / multi-step plans (reuses command-bar dispatch validation) |
| `update_task_delivery_target` | assign the repository and optional branch a task-targeted repository agent needs before launch |
| `get_agent_run`, `cancel_agent_run` | child run status / cancellation |
| `draft_custom_agent`, `create_custom_agent` | generate a complete durable agent draft, approve its ID, then create it without reconstructing capabilities |
| `promote_run_to_agent` | turn a proven run into a reusable agent |

**Ordinary mutation contract:** Ask Agent calls `prepare_dock_execution` with
the exact tool aliases, target constraints, maximum calls, and expected
outcomes. The resulting proposal is durable and immutable. Approval contains
only `{phase: "dock_execution_confirm", action: {proposal_id}}`; after approval,
`activate_dock_execution` issues a 15-minute grant. The central internal-command
bridge rejects tools, inputs, or call counts outside that grant. Agent, Git,
delivery/release, support-reply, and escalation mutations cannot use this path.

For recovery from an early single mutation call, the central guard creates or
reuses an exact one-call proposal and returns a compact structured
`dock_execution_approval_required` error containing the `request_approval`
payload. After that approval resolves, retrying the identical mutation lazily
activates and consumes the proposal. This keeps the intuitive
mutation → approval → retry sequence safe without repeating research or
embedding large document content in another model-generated proposal. Explicit
prepare/activate remains the preferred path for multi-operation batches.

**Sub-agent approval contract:** a launch approval stores the complete
`dock_plan_confirm` action, including instructions and optional tool overrides.
After approval, `start_agent_run` or `start_agent_plan` accepts only the
`approval_interaction_id`; the server loads and dispatches the stored action.
The model therefore cannot lose or subtly change a long instruction payload
between approval and execution. Approvals are single-use. For saved agents,
omitting `allowed_tools` preserves the agent's configured defaults.

Every direct launch step has an explicit target. Saved preset agents and
one-shot Sub-agents use the same target contract and target allowlist checks.
Entity work stays entity-targeted: after creating a task, the Dock launches a
task planner with that task's ID rather than falling back to workspace. If a
repository-capable agent reports that the task lacks a delivery target, the
Dock selects an unambiguous connected repository, assigns it with
`update_task_delivery_target`, and retries the same task target once. It asks
the user when several repositories remain plausible and never creates or
attaches an epic as a repository-configuration workaround.

**Reusable-agent contract:** `draft_custom_agent` uses the workspace tool and
skill catalogs to generate the complete prompt, tools, targets, skills, and
runtime configuration before approval, then persists that draft as an
`agent_draft` proposal. Approval and `create_custom_agent` pass only the
proposal/interaction IDs. The created agent therefore matches the reviewed
draft, rather than being re-generated after approval.

**Capability-driven delegation:** the Dock treats a multi-domain request as
one task whenever its current tools cover every step. It can inspect its actual
run grants with `get_my_capabilities` and load applicable guidance with
`find_skills` and `read_skill`. It
delegates only the smallest portion requiring a missing or intentionally
isolated capability. Repository writes, code implementation/validation, and
specialist code review normally go to coding/review agents; read-only
investigation, synthesis, planning, and supported product mutations stay in
the Dock.

The backing Dock run remains workspace-targeted for its lifetime. Selected
page context is conversational context, not a runtime retarget operation.
Commands with explicit IDs—such as `document_id` for Docs reads and
mutations—use that ID as the action target and validate workspace ownership
server-side. The Dock must not loop on target resolution or fall back to a
proposal merely because a selected document is being edited from the
workspace-targeted chat.

For complex or long work, the Dock maintains its own plan with `update_plan`.
The current plan is projected through the run stream and rendered inline in
the chat using the same plan component as the agent run sheet, labeled “Work
plan.” This plan describes the Dock's execution progress; it does not create
sub-agents or authorize mutations.

**Child results:** plans launched from a chat carry
`command_bar_plans.parent_chat_run_id` / `dock_chat_id`. When a plan settles,
helpin resumes the parent chat run with a `<child_run_result>` block
(`internal/service/dock_chat_results.go`; immediate via the terminal
finalizer, 30s sweep as backstop; ended chats get results via successor-run
carry-forward). Each child is instructed to end with a self-contained handoff
of at most 2,500 characters. Delivery includes up to 3,000 Unicode characters
plus `summary_truncated`, `summary_char_count`, `result_available`, and compact
artifact references. Run reports are emitted in plan-step order.

When `summary_truncated` is true, Ask Agent retrieves the persisted response
from the same run with
`get_agent_run({run_id, detail_level: "result", result_offset?, result_limit?})`.
Result reads are bounded and paginated (`next_offset`) and are authorized only
for runs belonging to a plan launched from the current dock chat. Truncation is
never a reason to launch replacement work; a new child requires a genuine
failed/incomplete result and the normal user approval.

If a sub-agent returns useful research or drafted content but lacked an
ordinary product mutation tool, the Dock can continue from that handoff using
its own scoped execution proposal. It does not need to relaunch the child just
to create or update the final document, task, or CRM record.

## Where the code lives

| Layer | Path |
|-------|------|
| FE shell | `frontend/src/components/agents/AskAgentsDock.tsx` (+ `dock/ChatView.tsx`, `dock/ChatListView.tsx`, `dock/DockPlanConfirmCard.tsx`, `dock/dockChatState.ts`) |
| FE service / store | `frontend/src/lib/services/dockChatService.ts`, `frontend/src/stores/dockStore.ts` |
| API | `/api/dock/chats*` (`server/internal/handler/dock_chat.go`, router `/dock` block) |
| Service | `server/internal/service/dock_chat.go`, `dock_chat_results.go` |
| Orchestration tools | `server/internal/service/internal_command_agents.go` |
| Direct-execution grants | `server/internal/service/internal_command_dock_execution.go`, `server/internal/model/dock_action_proposal.go` |
| Preset | `ask_agent` in `server/internal/service/agent_presets.go` |
| Plan dispatch (kept) | `server/internal/service/command_bar_dispatch.go`, `command_bar_plans.go`, `command_bar_orchestration_steps.go` |

## Invariants

- The Dock performs ordinary product mutations directly only through an
  immutable `dock_execution_confirm` proposal and a bounded, expiring grant.
- Repository tools in the Dock are read-only. Branch changes, file writes,
  commits, pushes, merges, and pull requests remain sub-agent work.
- Sub-agents are selective, not the default execution path. Saved-agent
  launches inherit configured tools unless an explicit override is approved.
- Chats are per-user (`dock_chats.user_id` ownership on every endpoint); run
  reads are proxied through `/api/dock/chats/{id}/run*` so users without PM
  permissions can use their own dock.
- Chat-run tool access is narrowed at start time to the requesting user's
  module permissions, and the command bridge re-checks the actor's role on
  every command execution.
- One active backing run per chat; successor runs (after idle expiry or
  terminal states) carry forward recent transcript + undelivered child results
  and keep `agent_runs.dock_chat_id` pointing at the same chat.
- A paused backing run is automatically rotated when its frozen allowed-tool
  set no longer matches the current Ask Agent preset or the user's scoped
  permissions. This makes existing chats gain newly added capabilities and
  lose revoked ones without waiting for idle expiry.
- Core Ask Agent self-execution tools are managed capabilities. Reconciliation
  unions them into older workspace-pinned Ask preset versions before applying
  per-user permission scoping, preventing pinned snapshots from regressing the
  Dock to metadata-only repository access or delegation-only execution.
- Ask Agent's primary runtime target remains the product `workspace`; managed
  reconciliation removes stale `workspace.mode=repository` execution settings.
  Repository context is attached dynamically by `checkout_repositories`, so a
  repository read followed by a product mutation stays within the same run
  without asking the repository-spec provider to resolve a product workspace.
- When Agent Runtime revalidates a dynamically checked-out lease after an
  interactive approval pause, the repository-spec adapter reconstructs the
  repository target from the runtime's explicit primary-checkout metadata.
  The run's product target remains `workspace` for document/PM/CRM commands.
