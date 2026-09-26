# Sprint automation prompt dismissal design

> Historical design, source-compared on 2026-09-17. This page explains the
> original dismissal decision for contributors. The core dismissal is implemented;
> the original settings navigation and fetch-error guarantee below are outdated.

## Current implementation

The [prompt helper](../../frontend/src/components/pm/sprintAutomationPrompt.ts)
suppresses the prompt when a team has any `sprint_auto_create` row, enabled or
not. Dismissal saves a disabled row with the selected count, duration, and start
day; it does not change `sprint_move_unfinished`. The
[creation dialog](../../frontend/src/components/pm/GlobalCreateModals.tsx) uses
this handler for both **No thanks** and closing the prompt. It closes after a
successful save and displays an error without closing if saving fails.

The same dialog catches thrown errors while fetching automations, but ignores
the API result's `error` field. Missing data makes the helper return true, so
an error returned without throwing can still display the prompt. The original
claim that every failed fetch skips prompting is not guaranteed by this code.

Sprint automation is now edited in
[team Sprint Settings](../../frontend/src/components/settings/TeamsTab.tsx).
[Automations settings](../../frontend/src/components/settings/AutomationsTab.tsx)
shows a read-only overview with an Edit link to the team. Its “Auto-create ✓”
indicator currently checks row existence, not `enabled`, and can therefore appear
for a dismissed automation. Team settings disables existing rows rather than
deleting them; the original delete-to-reset UI description is not current.
The [repository](../../server/internal/repository/pm_automation.go) still supports
team-scoped deletion, which would remove the suppression marker.

The [helper tests](../../frontend/src/components/pm/__tests__/sprintAutomationPrompt.test.ts)
cover enabled/disabled suppression, dismissal payloads, and save errors. They
are not browser coverage of the entire creation flow and were not rerun for this
source comparison. The original requirements and alternatives follow.

## Original design record

## Summary

Reduce repeat sprint automation prompts by treating `No thanks` as a team-wide dismissal.

The current sprint creation flow shows the sprint automation prompt whenever a team has no `sprint_auto_create` automation row. Clicking `No thanks` only closes the dialog, so the same team is prompted again on the next manual sprint creation.

The approved design keeps the prompt team-scoped, but persists dismissal by creating a disabled `sprint_auto_create` automation row for that team. This reuses the existing PM automation model and settings UI instead of introducing a separate prompt-dismissal settings table.

## Goals

- Stop repeatedly prompting managers for the same team after one explicit decline.
- Keep sprint automation ownership team-specific.
- Preserve a clear path to enable automation later in Settings.
- Avoid adding new backend models or settings tables for prompt state alone.

## Non-Goals

- Changing epic automation behavior.
- Changing when the sprint automation prompt first appears.
- Adding user-level dismissal or cooldown logic.
- Changing the existing `Move unfinished stories` default.

## Behavior before implementation

- After a sprint is created, the modal fetches PM automations for the workspace.
- If the selected team has no `sprint_auto_create` row, it opens the sprint automation prompt.
- Clicking `Enable` creates a team-scoped `sprint_auto_create` row and optionally a `sprint_move_unfinished` row.
- Clicking `No thanks` closes the prompt without persisting any state.
- Because no state is persisted, the same team is prompted again on the next manual sprint creation.

## Options Considered

### 1. Recommended: Use disabled `sprint_auto_create` as the dismissal marker

When a manager clicks `No thanks`, upsert a `sprint_auto_create` automation row for that team with `enabled=false` and the dialog's current config values.

Why this is preferred:

- Reuses an existing team-scoped model and API.
- Matches the current prompt guard, which already treats the existence of `sprint_auto_create` as "this team has been configured".
- Makes the dismissal visible and reversible in Settings.
- Avoids schema expansion for prompt-only state.

Trade-offs:

- A disabled automation row now means either "configured but off" or "dismissed from prompt". That is acceptable because both states should suppress the prompt and leave enablement to Settings.

### 2. Add a dedicated team-level dismissal setting

Store a separate prompt-dismissed flag in team settings.

Advantages:

- Cleaner semantics for prompt state.

Disadvantages:

- Adds new persistence, service, and API surface for a narrow problem.
- Duplicates state that is already representable through existing automation records.

### 3. Add a snooze or cooldown

Hide the prompt temporarily after `No thanks`.

Advantages:

- Smaller product change.

Disadvantages:

- Does not satisfy the goal of fewer repeat prompts.
- Still causes re-prompting without a new team decision.

## Approved Design

### Trigger and Suppression Rules

- Keep the prompt team-scoped.
- Continue showing it only after manual sprint creation for a team with no `sprint_auto_create` row.
- Once a `sprint_auto_create` row exists for that team, never show the prompt again automatically.
- This suppression applies whether that row is enabled or disabled.

### `Enable` Behavior

- No behavioral change.
- Upsert `sprint_auto_create` with `enabled=true`.
- If `Move unfinished stories` is on, also upsert `sprint_move_unfinished` with `enabled=true`.

### `No thanks` Behavior

- Replace the current close-only action with a persistence step.
- Upsert `sprint_auto_create` for the team with:
  - `enabled=false`
  - `config_int` from the dialog's sprint count
  - `config_int2` from the dialog's sprint length in weeks
  - `config_int3` from the dialog's start day
- Do not create or modify `sprint_move_unfinished`.
- After the disabled upsert succeeds, close the modal and show a short success toast confirming the team preference was saved.

### Settings Behavior

- The team should appear in Settings > Automations under `Auto-Create Future Sprints` even if the saved row is disabled.
- A manager can later enable it from Settings without recreating the row.
- If a manager deletes the `sprint_auto_create` row entirely from Settings, that acts as a reset. The prompt may appear again the next time a sprint is manually created for that team.

## Data and API Impact

### Backend

- No schema changes.
- No new models required.
- Reuse the existing PM automation upsert endpoint and repository behavior.

### Frontend

- Add a dedicated dismiss handler for the sprint automation prompt.
- That handler should upsert disabled `sprint_auto_create` instead of only closing the modal.
- Keep the existing prompt guard based on `sprint_auto_create` row existence.

## Error Handling

- If the dismiss upsert fails, keep the modal open and show an error toast.
- Do not silently fall back to close-only behavior, because that would reintroduce repeated prompts and make the saved-state promise unreliable.
- If the initial automation fetch fails after sprint creation, continue current behavior and skip prompting. This is already treated as non-critical.

## Testing

### Frontend

- Add a test covering first sprint creation for a team with no `sprint_auto_create` row: prompt appears.
- Add a test covering `No thanks`: it sends an upsert request for disabled `sprint_auto_create` with the team ID and current config values.
- Add a test covering a subsequent sprint creation when the team has a disabled `sprint_auto_create` row: prompt does not appear.

### Backend

- No new backend behavior is required beyond existing PM automation upsert support.
- If there is service-level coverage around PM automation upsert semantics, no additional backend test is necessary for this design alone.

## Open Questions

- None. The approved behavior is team-wide dismiss with fewer repeat prompts.
