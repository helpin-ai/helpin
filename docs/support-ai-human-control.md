# Human control of support AI

The conversation header shows who is handling the conversation. Teammates with
`support.edit` and access to the conversation's mailbox can change AI control.

- **Pause AI** immediately blocks automatic replies and follow-ups. Existing
  human assignment stays unchanged; an unassigned conversation waits in the
  human queue. No customer message is sent by this action.
- Assigning a teammate or sending a public teammate reply also takes over.
  Reading a conversation and writing an internal note do not take over.
- **Return to AI** releases human assignment and lets the configured support
  agent respond to the **next customer message**. Earlier queued messages are
  not replayed. Existing history is retained for the next AI session.
- Returning after the customer requested a human requires explicit confirmation.
  The historical request remains in the thread. Resolved, spam and anonymized
  conversations cannot be returned; workspace AI and the reply channel must
  be enabled.

A handoff adds one internal briefing with the issue, steps suggested or tried,
outstanding questions, and the reason. Agent-initiated handoff can supply these
fields through `escalate_to_human`. Otherwise the note uses attributed public
transcript excerpts. It does not invent completed actions or require another
model call. These notes are never delivered to the widget or as customer email.
Provider failures therefore do not prevent a factual handoff note.

## Reliability

Ownership changes and AI reply publication serialize on the conversation row.
A monotonic control version rejects conflicting teammate actions and stale
conversation saves. Returning records a timestamp that excludes earlier
customer messages, deferred work, child results, and follow-up episodes. Runtime
cancellation happens after the database commit; a failed cancellation cannot
restore delivery rights, and the existing sweep retries closing revoked runs.

The current Runtime run ID is checked again when an AI reply commits. A run
that started before takeover cannot bind to a later ownership version. Pause
and return are recorded in internal notes; they do not change resolution metrics.

## Deployment

Apply migration `202609160005_support_ai_control.sql` with the normal edition
migration command before restarting Helpin. For EE, from `server/`:

```sh
GOWORK=off go run -tags ee ./cmd/migrate up
```

Community uses the same command without `-tags ee`. Restart the Helpin API and
workers and deploy the frontend. There are no new environment variables or
Agent Runtime / SDK changes. The optional escalation fields `issue_summary`, `attempted_steps`, and
`unresolved_questions` are backward compatible and are defined in the
[escalation tool metadata](../server/internal/service/internal_command_support_reply.go).

Automated coverage includes SQLite service tests, frontend interaction tests,
and a PostgreSQL integration test for publication waiting on a takeover lock.
The latter uses an isolated schema in `SUPPORT_FOLLOWUP_TEST_DSN` and the
`integration` build tag. Before release, check the controls in a real inbox
while an AI reply is in progress, then return to AI and send a fresh message.

## Source references

The [control service](../server/internal/service/support_ai_control.go) checks
eligibility and writes the transition through the
[conversation repository](../server/internal/repository/support_ai_control.go).
The [handoff-note builder](../server/internal/service/support_handoff_brief.go)
keeps the briefing internal. The
[PostgreSQL test](../server/internal/service/support_ai_control_postgres_test.go)
checks serialization with reply publication; its presence is not evidence that
the integration test ran against a deployment.
