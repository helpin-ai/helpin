# PM Recurring Template Reuse Design

## Summary

Helpin already has two adjacent but separate concepts:

- `pm_story_templates` for manual story presets
- `pm_recurring_templates` for schedule-driven story generation

Recurring templates cannot currently be used when creating a new story, even though they already contain most of the same seed data. The gap is not in persistence. It is in the creation UX and the adapter layer between recurring-template seed data and the story-create form.

This design adds recurring templates as a second preset source in the story creation flow without changing the meaning of recurring ownership or generated-story linkage.

## Current State

### What exists today

- The story create modal applies only `StoryTemplate` records from `pm_story_templates`.
- Recurring templates are created from an existing story and require `story_id`.
- Recurring templates already store a structured story seed in `SeedPayload`.
- The recurring-template API already returns parsed `seed` data through `RecurringTemplateDetail`.

### Why it does not work today

- The create modal does not load recurring templates.
- The create modal has no mapper from `RecurringTemplateDetail.seed` into its local form state.
- There is no product decision in the current flow about whether recurring-template workflow/state fields should override the modal's current creation context.

## Goals

- Let users start a new story from a recurring template.
- Reuse existing recurring-template data instead of duplicating storage.
- Keep the current meaning of `recurring_template_id`: it links generated occurrences to their parent recurring template.
- Avoid a broad refactor of story-template and recurring-template models.

## Non-Goals

- Redefining recurring templates as generic story templates.
- Letting manually created stories pretend to be generated occurrences.
- Rebuilding recurring-template authoring to work from scratch instead of from an existing story.
- Merging `pm_story_templates` and `pm_recurring_templates` in this change.

## Approaches

### Approach 1: Frontend-only preset reuse from existing recurring-template detail

Load recurring templates in the create modal, read their existing `seed`, and apply that seed into form state the same way story templates are applied today.

Pros:

- Smallest change
- Reuses existing API shape
- Low backend risk

Cons:

- The modal must understand two preset shapes
- Filtering logic remains partly frontend-owned

### Approach 2: Add a dedicated recurring-template preset API

Add a backend endpoint that returns only the fields needed to prefill the create-story form, already normalized for UI use.

Pros:

- Cleaner frontend contract
- Easier future reuse in other create flows
- Better place for workflow/team filtering and normalization rules

Cons:

- More backend work for a feature that can already be supported with current data

### Approach 3: Unify story templates and recurring templates under one preset model

Treat both as variants of a single reusable preset concept with optional recurrence settings.

Pros:

- Strong long-term consistency
- Cleaner product model if presets expand further

Cons:

- Larger refactor
- Higher migration and compatibility risk
- Not justified for the immediate need

## Recommendation

Use Approach 1 now.

It is the most direct path because recurring-template `seed` data already exists in the current API response. The product problem is selection and application, not missing data. This keeps the recurring engine intact and avoids overloading `recurring_template_id` semantics.

If recurring-template reuse later needs to appear in more places than the create-story modal, we can add the dedicated preset endpoint in a follow-up without undoing this work.

## Product Design

### Create-story entry point

Add recurring templates as an additional preset source in the create-story modal.

Preferred UX:

- Replace the current `Apply template` control with a unified `Apply preset` control.
- Show both manual story templates and recurring templates in one menu.
- Visually label each option as `Template` or `Recurring`.

Alternative acceptable UX:

- Keep the current story-template picker
- Add a second `Apply recurring template` picker nearby

The unified picker is preferable because the user intent is the same: prefill a story from a saved preset.

### Prefill behavior

When a recurring template is selected, prefill the form with data from `RecurringTemplateDetail.seed`:

- `name`
- `description`
- `story_type`
- `priority`
- `severity`
- `estimate`
- `team_id`
- `owner_member_id`
- `requester_member_id`
- `label_ids`
- `epic_id`
- checklist items
- external links

The modal should also auto-open checklist and external-link sections when the selected recurring template contains data for them, matching current story-template behavior.

### Workflow and state behavior

Do not blindly apply `seed.workflow_id` and `seed.workflow_state_id` into the modal.

For manual creation, the modal's current context should remain authoritative:

- keep the currently selected workflow
- keep the currently selected target state

Reason:

- users may open the modal from a specific board column
- recurring templates may have been created in another workflow or from an older state configuration
- silently changing state/workflow during create is surprising and harder to reason about

If the template's team differs from the current form team, applying the recurring template may update the selected team, following the current behavior used for story templates.

## Data and API Design

### Backend changes

No required schema change is needed for the first version.

No required API change is needed for the first version either, because `RecurringTemplateDetail` already includes:

- `template`
- `config`
- `seed`

Optional backend follow-up:

- add a dedicated recurring-preset list endpoint if payload size or normalization logic becomes a concern

### Frontend mapping layer

Add a small adapter that converts `RecurringTemplateDetail` into the create-modal form state.

This adapter should intentionally ignore:

- `template.id` as story linkage
- `config` for story creation
- `seed.workflow_id`
- `seed.workflow_state_id`

This adapter should preserve the distinction between:

- using a recurring template as a manual preset
- generating a story from a recurring template as an automated occurrence

## Provenance and Linking

Do not set `recurring_template_id` when a user manually creates a story from a recurring template preset.

That field currently means the story belongs to the recurring chain and is used by the recurring system for generated stories. Reusing it for manual prefills would conflate two different concepts and make run history, badge behavior, and future automation harder to reason about.

If the product later wants provenance such as `created from recurring template X`, add a separate field for that purpose.

## Error Handling

- If recurring templates fail to load, the create modal should still work normally with story templates and manual entry.
- If a recurring template contains references that are no longer valid in the current workspace context, apply the valid fields and ignore invalid references.
- If labels, team, epic, or owner references are filtered out by current workspace/team availability, the modal should not block story creation.

## Testing

### Frontend

Add focused tests for the create-story modal:

- recurring templates are loaded and shown in the preset picker
- selecting a recurring template applies the expected form fields
- checklist and external-link sections auto-open when seed data exists
- current state selection is preserved after applying a recurring template
- creating from a recurring template does not send `recurring_template_id`

### Backend

No required backend tests for v1 if no backend code changes are made.

If a dedicated preset endpoint is added later, test:

- list filtering
- seed normalization
- auth and workspace scoping

## Implementation Outline

1. Extend the create-story modal to load recurring templates alongside story templates.
2. Add a unified preset model in the modal layer for `story template` and `recurring template` entries.
3. Add a recurring-template-to-form adapter.
4. Update the preset picker UI to present both sources clearly.
5. Add regression tests around selection, prefill behavior, and request payload shape.

## Rollout Notes

- This change is safe to ship incrementally because it does not alter recurring generation behavior.
- Existing recurring templates immediately become reusable in the create-story flow without data migration.
