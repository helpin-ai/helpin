# Public demo workspace

A read-only workspace in prod that shows the whole product to visitors without
an account. Helpin supports Helpin here: it is seeded from the public `helpin`
and `agent-runtime` repos and the public docs, with fictional teams and a
recurring cast of bot customers. History is never reset; it is the input for
support gap detection.

Only one product change backs this: `POST /api/auth/demo` mints a session for a
shared viewer account, and every non-read request from that account is rejected
(see `DEMO_*` in `docs/community/configuration.md`). Everything else is set up
once through the product, as a customer would.

## One-time setup checklist

1. Create the demo workspace under a dedicated owner account. Not the real
   Helpin workspace.
2. Create the viewer account (`DEMO_VIEWER_EMAIL`). Invite it to the demo
   workspace as `viewer`, set that as its default workspace. No 2FA, not a
   platform admin, member of no other workspace.
3. Install the GitHub App against the public `helpin` and `agent-runtime` repos.
4. Import the public docs into Docs and the help center.
5. Create the fictional teams, the support team, and the "human" agent
   accounts the reply bots will use. Members of the demo workspace only.
6. Create the support inbox. Enable the AI agent. Set `allowed_origins` to the
   demo site domain. Set the welcome text to say conversations are public.
   Enable identity verification (HMAC) once the demo site signs identify calls.
7. Deploy the demo site with the widget embed.
8. Sign in as the viewer account and walk every settings page. Nothing shown
   there may be a secret.
9. Set `DEMO_VIEWER_EMAIL` (and optionally `DEMO_REQUIRE_EMAIL`,
   `DEMO_LEAD_WEBHOOK_URL`) on the prod API, redeploy, open `/demo`.

## Rules that keep it honest

- The demo widget lives on the demo site only. The marketing site widget points
  at the real workspace.
- Bots write; visitors only read. Bot personas are fixed and recurring so the
  CRM shows repeat customers rather than thousands of one-off contacts.
- Issue authors are stripped on import and re-attributed to bot personas.
- Real visitors who chat in the demo widget see that the conversation is
  public. Those conversations are tagged by source and moderated, not wiped.

## Not yet built

- Visitor bots (agent-browser through the widget, scenario library, daily cap)
  and reply bots (API service accounts), as CronJobs in `k8s/prod`.
- Demo site accounts for bot personas with HMAC-signed widget identify.
- An external MCP server exposing demo-site user data to the support agent.
- Backups for the demo workspace.
- A "public demo" banner in the app for demo sessions.
- Recorded loop for the marketing site.
