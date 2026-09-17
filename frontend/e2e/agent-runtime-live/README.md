# Agent Runtime live end-to-end checks

These opt-in checks exercise a running Helpin EE frontend, backend, Agent Runtime API, shared worker, and execution worker against the `Usermaven` workspace. They create real chats, runs, artifacts, and approvals. The coding check also creates a real branch and pull request.

Start the local services behind `https://helpin-dev-fe.tryunhide.com` and `https://helpin-dev.tryunhide.com`, then provide a test account without committing its credentials:

```bash
export HELPIN_E2E_EMAIL='test@example.com'
export HELPIN_E2E_PASSWORD='...'
```

Run the independent checks from `frontend/`:

```bash
pnpm test:e2e:agent-runtime:ordinary
pnpm test:e2e:agent-runtime:analysis
pnpm test:e2e:agent-runtime:typesafe
pnpm test:e2e:agent-runtime:interrupted
```

The analysis check prints its chat and run IDs. Use those IDs to verify private artifact download and durability after terminal cleanup:

```bash
HELPIN_E2E_CHAT_ID='<chat-id>' \
HELPIN_E2E_RUN_ID='<run-id>' \
pnpm test:e2e:agent-runtime:artifacts
```

The coding check edits `usermaven/events-pipeline`, pushes a disposable timestamped branch, and opens a pull request. It is disabled unless external writes are explicitly enabled:

```bash
HELPIN_E2E_ALLOW_GITHUB_WRITES=true pnpm test:e2e:agent-runtime:coding
```

Each script fails on a missing marker, unexpected route or approval, terminal run failure, or privacy/lifecycle regression. Provider failures are reported as failures rather than silently retried, so a failed run can distinguish infrastructure instability from a product assertion.
