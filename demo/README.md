# Public demo workspace

This is the operating note for Helpin's own hosted demo workspace. Community
operators only need the `DEMO_*` settings in
[configuration](../docs/community/configuration.md). The repository
implements passwordless demo login and a read-only guard for the configured
viewer account. It does not provision, seed, or verify a production demo.

The intended demo uses public Helpin and Agent Runtime material, fictional teams,
and recurring bot customers. Retaining that history for support-gap detection
is an operating policy, not a reset/retention mechanism implemented here.

`POST /api/auth/demo` issues a session for an existing account selected by
`DEMO_VIEWER_EMAIL`. On authenticated routes using the demo middleware, requests
other than GET, HEAD, and OPTIONS return `403` with `demo_read_only`. Public
login and widget endpoints are separate; this guard does not make the public
widget read-only. See [demo configuration](../docs/community/configuration.md).

The login handler rejects a platform-admin or 2FA-enabled viewer account. It
does not validate that the account belongs to exactly one workspace or has only
the viewer role. Verify those setup requirements before enabling public access.

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

## Intended demo operating rules

- The demo widget lives on the demo site only. The marketing site widget points
  at the real workspace.
- Bots write; visitors only read. Bot personas are fixed and recurring so the
  CRM shows repeat customers rather than thousands of one-off contacts.
- Issue authors are stripped on import and re-attributed to bot personas.
- Real visitors who chat in the demo widget see that the conversation is
  public. Those conversations are tagged by source and moderated, not wiped.

## Planned provisioning and follow-up work (not implemented)

- Visitor bots (agent-browser through the widget, scenario library, daily cap)
  and reply bots (API service accounts), as CronJobs in `helpin-ai/gitops:helpin/prod`.
- Demo site accounts for bot personas with HMAC-signed widget identify.
- An external MCP server exposing demo-site user data to the support agent.
- Backups for the demo workspace.
- A "public demo" banner in the app for demo sessions.
- Recorded loop for the marketing site.

## Implementation references

- [Demo login service](../server/internal/service/auth_demo.go): account checks,
  token issuance, and optional asynchronous lead webhook.
- [Read-only middleware](../server/internal/middleware/demo.go) and
  [route registration](../server/internal/router/router.go): protected methods
  and where the guard applies.
- [Demo page](../frontend/src/pages/Demo.tsx): disabled state, optional email
  collection, and navigation after login.

Source review does not establish whether these settings or the intended demo
content are deployed. Check the actual installation before sharing its URL.
