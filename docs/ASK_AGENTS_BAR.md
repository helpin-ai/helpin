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

The Ask Agent answers read-only questions directly with product command tools
(scoped per-user by module permissions at run start) and orchestrates durable
work through the `agents.*` command tools:

| Tool | Purpose |
|------|---------|
| `list_agents` | discover saved agents (by id) |
| `start_agent_run` / `start_agent_plan` | launch child runs / multi-step plans (reuses command-bar dispatch validation) |
| `get_agent_run`, `cancel_agent_run` | child run status / cancellation |
| `create_custom_agent`, `promote_run_to_agent` | create reusable agents |

**Approval contract:** mutating `agents.*` calls require a resolved
`request_approval` interaction with payload
`{kind: "dock_plan_confirm", summary, action}` where `action` exactly matches
the tool input. The server verifies this by canonical hash
(`internal/service/internal_command_agents.go`) — approvals are single-use.

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

## Where the code lives

| Layer | Path |
|-------|------|
| FE shell | `frontend/src/components/agents/AskAgentsDock.tsx` (+ `dock/ChatView.tsx`, `dock/ChatListView.tsx`, `dock/DockPlanConfirmCard.tsx`, `dock/dockChatState.ts`) |
| FE service / store | `frontend/src/lib/services/dockChatService.ts`, `frontend/src/stores/dockStore.ts` |
| API | `/api/dock/chats*` (`server/internal/handler/dock_chat.go`, router `/dock` block) |
| Service | `server/internal/service/dock_chat.go`, `dock_chat_results.go` |
| Orchestration tools | `server/internal/service/internal_command_agents.go` |
| Preset | `ask_agent` in `server/internal/service/agent_presets.go` |
| Plan dispatch (kept) | `server/internal/service/command_bar_dispatch.go`, `command_bar_plans.go`, `command_bar_orchestration_steps.go` |

## Invariants

- The chat agent never mutates product data directly — child runs do, behind
  the server-enforced `dock_plan_confirm` approval.
- Chats are per-user (`dock_chats.user_id` ownership on every endpoint); run
  reads are proxied through `/api/dock/chats/{id}/run*` so users without PM
  permissions can use their own dock.
- Chat-run tool access is narrowed at start time to the requesting user's
  module permissions, and the command bridge re-checks the actor's role on
  every command execution.
- One active backing run per chat; successor runs (after idle expiry or
  terminal states) carry forward recent transcript + undelivered child results
  and keep `agent_runs.dock_chat_id` pointing at the same chat.
