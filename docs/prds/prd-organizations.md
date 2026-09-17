# PRD: Organizations

## Overview
Introduce an **Organization** layer above Workspaces. Organizations group workspaces under a single entity (company, team, department). Users belong to organizations and can access workspaces within them.

## Current State
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
