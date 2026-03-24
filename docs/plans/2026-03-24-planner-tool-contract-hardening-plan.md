# Planner Tool Contract Hardening Plan

## Status

Draft plan for tightening the contract between:

- planner system prompts
- tool schemas exposed to the model
- backend decode and normalization
- approved-artifact application

This plan focuses on epic planner and story planner first, because they are the most sensitive to prompt/schema drift and because approval-driven artifact application is already the dominant interaction model.

## Goal

Make planner runs behave like a production-grade tool-calling system:

- the model sees one clear contract per planner output
- prompt examples match the tool schemas exactly
- backend decode and apply logic accept only intentional, documented variations
- artifact approval and application are deterministic
- regressions are caught by contract tests before they reach production

## Non-Goals

This phase does not aim to:

- replace the current provider/runtime stack
- redesign the PM planning workflow itself
- remove all compatibility handling immediately
- generalize every planner concept into a new framework before the contract is stable

## Current Problems

### 1. Prompt/schema/backend drift

We have repeatedly seen planner outputs fail because the model emitted:

- `title` instead of `name`
- `type` instead of `story_type`
- `test_strategy` as an array instead of a string
- nested or stringified preview payloads

These are all symptoms of the same issue: the model contract is not strict enough in one place and not tolerant enough in another.

### 2. Too much contract knowledge is implicit

Right now, the model learns planner tool usage from:

- the JSON schemas in [server/internal/worker/tools.go](/root/teampulse/server/internal/worker/tools.go)
- the preset prompt text in [server/internal/service/agent_system_prompts.go](/root/teampulse/server/internal/service/agent_system_prompts.go)

That is workable, but some high-value tools still rely too much on prose instead of one canonical example.

### 3. Compatibility normalization is scattered

We already normalize several planner cases in:

- preview publishing
- approved artifact decoding
- planner story validation

That keeps the system alive, but it is not yet organized around a documented compatibility boundary.

### 4. There is no full contract-test layer

We have targeted tests for pieces of the planner stack, but we do not yet have a strong end-to-end contract suite that proves:

1. the prompt example shape
2. the tool schema shape
3. the decoded Go model shape
4. the approved artifact application path

all agree.

## Design Principles

1. Canonical shapes first, compatibility second.
2. One planner action should have one primary tool shape.
3. Prompt examples should be short, exact, and copy-pasteable by the model.
4. Compatibility handling should be deliberate and documented, not accidental.
5. Backend application must be deterministic and idempotent for approved artifacts.
6. Model-facing errors should help the model repair output, not just help engineers debug.

## Target Contract Model

## 1. Canonical planner preview shapes

Define and document one canonical payload per planner artifact.

### Epic PRD preview

Tool: `publish_prd_draft`

Canonical shape:

```json
{
  "title": "PRD Draft",
  "content": "# Problem\n..."
}
```

### Epic story plan preview

Tool: `publish_story_plan`

Canonical shape:

```json
{
  "title": "Story Plan",
  "content": {
    "summary": "...",
    "proposed_stories": [
      {
        "ref": "story_1",
        "name": "Add tracking helper",
        "description": "...",
        "story_type": "chore",
        "acceptance_criteria": ["..."],
        "dependency_refs": [],
        "slice_type": "enabler",
        "implementation_brief": {
          "approach": "...",
          "files_to_modify": [
            {
              "path": "server/internal/worker/tools.go",
              "action": "modify",
              "description": "..."
            }
          ],
          "test_strategy": [
            "..."
          ]
        }
      },
      {
        "ref": "story_2",
        "name": "Wire tracking into capture errors",
        "description": "...",
        "story_type": "feature",
        "acceptance_criteria": ["..."],
        "dependency_refs": ["story_1"],
        "slice_type": "vertical",
        "implementation_brief": {
          "approach": "...",
          "files_to_modify": [
            {
              "path": "server/internal/capture/errors.go",
              "action": "modify",
              "description": "..."
            }
          ],
          "test_strategy": [
            "..."
          ]
        }
      }
    ],
    "open_questions": [],
    "risks": []
  }
}
```

`dependency_refs` must contain refs that exist elsewhere in the same `proposed_stories` array. Example: `"dependency_refs": ["story_1"]` means the current story depends on the story whose `ref` is `"story_1"`.

### Story planning doc preview

Tool: `publish_story_plan_doc`

Canonical shape:

```json
{
  "title": "Story Planning Document",
  "content": "# Outcome\n..."
}
```

## 2. Explicit compatibility boundary

We should intentionally support a small alias set during migration:

- `title` -> `name` for proposed stories
- `type` -> `story_type`
- `test_strategy` string or string array
- stringified JSON content for preview `content`

Everything else should be considered unsupported and surfaced as a precise validation error.

## Workstreams

## Workstream 1: Canonicalize Planner Payload Models

### Objective

Make the backend model layer the single documented source of truth for planner payloads.

### Tasks

1. Add explicit custom unmarshal logic for planner payload models where compatibility is required.
2. Normalize all accepted aliases into canonical Go fields immediately on decode.
3. Add a small helper layer for planner payload normalization instead of repeating ad hoc fixes in multiple call sites.
4. Document the accepted compatibility aliases inline in code comments.

### Primary files

- [server/internal/model/agent_planning.go](/root/teampulse/server/internal/model/agent_planning.go)
- new file: `/root/teampulse/server/internal/model/agent_planning_normalization.go`
- [server/internal/temporalapp/activities.go](/root/teampulse/server/internal/temporalapp/activities.go)

### Exit criteria

- approved story-plan preview decoding is resilient to the documented alias set
- normalized planner models are canonical by the time they hit validation and apply logic
- no planner apply path depends on raw model output naming quirks

## Workstream 2: Tighten Tool Schemas Around Canonical Shapes

### Objective

Make the tool schemas reflect what we truly accept, no looser and no stricter.

### Tasks

1. Review `publish_prd_draft`, `publish_story_plan`, and `publish_story_plan_doc` input schemas.
2. Review planner story-plan nested schemas:
   - `proposed_stories`
   - `implementation_brief`
   - `files_to_modify`
3. Encode intentional flexibility in schema where we know model output varies:
   - `test_strategy`: string or string array
4. Remove accidental ambiguity in schema where we want one canonical shape only.
5. Add tests that validate the exported schema contains the expected compatibility forms.

### Primary files

- [server/internal/worker/tools.go](/root/teampulse/server/internal/worker/tools.go)
- [server/internal/worker/tools_test.go](/root/teampulse/server/internal/worker/tools_test.go)

### Exit criteria

- planner tool schemas match the actual accepted backend contract
- schema tests fail if someone narrows or broadens a planner contract unintentionally

## Workstream 3: Rewrite Planner Prompt Examples to Match Schemas Exactly

### Objective

Reduce model drift by making planner prompts short, exact, and schema-aligned.

### Tasks

1. Replace prose-heavy shape descriptions with one canonical JSON example per critical planner tool.
2. Ensure examples use the same field names as the canonical backend model:
   - `name`
   - `story_type`
   - `test_strategy`
3. Add one explicit note for compatibility-sensitive fields only when needed.
4. Keep story planner and epic planner examples separate so context does not leak across flows.
5. Add stronger wording that repeated preview revisions should replace the same artifact, not invent a new shape.

### Primary files

- [server/internal/service/agent_system_prompts.go](/root/teampulse/server/internal/service/agent_system_prompts.go)
- [server/internal/worker/planning_prompt_pack.go](/root/teampulse/server/internal/worker/planning_prompt_pack.go)
- [server/internal/service/agent_system_prompts_test.go](/root/teampulse/server/internal/service/agent_system_prompts_test.go)

### Exit criteria

- every critical planner tool has one exact prompt example
- prompt tests verify those examples remain present
- prompt wording stops encouraging ambiguous field names like `title`/`type` inside story-plan JSON

## Workstream 4: Improve Model-Facing Validation Errors

### Objective

Make invalid planner output easier for the model to self-repair.

### Tasks

1. Replace low-level errors like:
   - `cannot unmarshal array into Go struct field ...`
   - `proposed story 1 is missing a name`
   with compact, repair-oriented messages such as:
   - `story 1 is missing name; use field "name" for the story title`
   - `implementation_brief.test_strategy must be a string or array of strings`
2. Keep internal logs detailed while returning a shorter model-facing message.
3. Standardize error wording for:
   - missing required fields
   - unsupported field type
   - invalid dependency refs
   - invalid preview content format

### Primary files

- [server/internal/service/agent_planning.go](/root/teampulse/server/internal/service/agent_planning.go)
- [server/internal/temporalapp/activities.go](/root/teampulse/server/internal/temporalapp/activities.go)
- [server/internal/worker/tools_preview.go](/root/teampulse/server/internal/worker/tools_preview.go)

### Exit criteria

- planner tool/apply errors are concise and actionable
- the model can correct most malformed planner payloads in one retry

## Workstream 5: Build Contract Tests Across the Full Planner Path

### Objective

Catch prompt/schema/decode/apply drift in CI instead of production.

### Test layers

#### A. Schema tests

Prove the tool definitions expose the expected planner input contracts.

#### B. Prompt tests

Prove the built-in prompts contain exact examples and required constraints.

#### C. Decode tests

Prove planner payloads decode from:

- canonical payloads
- stringified JSON
- accepted alias shapes

#### D. Apply tests

Prove approved artifacts are successfully applied for:

- PRD approval
- story plan approval
- story planning doc approval

### Tasks

1. Add table-driven tests for planner story payload variations.
2. Add tests for approved preview artifact application from stored artifact payloads, not just direct JSON strings.
3. Add regression tests for every real planner failure we have already seen:
   - `panel_key is required`
   - `content is required`
   - stringified story-plan JSON
   - `title`/`type` alias payloads
   - `test_strategy` array payloads
4. Add one snapshot-style test for the canonical story-plan example embedded in the prompt.

### Primary files

- [server/internal/model/agent_planning_test.go](/root/teampulse/server/internal/model/agent_planning_test.go)
- [server/internal/worker/tools_test.go](/root/teampulse/server/internal/worker/tools_test.go)
- [server/internal/worker/tools_interaction_test.go](/root/teampulse/server/internal/worker/tools_interaction_test.go)
- [server/internal/service/agent_system_prompts_test.go](/root/teampulse/server/internal/service/agent_system_prompts_test.go)
- [server/internal/temporalapp/activities_test.go](/root/teampulse/server/internal/temporalapp/activities_test.go)

### Exit criteria

- planner tool contracts are covered by targeted regression tests
- newly introduced prompt/schema drift fails CI

## Workstream 6: Add a Canonical Planner Contract Reference

### Objective

Stop relying on tribal knowledge when adjusting planner prompts or schemas.

### Tasks

1. Create a small reference doc describing:
   - planner tools
   - canonical payloads
   - compatibility aliases
   - model-facing error rules
2. Link it from:
   - planner prompt code comments
   - planner schema tests
   - future planner refactors

### Candidate file

- `/root/teampulse/docs/AGENTS_AND_AUTOMATION.md`
  or
- new file `/root/teampulse/docs/plans/planner-tool-contract-reference.md`

### Exit criteria

- engineers can update planner tools without rediscovering the contract from production failures

## Recommended Sequence

1. Canonicalize planner payload models.
2. Tighten tool schemas.
3. Rewrite prompt examples to match the schemas exactly.
4. Improve model-facing validation errors.
5. Add full contract tests.
6. Publish a compact contract reference.

This order matters:

- model normalization stabilizes apply behavior first
- schema and prompt cleanup then reduce new malformed output
- better errors and tests keep the system stable afterward

## Concrete Deliverables

### Deliverable 1

Canonical planner payload decoding with documented alias support.

### Deliverable 2

Planner tool schemas that match actual runtime acceptance.

### Deliverable 3

Prompt examples for:

- `publish_prd_draft`
- `publish_story_plan`
- `publish_story_plan_doc`
- `request_human_approval`

### Deliverable 4

Regression tests covering every planner payload failure already observed in production-like runs.

### Deliverable 5

A short internal contract reference for future planner work.

## Success Metrics

We should consider this effort successful when:

- planner approval/application failures from field-shape mismatches drop to near zero
- planner retries become semantic revisions, not syntax repairs
- new planner prompt/schema changes require updating tests in one obvious place
- engineers can explain the planner contract without reading five different files

## Open Questions

1. Do we want `test_strategy` to remain a normalized string internally, or should it become a first-class `[]string` in the model?
2. Should `title`/`type` remain a compatibility alias indefinitely, or should we deprecate them after prompt stabilization?
3. Should planner preview tools eventually emit a versioned artifact envelope with its own typed payload structs, instead of generic preview content?

## Recommendation

Do this work before any broader planner-runtime redesign.

The current system is close enough to stable that a contract-hardening pass will pay off immediately. A transport or orchestration rewrite before this cleanup would just move the same planner ambiguity into a new layer.
