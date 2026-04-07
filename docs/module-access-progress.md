# Module Access Progress Tracker

Last updated: 2026-04-07
Owner: Codex
Source PRD: `docs/PRD-module-access-control.md`

## Scope

V1 scope for this implementation:

- CRM module access
- Support module access
- Access Page in workspace settings
- Accessible modules returned from `/api/workspaces/{id}/me`

Out of scope for this implementation pass:

- PM explicit module grants
- Docs explicit module grants
- CRM record-level scoping
- Reworking Support mailbox access rules beyond integrating module access

## Progress Summary

- [x] Review refined PRD and existing code paths
- [x] Add backend module grant model and storage
- [x] Add backend module resolution to workspace access payload
- [x] Add backend grant management APIs for Access Page
- [x] Add backend module route guards for CRM and Support
- [x] Update frontend access types and consumers
- [x] Replace frontend CRM/Support visibility flags with real access data
- [x] Add Access Page UI in Settings
- [x] Add targeted tests and verification

## Backend

### Data model and auth primitives

- [x] Add code-defined module constants for `crm` and `support`
- [x] Add `workspace_module_grants` model
- [x] Add repository methods for listing, creating, deleting, and resolving grants
- [x] Add `module_access.manage` permission constant
- [x] Update RBAC role matrix for the new permission

### Access resolution

- [x] Add module access service or resolver
- [x] Return accessible modules from `/api/workspaces/{id}/me`
- [x] Keep PM and Docs broadly available in v1
- [x] Ensure owner/admin bypass is enforced

### Settings APIs

- [x] Add settings handler endpoints for module access read/write
- [x] Gate Access Page APIs behind `module_access.manage`
- [x] Return module grants using workspace-scoped IDs

### Route enforcement

- [x] Add `RequireModuleAccess` middleware
- [x] Apply module middleware to CRM route group
- [x] Apply module middleware to Support route group
- [x] Preserve Support mailbox/inbox checks as second layer

## Frontend

### Access model consumers

- [x] Extend `WorkspaceAccess` with accessible module list
- [x] Add frontend helpers for module checks
- [x] Update sidebar visibility to use real module access
- [x] Remove CRM/Support dependence on temporary email feature flags

### Access Page

- [x] Add `access` settings section metadata
- [x] Add Access Page route rendering
- [x] Add settings service methods for module grant CRUD
- [x] Add query and mutation hooks for module grants
- [x] Build team grant and direct member grant management UI
- [x] Invalidate settings and workspace access queries after grant changes

## Verification

- [x] Backend tests for module resolution
- [x] Backend tests for route guard behavior
- [x] Frontend type/build verification
- [x] Manual sanity check for sidebar and direct-route behavior

## Notes

- Direct grants should use `workspace_member.id`, not `users.id`.
- Team grants should use `workspace_teams.id`.
- Team membership changes should take effect on the next request because actor resolution is already request-time.
- `go test ./internal/authorization/...` passes.
- `npm run build` in `frontend/` passes.
- `go test ./internal/authorization/... ./internal/service/...` still reports unrelated pre-existing failures in the broader service suite.
