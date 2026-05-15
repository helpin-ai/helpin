# Ask Agents Orchestration Chat Plan

## Summary

Ask Agents becomes a catalog-aware orchestration chat. It can answer simple read-only questions inline, propose durable one-shot runs, orchestrate existing agents, and draft reusable custom agents. The UX is one chat surface; the architecture keeps existing primitives: read-only inline answers use non-mutating tools, durable work creates normal `agent_run` / `command_bar_plan` records, and reusable agents are created only after explicit approval.

Implementation status: the first production slice is implemented. It adds durable chat threads/messages, chat-turn proposals, most-recent thread hydration, bounded inline read-only answers for Helpin guidance plus task/docs/CRM list/count context, normal command-bar plan dispatch, and approved custom-agent creation/create-and-run.

## Behavior Model

- Inline answer: for simple read-only questions about Helpin settings, docs, CRM, PM, support, or workspace data. Ask Agents uses only non-mutating tools and replies directly in chat without creating an agent run.
- One-shot Command Agent run: for ad hoc work that is long-running, cross-domain, approval-gated, mutation-capable, or worth tracking in Runs. It uses the existing system `Command Agent` with a narrowed tool subset.
- Existing-agent orchestration: for requests that clearly match saved/system/custom agents, or ask for chaining, fan-out, or DAG-style work. Dispatch creates normal `command_bar_plans` and `agent_run`s.
- Reusable custom agent creation: only when the user asks for a recurring capability. Ask Agents proposes a validated custom-agent draft and creates the agent only after explicit approval.
- Create-agent-and-run: only when the user explicitly wants a new reusable agent and immediate execution.

## Backend Changes

- Add durable chat storage:
  - `command_bar_threads`: workspace, actor, title, status, timestamps.
  - `command_bar_messages`: thread, role, content, optional structured proposal, timestamps.
- Add `POST /command-bar/chat/turns`:
  - request: `thread_id?`, `text`, `page_context`.
  - response: thread, assistant message, optional proposal.
- Add `GET /command-bar/chat/threads`:
  - returns recent open threads with their latest messages so the dock can hydrate the last conversation.
- Add proposal types:
  - `inline_answer`: answer generated with read-only tools only.
  - `run_plan`: existing `CommandBarPlan` steps for known agents, one-shot, fan-out, task pipeline, or DAG.
  - `create_agent`: validated `CustomAgentDraft`.
  - `create_agent_and_run`: validated draft plus first run target/instruction.
  - `clarification`.
  - `no_match`.
- Add confirmation endpoints:
  - keep existing `/command-bar/plans/dispatch` for `run_plan`.
  - add create-agent confirmation for `create_agent`.
  - add create-agent-and-run confirmation that creates the agent through `AgentService.CreateAgent`, then starts a normal run or plan.
- Reuse existing services:
  - `CommandBarService` for plan validation and dispatch.
  - `AgentService.DraftCustomAgentWithCatalog` for draft validation.
  - `AgentService.CreateAgent` for custom-agent persistence.
  - existing agent run start path for execution.
- Revalidate on confirmation: permissions, target support, agent/team access, allowed targets, one-shot tool subsets, custom-agent tools/skills, and DAG dependency limits.

## Planner Policy

- The chat planner receives recent thread messages, page context, accessible agents, readable tools, executable tools, and skill catalog.
- Use `inline_answer` when the request can be answered quickly with non-mutating tools.
- Use `run_plan` when the user asks for durable work, orchestration, long-running work, approval, mutation, or traceable execution.
- Use `Command Agent` one-shot steps for ad hoc non-reusable work.
- Use saved/system/custom agents when the request clearly matches their purpose.
- Use `create_agent` only for reusable or recurring roles.
- Ask a clarification instead of guessing when scope, target, permissions, or destructive intent is ambiguous.
- Never enable all tools by default for execution; every durable run must have a narrowed, validated tool or agent scope.

## Frontend Changes

- Upgrade `AskAgentsDock` into a real chat thread:
  - render persisted user and assistant messages.
  - hydrate recent active threads.
  - keep current collapsed dock, run history, approval, and drawer behavior.
- Add proposal renderers:
  - inline answer card.
  - existing plan preview for `run_plan`.
  - custom-agent draft preview for `create_agent`.
  - combined draft plus first-run preview for `create_agent_and_run`.
  - clarification card.
- Keep confirmation explicit:
  - no approval needed for inline read-only answers.
  - `Approve & run` for run plans.
  - `Create agent` for reusable agents.
  - `Create agent & run` for combined proposals.
  - `Edit` and `Discard` available before execution.
- Preserve post-run promotion from completed one-shot Command Agent runs.

## Tests And Docs

- Backend tests:
  - inline read-only Helpin/settings/docs/CRM question returns no run.
  - one-shot request returns narrowed Command Agent proposal.
  - existing-agent chain and DAG proposals dispatch normal runs.
  - reusable-agent request returns validated draft.
  - create-agent-and-run creates one custom agent and one normal run or plan.
  - stale tools, invalid targets, team-scope violations, and unsafe mutations are rejected on confirmation.
- Frontend tests:
  - renders inline answer without dispatch.
  - renders and confirms run plan.
  - renders and confirms custom-agent draft.
  - renders clarification.
  - does not create or dispatch anything before explicit confirmation.
- Docs:
  - update `docs/AGENTS_AND_AUTOMATION.md` with Ask Agents as the orchestration chat layer.
  - update command-bar architecture docs to include inline read-only answers and proposal confirmation.

## Assumptions

- "All accessible tools and skills" means planner visibility, not execution with every tool enabled.
- Inline answers may use only non-mutating tools or read-only service calls, and must hand off to a one-shot run when broader live lookup is needed.
- Durable work continues through existing `agent_run` and `command_bar_plan` primitives.
- One-shot work uses the existing system `Command Agent`.
- Saved custom agents are created only through explicit user approval.
