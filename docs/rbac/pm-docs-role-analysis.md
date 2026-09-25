# Historical PM and Docs role analysis

This page preserves an earlier role analysis for design context. It is not the
current permission matrix. Source comparison on 2026-09-17 found these differences:

- Members can create and edit epics in their own teams through
  `requireCanEditTeamEpics`; team ownership is not required for those operations.
  See [epic service](../../server/internal/service/pm_epic.go),
  [team access helpers](../../server/internal/service/pm_team_access.go), and
  [permission tests](../../server/internal/service/pm_team_access_test.go).
- The document list service filters drafts to the current user for non-admins
  and non-owners. The team-manager draft visibility described below is not
  implemented by that list filter. See
  [document listing](../../server/internal/service/docs_document.go).
- Current PM code uses tasks rather than stories. Role grants are only one layer:
  module access and resource-specific checks also apply. Start with the
  [authorization service](../../server/internal/authorization/authz.go) and
  [role grants](../../server/internal/authorization/rbac.go) when evaluating access.

The sections below retain the original analysis and recommendations; do not use
them as acceptance criteria without checking the current route and service.


## PM Module — Role Capabilities

### Viewer
- Can view stories, epics, sprints in their own teams only
- Can view objectives across all teams (strategic visibility)
- Cannot create, edit, or delete any PM entity
- Cannot manage workflows, labels, automations
- Cannot import data
- Cannot see entities with no team assigned (NULL team_id)
- If not assigned to any team: sees nothing except objectives

### Member
- Can view stories, epics, sprints in their own teams only
- Can view objectives across all teams (strategic visibility)
- Can create and edit stories in their own teams
- Can create and delete comments
- Cannot create or edit epics (requires team manager)
- Cannot create or edit sprints (requires team manager)
- Cannot create or edit objectives (requires team manager)
- Cannot manage workflows, labels, automations
- Cannot import data
- Cannot see entities with no team assigned (NULL team_id)
- If not assigned to any team: sees nothing except objectives

### Team Manager
A team manager is a workspace member with role "owner" in the team_memberships table for a specific team. Their workspace role remains "member" but they gain planning capabilities for their team.

- Everything a Member can do, plus:
- Can create and edit epics in their own teams
- Can create and edit sprints in their own teams
- Can create and edit objectives for their own teams
- Can manage key results on objectives for their own teams
- Can manage epic health, labels, and associations for their own teams
- Can manage objective owners, teams, and epic links for their own teams
- Can edit team name and settings for their own team
- Can manage team estimate settings (scale, allow zero, etc.) for their own team
- Can manage team field visibility settings for their own team
- Can manage team repository defaults for their own team
- Can add and remove members from their own team
- Can add and remove invitation preassignments for their own team

### Admin
- Can view all stories, epics, sprints, objectives across all teams
- Can create and edit stories, epics, sprints, objectives in any team
- Can manage workflows and states
- Can manage labels
- Can manage automations
- Can import data
- Can see entities with no team assigned (NULL team_id)
- Full access regardless of team membership

### Owner
- Same as Admin for PM module
- Can delete workspace (additional privilege outside PM)


## Docs Module — Role Capabilities

### Viewer
- Can view workspace-wide spaces
- Can view team-only spaces if they belong to at least one of the space's teams
- Can read published documents in accessible spaces
- Can read their own draft documents
- Cannot see other users' drafts
- Can search docs
- Can read Help Center config
- Cannot create, edit, or delete any docs entity
- Cannot publish documents

### Member
- Everything Viewer can do, plus:
- Can create spaces
- Can edit space metadata (name, description, visibility)
- Cannot delete or restore spaces
- Can create, edit, delete, and restore collections
- Can create, edit, delete, archive, unarchive, and move documents
- Can publish and unpublish documents internally (transition to published status)
- Can publish and unpublish documents to Help Center externally
- Can toggle document sharing and locking
- Can save document content (rich text and markdown)
- Can submit article feedback
- Can create document versions

### Team Manager
- Everything Member can do, plus:
- Can view published docs in accessible spaces
- Can view their own drafts
- Can view other members' drafts in spaces owned by / accessible to their managed teams
- Cannot view drafts in spaces unrelated to their team
- Cannot view drafts in private or restricted spaces outside their authority

### Admin
- Everything Member can do, plus:
- Can delete and restore spaces (soft delete)
- Can manage Help Center config (update settings, upload assets)
- Can view all spaces regardless of team-only visibility
- Can view all drafts regardless of author

### Owner
- Same as Admin for Docs module

### Team-Only vs Workspace-Wide Spaces

Spaces have a visibility setting: workspace_wide or team_only.

- workspace_wide: All workspace members with docs.read can view the space
- team_only: Only members of the space's associated teams can view it
- Admin/Owner: Always see all spaces regardless of visibility setting
- Member/Viewer: Must be in at least one of the space's teams to see team_only spaces

Draft documents are visible to:
- The document creator (always)
- Team managers for spaces accessible to their managed teams
- Admins and owners (all drafts regardless of space)


## How Team Manager Permission Works

### PM Entity Operations (requireCanManage)

For planning entities (epics, sprints, objectives), the service layer uses `requireCanManage` / `requireCanManageTeams`:
1. Admin/Owner → allow (any team)
2. Viewer → deny
3. Member + team owner for the target team → allow
4. Member without team ownership → deny

For execution entities (stories), the service layer uses `requireCanEdit`:
1. Viewer → deny
2. Member+ → allow (within own teams)

### Team Settings Operations (RequireTeamPermission middleware)

The RequireTeamPermission middleware in the router checks:
1. Does the user have the team.manage permission? (workspace admin/owner) → allow
2. Is the user a team owner for this specific team? → allow
3. Otherwise → deny

### What Team Managers CANNOT Do
- Delete teams (requires workspace admin)
- Create new teams (requires workspace admin)
- Manage workflows, labels, or automations (requires workspace admin)
- Access other teams' settings or members
- See PM entities outside their own teams (except objectives)
- Delete epics or objectives (requires workspace admin)


## Summary Table

PM Actions:
  View stories/epics/sprints:  Viewer (own teams), Member (own teams), Team Manager (own teams), Admin (all), Owner (all)
  View objectives:             Viewer (all), Member (all), Team Manager (all), Admin (all), Owner (all)
  Create/edit stories:         Member (own teams), Team Manager (own teams), Admin (any team), Owner (any team)
  Create/edit epics:           Team Manager (own teams), Admin (any team), Owner (any team)
  Create/edit sprints:         Team Manager (own teams), Admin (any team), Owner (any team)
  Create/edit objectives:      Team Manager (own teams), Admin (any team), Owner (any team)
  Delete epics/objectives:     Admin, Owner
  Delete sprints:              Team Manager (own teams), Admin (any team), Owner (any team)
  Manage workflows:            Admin, Owner
  Manage labels:               Admin, Owner
  Manage automations:          Admin, Owner
  Import data:                 Admin, Owner

Docs Actions:
  View spaces:                 Viewer+ (team-only filtered for non-admins)
  View own drafts:             Viewer, Member, Team Manager, Admin, Owner
  View others' drafts:         Team Manager (own team spaces), Admin (all), Owner (all)
  Create/edit spaces:          Member, Team Manager, Admin, Owner
  Delete spaces:               Admin, Owner
  Create/edit docs:            Member, Team Manager, Admin, Owner
  Publish internally:          Member, Team Manager, Admin, Owner
  Publish to Help Center:      Member, Team Manager, Admin, Owner
  Manage Help Center:          Admin, Owner

Team Management:
  Create teams:                Admin, Owner
  Delete teams:                Admin, Owner
  Manage own team settings:    Team Manager, Admin, Owner
  Add/remove team members:     Team Manager, Admin, Owner


## Known Issues & Recommended Improvements

### Team Manager lacks PM config rights
A team lead should be able to manage their team's workflows, labels, and templates without being workspace admin. This is how Linear and Plane handle team-level administration. Recommendation: let team managers manage workflows, labels, and templates scoped to their own team.

### No cross-team read-only access
Executives, analysts, and cross-functional stakeholders who need read-only visibility across teams must either be added to every team or made admin. Recommendation: consider a workspace-wide viewer option or strategic-read permission.
