# AI Usage Metering Corrections Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Correct AI usage feature selection, duplicate runtime reply charging, execution idempotency, and cached/reasoning token normalization without adding customer-facing complexity.

**Architecture:** Preserve the central `AIUsageMeter` and one normalized formula. Make call sites supply stable execution identities and make provider adapters supply complete token breakdowns.

**Tech Stack:** Go 1.24, standard-library tests, OpenAI-compatible Chat Completions, Anthropic Messages API.

---

### Task 1: Feature catalog and runtime reply charging

**Files:**
- Modify: `server/internal/service/ai_usage_meter.go`
- Modify: `server/internal/service/internal_command_support_reply.go`
- Test: `server/internal/service/ai_usage_meter_test.go`
- Test: `server/internal/service/internal_command_support_reply_test.go`

- [ ] Add tests proving the command-bar feature is absent and runtime reply delivery does not create a second usage charge.
- [ ] Run the focused tests and confirm they fail for the intended behavior.
- [ ] Remove the unused command-bar feature and runtime delivery charge.
- [ ] Run the focused tests and confirm they pass.

### Task 2: Execution-scoped idempotency

**Files:**
- Modify tests and call sites in `server/internal/service/crm_summary.go`, `support_ai_admin.go`, `crm_deal_automation.go`, and `docs_helpcenter_translation.go`.

- [ ] Add failing tests showing changed source versions or executions produce different keys while retries of one execution remain stable.
- [ ] Add source timestamps, triggering message IDs, or prompt hashes to the affected keys.
- [ ] Run focused service tests and confirm they pass.

### Task 3: Provider token normalization

**Files:**
- Modify: `server/internal/llm/provider.go`
- Modify: `server/internal/llm/openai.go`
- Modify: `server/internal/llm/claude.go`
- Modify: `server/internal/agentcontract/claude.go`
- Modify: `server/internal/service/ai_usage_meter.go`
- Test: corresponding `*_test.go` files.

- [ ] Add failing decoder and meter tests for cached input, cache writes, and reasoning output.
- [ ] Extend usage structs and provider response decoding.
- [ ] Pass normalized fields through `MeteredLLMProvider` and record cache writes as metadata.
- [ ] Run focused LLM and metering tests and confirm they pass.

### Task 4: Verification

- [ ] Run `go test ./internal/llm ./internal/agentcontract ./internal/service`.
- [ ] Run `go test ./...`.
- [ ] Run `go vet ./...`, `gofmt`, and `git diff --check`.
- [ ] Review the final diff for unrelated changes and behavioral scope.

