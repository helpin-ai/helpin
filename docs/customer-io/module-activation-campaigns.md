# Module activation campaigns

This is a campaign design for lifecycle operators, not a description of enabled
campaigns. The [milestone catalogue](../../frontend/src/lib/activationMilestones.ts)
defines the names below, but the current application has no production emitter
for `module_first_value` and no production consumer of that catalogue. Implement
and verify milestone tracking and workspace/user correlation before using it for
entry or suppression. The catalogue test checks definitions, not event delivery.

Module education is recipient-specific. Send it to people with access to the module who have not reached the module’s first-value milestone in the affected workspace.

| Module | First-value milestone | Primary audience |
|---|---|---|
| PM | First task created | Members and admins with PM access |
| Docs | First document created or article published | Members and admins with Docs access |
| Support | First reply sent | Support members and admins |
| CRM | First contact or deal created | CRM members and managers |
| Automation | First automation enabled or executed | Admins/owners with Automation access |

Rules:

- Owners/admins receive setup and workspace activation guidance.
- Team-adoption guidance should target people responsible for team configuration.
  The current workspace role matrix has viewer, member, admin, and owner roles;
  do not segment on an assumed workspace `manager` role.
- Members receive task-specific education tied to their enabled module and use case.
- Viewers are excluded from email education by default.
- Suppress the first-value sequence immediately after the milestone event.
- Keep separate campaigns for separate modules; do not send an all-product feature tour unless the user explicitly selected multiple use cases.

Recommended sequence for an eligible member:

1. Contextual message after module entry without first value.
2. One practical example 24–48 hours later.
3. Final reminder only if the user returned to the module but still has no first value.

Use Usermaven for analysis of module funnels and Customer.io for the behavioral message delivery.

## Implementation limits

[Browser analytics](../../frontend/src/lib/analytics.ts) sends generic events to
Usermaven and Customer.io only on the configured analytics hostname. The
[Community configuration](../../frontend/src/edition/community/config.ts) leaves
that hostname and both keys empty, so these browser analytics calls are disabled
in Community builds. The recommended delays, module-access audience filters,
return-visit checks, and suppression rules require campaign configuration and
verified event data; this repository does not establish their hosted state.
