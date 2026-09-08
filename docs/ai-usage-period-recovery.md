# AI usage renewal and recovery

Posted ledger entries retain their period and price snapshot. Renewal transfers
active reservations, including zero-value paused reservations, into the next
allowance. Token checkpoints remain unchanged. New usage posted after renewal
is attributed to the new period; delayed telemetry never edits a closed invoice.
This is posting-time attribution, not an estimate of when each token was generated.

Paused Ask chats release unused holds. Resume reserves capacity again against
the current allowance using the existing reservation identity and price snapshot.
Ask Chat strict caps reset at an explicit user-message resume, using a durable
token baseline. Late events share that turn cap; other agents retain their
cumulative run cap. Stale projections cannot erase a newer turn baseline.
Running work carries its hold across renewal. Carried holds may exceed a reduced
allowance; new holds and increases still undergo strict allowance enforcement.

Rollover serializes with usage writes using a PostgreSQL transaction advisory
lock per workspace. Closing, settlement snapshot creation, opening the successor,
and hold transfer commit together. A partial unique index already enforces one
open period per workspace. Failed workspaces do not block later batches. Missed
renewals advance to the current anniversary window without accumulating unused
allowances. Billing summary reads also recover an expired current period.

Billing-period reports use ledger period IDs rather than entry creation dates.
The UI offers the latest 24 periods and uses the selected period's allowance.
Historical charges remain accessible after rollover; they are not rebilled in
the newly opened allowance. Arbitrary date reports still filter by posting date.

## Production recovery

1. Run `scripts/billing/audit-period-recovery.sql` with a read-only database role.
   Review each overdue period's counter difference, holds to transfer, and
   settlement amount. Nonzero counter differences require investigation before
   rollout; do not automatically rewrite ledger entries or counters.
2. Review pending Stripe settlements separately. Recovery may create settlement
   work for extra-usage plans. Founder periods never create those invoices.
3. Approve deployment and the resulting period/settlement writes before rollout.
   Deploying this code starts recovery through the worker and billing reads;
   there is no separate manual SQL repair required.
4. Rerun the audit: no expired open periods should remain. Compare the ledger sum
   against each period counter, verify holds moved exactly once, and confirm
   previous Ask Chat usage through the billing period selector.
5. Verify a new Ask turn, pause, resume, cancel, and renewal in staging. Confirm
   ledger increments are idempotent and no unused hold survives a user-message
   pause. Monitor rollover errors and pending terminal usage after rollout.

Existing historical postings are preserved, including charges posted late into
previously stalled periods. Moving them into different past periods or changing
an existing invoice is a separate financial adjustment requiring review.

Regression checks include SQLite business cases, runtime event retry tests,
and PostgreSQL concurrency tests. Run the latter against a disposable database:

```sh
BILLING_TEST_DATABASE_URL='host=127.0.0.1 port=55439 user=postgres password=billing-local-test dbname=billing_test sslmode=disable' \
  go test -race -tags=integration ./internal/repository -run TestAIUsagePostgres
```

The integration fixture creates and drops a uniquely named schema. Never point
it at production. No production writes are part of implementing this fix.
