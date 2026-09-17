# Sprint Team Settings — Implementation Plan

> Source review, 2026-09-17

Historical implementation plan. Team settings now include `SprintsEnabled` in the
[model](../../server/internal/model/settings.go), with a
[sprint settings form](../../frontend/src/components/settings/teams/SprintSettingsForm.tsx)
and save wiring in [TeamsTab](../../frontend/src/components/settings/TeamsTab.tsx).
The tasks below document the original change, not missing functionality to build
again. Runtime automation behavior still depends on each team's saved settings.

> **For agentic workers:** Use superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add per-team sprint enable/disable toggle with configuration settings, and convert the global Automations tab sprint section into a read-only overview.

**Architecture:** Add `sprints_enabled` column to `workspace_teams` table. Sprint config (duration, start day, upcoming count) stays in existing `pm_automations` table. Team settings page gets a new Sprints section. Automations tab becomes a summary view.

**Tech Stack:** Go backend (GORM), React frontend (shadcn/ui Switch, Select components).

---

## Principles

- Default: `sprints_enabled = true` for engineering teams, `false` for custom teams
- Disabling sprints hides UI (sidebar, selectors) but preserves existing sprint data
- Sprint automation settings move to team-level; global Automations tab shows read-only overview
- Follow existing patterns: `DocsPublisherEnabled` column, `EstimateSettingsForm` component

---

### Task 1: Backend — Add `sprints_enabled` to WorkspaceTeam

**Files:**
- Modify: `server/internal/model/settings.go`
- Modify: `server/internal/repository/workspace_team.go`

- [ ] **Step 1: Add column to WorkspaceTeam model**

Add `SprintsEnabled *bool` field to `WorkspaceTeam` struct:
```go
SprintsEnabled *bool `json:"sprints_enabled" gorm:"not null;default:true"`
```

GORM AutoMigrate will add the column on next startup.

- [ ] **Step 2: Add to UpdateWorkspaceTeamRequest**

Add `SprintsEnabled *bool` field to the update request DTO.

- [ ] **Step 3: Handle in repository Update method**

Ensure the update repository method includes `sprints_enabled` in the update map when provided.

- [ ] **Step 4: Set default based on team type during creation**

In the team creation service, set `SprintsEnabled` based on team type:
- `engineering` → `true`
- `custom` → `false`

- [ ] **Step 5: Commit**

---

### Task 2: Backend — Guard sprint creation

**Files:**
- Modify: `server/internal/service/pm_sprint.go`

- [ ] **Step 1: Check `sprints_enabled` before creating sprint**

In `Create()`, fetch the team and check `sprints_enabled`. Return error if disabled:
```go
if team != nil && team.SprintsEnabled != nil && !*team.SprintsEnabled {
    return nil, fmt.Errorf("sprints are disabled for this team")
}
```

- [ ] **Step 2: Check in sprint automation too**

In `pm_automation.go`, the sprint auto-create workflow should skip teams with sprints disabled.

- [ ] **Step 3: Commit**

---

### Task 3: Frontend — Add sprint settings to team settings page

**Files:**
- Create: `frontend/src/components/settings/teams/SprintSettingsForm.tsx`
- Modify: `frontend/src/components/settings/TeamsTab.tsx`

- [ ] **Step 1: Create SprintSettingsForm component**

Layout matching the plan:
```
┌────────────────────────────────────────────────┐
│ Sprints                                         │
│                                                 │
│ Sprints help create rhythm and focus for your   │
│ team over short, time-boxed windows.            │
│ Automations handle creating upcoming sprints,   │
│ rolling over unfinished work, and tracking      │
│ progress across cycles.                         │
│                                                 │
│ ┌─────────────────────────────────────────────┐ │
│ │ Enable sprints                     [toggle] │ │
│ └─────────────────────────────────────────────┘ │
│                                                 │
│ When enabled:                                   │
│ ┌─────────────────────────────────────────────┐ │
│ │ Each sprint lasts        [ 2 weeks     ▾ ]  │ │
│ │ Sprints start on         [ Monday      ▾ ]  │ │
│ │ Upcoming sprints         [ 2 sprints   ▾ ]  │ │
│ ├─────────────────────────────────────────────┤ │
│ │ Auto-create sprints               [toggle]  │ │
│ │ Automatically create new sprints to         │ │
│ │ maintain a pipeline of upcoming cycles.     │ │
│ │                                             │ │
│ │ Roll over unfinished work         [toggle]  │ │
│ │ When a sprint ends, move incomplete         │ │
│ │ stories to the next sprint.                 │ │
│ └─────────────────────────────────────────────┘ │
└────────────────────────────────────────────────┘
```

Props:
```tsx
interface SprintSettingsFormProps {
  teamId: string;
  workspaceId: string;
  sprintsEnabled: boolean;
  automations: PMAutomation[];
  onToggleSprints: (enabled: boolean) => void;
  onUpsertAutomation: (type: string, enabled: boolean, config: object) => void;
  onRemoveAutomation: (type: string) => void;
}
```

Dropdown options:
- Duration: 1 week, 2 weeks (default), 3 weeks, 4 weeks
- Start day: Monday–Sunday (default: Monday)
- Upcoming count: 1–5 sprints (default: 2)

Automation toggles:
- Auto-create: maps to `sprint_auto_create` automation type
- Roll over: maps to `sprint_move_unfinished` automation type

- [ ] **Step 2: Add to TeamsTab**

Add `SprintSettingsForm` to the team settings page, after the estimate settings section. Pass existing automation data and handlers.

- [ ] **Step 3: Verify toggle shows/hides sub-settings**

- [ ] **Step 4: Commit**

---

### Task 4: Frontend — Convert global Automations tab sprint section to overview

**Files:**
- Modify: `frontend/src/components/settings/AutomationsTab.tsx`

- [ ] **Step 1: Replace sprint automation card with read-only overview**

Replace the current interactive sprint automation card with a summary view:
```
┌──────────────────────────────────────────────────┐
│ Sprint Automations                                │
│ Configure sprint settings per team in             │
│ Team Settings.                                    │
│                                                   │
│ ┌──────────────────────────────────────────────┐  │
│ │ Engineering    Enabled · 2-week cycles       │  │
│ │                Auto-create ✓  Roll over ✓    │  │
│ │                                    [Edit →]  │  │
│ ├──────────────────────────────────────────────┤  │
│ │ Marketing      Sprints disabled    [Edit →]  │  │
│ ├──────────────────────────────────────────────┤  │
│ │ Design         Enabled · 1-week cycles       │  │
│ │                Auto-create ✓  Roll over ✗    │  │
│ │                                    [Edit →]  │  │
│ └──────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────┘
```

Each row shows:
- Team name
- Sprint status (enabled/disabled)
- If enabled: cycle duration + automation status badges
- "Edit →" link navigates to `/w/{slug}/settings/teams?team={teamId}`

- [ ] **Step 2: Keep epic automations section unchanged**

- [ ] **Step 3: Commit**

---

### Task 5: Frontend — Conditionally hide sprints in sidebar

**Files:**
- Modify: `frontend/src/components/layout/sidebar/ProjectsTeamsNav.tsx`
- Modify: `frontend/src/components/layout/sidebar/config.ts`

- [ ] **Step 1: Pass team data to sidebar**

The sidebar needs to know which teams have sprints enabled. The team data already flows through the sidebar (team list is loaded). Add `sprints_enabled` to the team data used by the sidebar.

- [ ] **Step 2: Filter `teamSubItems` based on sprints_enabled**

In `ProjectsTeamsNav.tsx`, when rendering sub-items for each team, filter out 'sprints' if `team.sprints_enabled === false`:
```tsx
const items = teamSubItems.filter(item =>
  item.key !== 'sprints' || team.sprints_enabled !== false
);
```

- [ ] **Step 3: Verify sidebar updates when toggle changes**

After toggling sprints in settings, the sidebar should reflect the change (may need query invalidation).

- [ ] **Step 4: Commit**

---

### Task 6: Frontend — Hide sprint selector in story create/edit when disabled

**Files:**
- Modify: `frontend/src/components/pm/CreateStoryModal.tsx`
- Modify: `frontend/src/components/pm/StoryDetailPanel.tsx`

- [ ] **Step 1: Hide sprint field in CreateStoryModal**

When the selected team has `sprints_enabled === false`, don't render the sprint selector (`GroupedSidebarPopoverSelect` for sprints).

- [ ] **Step 2: Hide sprint field in StoryDetailPanel**

Same logic — check team's `sprints_enabled` before rendering the sprint `MetadataRow`.

- [ ] **Step 3: Commit**

---

### Task 7: Set correct defaults for existing teams

**Files:**
- Modify: `server/internal/repository/workspace_team.go` or migration

- [ ] **Step 1: Migration for existing data**

The GORM default `true` applies to new rows. For existing teams:
- Engineering teams: already correct (`true`)
- Custom teams: need to be set to `false`

Add a startup migration or SQL:
```sql
UPDATE workspace_teams SET sprints_enabled = false WHERE team_type = 'custom' AND sprints_enabled = true;
```

Only run once. Can be done in AutoMigrate callback or a numbered migration.

- [ ] **Step 2: Verify existing teams have correct defaults**

- [ ] **Step 3: Commit**

---

### Task 8: Verification

- [ ] **Step 1: Test full flow**
  - Create engineering team → sprints enabled by default, sidebar shows Sprints
  - Create custom team → sprints disabled by default, sidebar hides Sprints
  - Enable sprints for custom team → sidebar shows Sprints, can create sprints
  - Disable sprints for engineering team → sidebar hides Sprints, sprint creation blocked
  - Existing sprints preserved when disabled
  - Sprint selector hidden in story create/edit when team has sprints disabled
  - Automations tab shows read-only overview with correct status per team
  - "Edit →" link navigates to correct team settings page

- [ ] **Step 2: Test automation settings**
  - Enable auto-create → verify sprints auto-created
  - Enable roll over → verify stories move on sprint end
  - Disable sprints → verify automations stop running for that team

- [ ] **Step 3: Commit any remaining fixes**
