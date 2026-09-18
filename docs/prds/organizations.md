# Organization requirements

> Historical requirements, reviewed against the checkout on 2026-09-17.
> Organizations are implemented. This page preserves the original proposal for
> contributors; its former current-state section and role table are not the
> current authorization contract.

## Current implementation and differences

The [organization model](../../server/internal/model/organization.go),
[service](../../server/internal/service/organization.go), and
[API routes](../../server/internal/router/router.go) implement organizations,
membership, updates, deletion, and a separate ownership-transfer endpoint.
Assignable member roles include `admin`, `member`, and `viewer`; ownership is
transferred separately. Both owners and admins can update organization details,
contrary to the original role table's admin “Manage Org: No” cell.

Organization roles do not automatically grant access to every workspace.
[Workspace listing](../../server/internal/repository/workspace.go) joins active
workspace memberships, and [authorization membership lookup](../../server/internal/authorization/member_repo.go)
reads `workspace_members`. The original “All” workspace-access cells are therefore
not an implemented organization-role bypass.

The [workspace creation service](../../server/internal/service/workspace.go)
accepts an optional organization ID and passes it to persistence. Its current
handler/service/repository path does not enforce the proposed organization
owner/admin restriction before linking the workspace. The
[workspace model](../../server/internal/model/workspace.go) also retains a
nullable organization ID. These are gaps relative to the original requirements,
not guarantees supplied by the PRD; this documentation review does not change
runtime authorization.

[Signup](../../server/internal/service/auth.go) attempts to create a default
organization. The [workspace page](../../frontend/src/pages/Workspaces.tsx)
selects the remembered organization or the first available one, and includes a
fallback creation flow when none exists. This has superseded the proposed
mandatory organization-creation screen. The store persists the selected ID,
rather than the entire organization object.

The [startup backfill](../../server/internal/repository/org_migration.go) still
handles workspaces with no organization: it creates a uniquely named slug,
copies workspace members (owner remains owner; other roles become member), and
links each workspace inside a transaction. Do not assume that every installation
runs GORM AutoMigrate; Community uses versioned SQL for its schema. Current
[Enterprise routes](../../server/ee/handler/routes.go) also include organization
billing, so the original billing exclusion is historical scope rather than a
statement about the whole product.

## Original proposal

## Overview
Introduce an **Organization** layer above Workspaces. Organizations group workspaces under a single entity (company, team, department). Users belong to organizations and can access workspaces within them.

## State before implementation
- **Workspace** is the top-level container for all data
- Users directly create/join workspaces via `workspace_members`
- No organizational grouping exists

## Proposed Architecture
```
User -> OrganizationMember -> Organization -> Workspace -> (all domain data)
```

## Data Model

### Organization
| Column | Type | Notes |
|--------|------|-------|
| id | UUID | PK, auto-generated |
| name | VARCHAR | NOT NULL |
| slug | VARCHAR | UNIQUE, NOT NULL |
| owner_id | UUID | FK to users, NOT NULL |
| logo_url | VARCHAR | nullable |
| created_at | TIMESTAMP | auto |
| updated_at | TIMESTAMP | auto |

### OrganizationMember
| Column | Type | Notes |
|--------|------|-------|
| id | UUID | PK, auto-generated |
| organization_id | UUID | FK to organizations, NOT NULL |
| user_id | UUID | FK to users, NOT NULL |
| role | VARCHAR | 'owner' / 'admin' / 'member', NOT NULL |
| created_at | TIMESTAMP | auto |
| updated_at | TIMESTAMP | auto |
| UNIQUE | (organization_id, user_id) | |

### Workspace (updated)
| Column | Type | Notes |
|--------|------|-------|
| organization_id | UUID | FK to organizations, nullable for migration, then required |

## Roles
| Role | Manage Org | Invite Users | Create Workspaces | Access Workspaces |
|------|-----------|-------------|-------------------|-------------------|
| owner | Yes | Yes | Yes | All |
| admin | No | Yes | Yes | All |
| member | No | No | No | Assigned only |

## API Endpoints

### Organizations
- `GET /api/organizations` — list user's organizations
- `POST /api/organizations` — create organization
- `GET /api/organizations/{id}` — get organization details
- `PUT /api/organizations/{id}` — update organization (owner/admin)
- `DELETE /api/organizations/{id}` — delete organization (owner only)
- `GET /api/organizations/{id}/members` — list members
- `POST /api/organizations/{id}/members` — add member (owner/admin)
- `PUT /api/organizations/{id}/members/{userId}` — update member role
- `DELETE /api/organizations/{id}/members/{userId}` — remove member

### Workspaces (updated)
- `POST /api/workspaces` — now requires `organization_id` in body
- `GET /api/workspaces` — now accepts optional `?organization_id=` filter

## Frontend Flow
1. After login, load user's organizations
2. If 0 orgs: show "Create Organization" page
3. If 1 org: auto-select, show workspaces within it
4. If multiple orgs: show org selector, then workspaces
5. Org context stored in Zustand store, persisted to localStorage
6. Workspace slugs remain globally unique — URL structure unchanged (`/w/{slug}`)

## Migration Strategy
- For each existing workspace, create an organization named after the workspace
- Set workspace owner as org owner
- Add all workspace members as org members
- Set `organization_id` on the workspace
- This runs via GORM AutoMigrate + a one-time data migration in main.go

## Out of Scope (Future)
- Organization-level billing
- Organization-level settings
- Cross-organization workspace sharing
- Organization invitations (separate from workspace invitations)
