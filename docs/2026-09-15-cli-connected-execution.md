# CLI connected execution and terminal UX

This is the September 15 delivery record for contributors and operators working
on connected local execution. Reviewed against the Helpin checkout on 2026-09-17:
the backend paths below exist, but the CLI release, terminal behavior, deployment,
and original test results require separate verification.

## Source verification boundary

[Admission and renewal](../server/internal/service/cli_admission.go),
[execution/reporting](../server/internal/service/cli_execution.go), and the
[provider gateway](../server/internal/service/cli_gateway.go) implement the
server-side flow. The gateway requires the saved OpenRouter route without a
custom endpoint and an API-key credential. Both enablement flags are false unless
set through [configuration](../server/internal/config/config.go).

Saved generation replay requires the same request hash and a completed journal
entry with no busy execution fence. It is not an unconditional retry guarantee.
After provider response, persistence and settlement use detached contexts bounded
at 15 and 20 seconds respectively. Reporting stores client output under
`local_report`, derives usage from the provider journal, and publishes run state;
it does not call cloud delivery/publishing finalizers.

The [compiled-CLI HTTP tests](../server/internal/service/cli_http_integration_test.go)
skip when `AGENT_RUNTIME_CLI_TEST_BINARY` is unset. A passing broader Go suite
therefore does not by itself establish CLI end-to-end coverage. The runtime CLI
source and the two named smoke scripts are not included in this Helpin repository;
the available adjacent runtime checkout also lacks them. Commands, 30-second
client renewal, terminal behavior, and patch collection described below remain
claims from the original delivery record, not reverified client behavior. Use a
matching runtime checkout and compiled binary before relying on those instructions.


## Recorded delivered scope

The original Go CLI now uses a full-screen terminal with a scrollable transcript,
provider detection/preferences, run history/resume, event details, and Git diffs.
Helpin's local execution path now includes automatic execution leases, a managed
OpenRouter API-key model gateway, private local-report artifacts, and shared run
messages including tool details. The shared transcript identifies local provenance
and directs replies/approvals back to the terminal.

Provider requests are journaled with stable request IDs. Saved responses replay
without another provider call or usage charge. Unknown outcomes remain fenced;
revoke the admission and inspect it before starting another. Settlement continues
for a bounded period after a client disconnect. Only persisted provider usage
enters normal usage accounting; client-supplied output is nested as local_report.
Local completion does not trigger cloud delivery, task verification, or publishing.

## Recorded rollout procedure

1. Deploy the additive `202609150002_cli_connected_execution.sql` migration and
   backend changes after the earlier CLI OAuth/admission migration.
2. Configure the existing CLI OAuth origins and set `CLI_ENABLED=true` plus
   `CLI_MODEL_GATEWAY_ENABLED=true`. Both default to false.
3. Use an eligible native coding/review agent with a supported OpenRouter API-key
   AI profile. Other provider routes and custom endpoints fail admission before
   usage preflight. Credentials remain in Helpin.
4. Install the rebuilt `agent-runtime-cli`. Existing named connections run
   `agent-runtime-cli connections refresh NAME` to refresh capabilities. Identity
   and scopes remain pinned; changed identity requires a separate connection.
5. Run `agent-runtime-cli run --connection NAME --agent ID --target task:ID
   --dir /checkout 'Implement and test the change'` (one shell command).

The server renews a 15-minute grant; the CLI renews every 30 seconds during work.
Local approval pauses stop renewal; resume reacquires an expired grant. Saved
history can be resent with `agent-runtime-cli runs sync LOCAL_RUN_ID`.
Patches cover tracked unstaged checkout changes, at most 256 KiB. They are not
isolated per-run diffs. Binary attachments, incremental sync, remote app tools,
live managed token streaming, additional providers, and automatic worktrees are
outside this slice.

## Validation and operational limits

Tests exercise the compiled CLI through real Helpin HTTP handlers and OAuth PKCE,
with seeded SQLite and a scripted model. They perform actual local Python edits
and tests, persist shared messages/artifacts, reject stale/forged grants, and
check usage replay and disconnect settlement. A separate wire test covers the
OpenRouter request/response tool and reasoning conversion. Terminal smoke tests
use an actual PTY, alternate-screen entry/exit, resizing, scrolling, and provider
detection. These are local integration checks, not live provider or deployed
PostgreSQL validation. Deployment and enablement are separate rollout steps.

Original verification record (not rerun by this documentation audit):

- Runtime: `go test ./...`, `go vet ./...`, compiled binary build, CLI race suite,
  `scripts/cli-smoke.py`, and `scripts/cli-terminal-smoke.py`.
- Helpin (run from `server/`, replacing the binary path): `AGENT_RUNTIME_CLI_TEST_BINARY=/absolute/path/to/agent-runtime-cli go test
  -p 1 -race ./internal/service ./internal/handler -run '^(TestCLI|TestCodingSession)'
  -count=1` (one shell command).
- Frontend: `pnpm --dir frontend exec tsc -b` and the coding-session composer
  regression suite (8 tests).

The final security regressions also cover simultaneous report/model attempts and
truncated tool-call JSON retaining usage without executing an incomplete tool.
