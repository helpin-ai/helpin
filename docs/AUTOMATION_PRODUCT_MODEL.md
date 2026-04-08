# Automation Product Model

This doc describes the user-facing Automation model.

Use it for product, design, and UX decisions. For backend/runtime details, see [AGENTS_AND_AUTOMATION.md](/root/teampulse/docs/AGENTS_AND_AUTOMATION.md).

## The mental model

Users should understand Automation as one sentence:

`When X happens, run Y agent on Z.`

They should not need to assemble this from separate nouns like trigger, rule, execution, and run.

## Primary surfaces

Automation should read as four connected surfaces:

- `Flows`
  Configuration. What is supposed to happen.
- `Activity`
  Operational history. What actually happened.
- `Agents`
  Executors. Which agents are available and how they are configured.
- `Library`
  Reference. Which triggers and tools exist.

## Surface relationship

```text
                         +------------------+
                         |     Library      |
                         | triggers / tools |
                         +---------+--------+
                                   |
                                   | discover
                                   v
 +------------------+      +-------+--------+      +------------------+
 |      Agents      |<-----+      Flows     +----->|     Activity     |
 | executor config  |      | configuration  |      | what happened    |
 | and ownership    |      | when X -> run Y|      | match / skip / fail
 +---------+--------+      +-------+--------+      +---------+--------+
           ^                       |                         |
           |                       | launches                | links to
           |                       v                         v
           |               +-------+--------+        +-------+--------+
           +---------------+    Agent Runs  +--------+    Outcome      |
                           | execution      |        | messages/artifacts
                           +----------------+        +------------------+
```

Read this as:

- `Library` helps users discover building blocks
- `Flows` is where users define automation behavior
- `Agents` supplies the executors used by flows
- `Activity` explains what happened when events arrived
- `Agent Runs` and outcome details are the deeper runtime layer behind successful launches

## What each surface answers

### Flows

Answers:

- what starts this
- what filters apply
- which agent runs
- what target it uses

Each flow should read like a sentence:

- `When`
- `If`
- `Then`
- `Using`
- `On`
- `Recent activity`

### Activity

Answers:

- what happened
- what matched
- what did not match
- what was skipped
- what failed before launch
- what run started
- what the outcome was

This is the top-level debugging surface.

`Activity` should include both automation-triggered events and manual launches.

### Agents

Answers:

- what this agent is for
- what can trigger it
- what it has run recently
- what tools and permissions it has

This remains executor-focused, not the primary place to understand the whole automation system.

### Library

Answers:

- what triggers exist
- what tools exist

This is reference, not the primary workflow surface.

## Object distinctions

The UI should teach the causal chain, even if backend records are separate:

```text
event happens
  -> flow matches or does not match
  -> automation activity is recorded
  -> agent run may start
  -> agent produces an outcome
```

Important distinction:

- `Activity` is the top-level user-facing operational log
- `Agent Runs` are deeper execution records

Do not collapse them into one storage concept, but do present them as one story in the UI.

## Flow detail shape

Each flow detail page should tell the whole story in one place:

```text
When this happens
  -> trigger family and event

Only if
  -> filters and scope

Then do this
  -> action

Using
  -> agent

On
  -> target

Recent activity
  -> matched / skipped / failed / launched

Run outcome
  -> what the agent actually did
```

## Built-ins vs user flows

Not every automation is the same kind of object.

We should show both in one ecosystem, but distinguish:

- `User flows`
- `System automations`

They may share layout patterns, but should not pretend to be authored the same way.

## Navigation truth

Automation is now a top-level workspace area.

Canonical routes:

- `/w/:slug/automation/flows`
- `/w/:slug/automation/activity`
- `/w/:slug/automation/agents`
- `/w/:slug/automation/library`
- `/w/:slug/automation/tools`
- `/w/:slug/automation/runs`

Legacy PM or settings locations may still redirect, but should not be presented as the main home.

## Product language rules

Prefer:

- `Flows`
- `Activity`
- `Agents`
- `Library`

Avoid leading with:

- `AI & Automations`
- raw `automation rules`
- raw `trigger executions`

Those are implementation terms, not the primary user mental model.

## Success criteria

A user should be able to open one flow and answer:

- What starts this?
- What conditions apply?
- Which agent runs?
- What target does it use?
- What happened recently?
- If it did not run, why not?
- If it ran, what was the outcome?
