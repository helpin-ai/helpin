# Support task team preference implementation plan

This historical plan explains the original per-member team preference for creating
PM tasks from support conversations. Use the source notes below when changing the
current flow; the implementation checklist is retained as design history.

## Source review — 2026-09-18

- [Workspace preferences](../../server/internal/service/workspace.go) validate a
  nonempty team against the caller's workspace. The model, repository, `/me`
  response, and PATCH route contain the preference fields. They are also present
  in the versioned core foundation SQL; AutoMigrate is optional, not a universal
  installation mechanism.
- [MessageThread](../../frontend/src/components/support/MessageThread.tsx) first
  attempts PM task matching. An enabled review result can replace the legacy
  one-click path. In that legacy path, the dialog opens if either dismissal or
  a saved team is missing; it is not automatically a one-time dialog.
- Dialog confirmation creates the task **before** saving the preference. Those
  requests are separate: a preference failure does not roll back the task. The
  reviewed-draft path explicitly reports a preference-save failure after creation.
- [CreateTaskDialog](../../frontend/src/components/support/CreateTaskDialog.tsx)
  handles asynchronously loaded defaults without overwriting a user selection.
  The parent prefers the saved team, then the first membership; the dialog can
  fall back to the first available team.
- [CreateTaskFromConversation](../../server/internal/service/support_inbox.go)
  requires an explicit `team_id`. The proposed backend saved-preference/first-team
  fallback in Task 11 is not implemented. The historical snippets and build/push
  commands below are not current instructions or proof of passing checks.

## Original implementation plan

**Goal:** When creating a task from a support conversation, show a one-time info dialog with team picker, save the team preference per-user on the backend, and use it for all subsequent one-click task creation.

**Architecture:** Add two fields to `WorkspaceMember` model (`support_default_team_id`, `support_task_dialog_dismissed`). Add a PATCH endpoint to update these preferences. Frontend shows a dialog on first "Create Task" click with info text + team picker. After dismissal, subsequent clicks use the saved preference silently. The dialog pre-selects the user's first team from their `team_memberships`.

**Tech Stack:** Go (GORM model + handler + service + repository), React (Dialog component, TanStack Query mutation)

---

### Task 1: Add preference fields to WorkspaceMember model

**Files:**
- Modify: `server/internal/model/workspace.go:31-44`

- [ ] **Step 1: Add fields to WorkspaceMember struct**

Add two fields after `AcceptedAt`:

```go
SupportDefaultTeamID      *string `json:"support_default_team_id,omitempty" gorm:"type:uuid"`
SupportTaskDialogDismissed bool   `json:"support_task_dialog_dismissed" gorm:"default:false"`
```

GORM AutoMigrate will add these columns on next startup.

- [ ] **Step 2: Verify build**

Run: `cd server && go build ./...`
Expected: clean build

- [ ] **Step 3: Commit**

```bash
git add server/internal/model/workspace.go
git commit -m "feat: add support task preference fields to WorkspaceMember"
```

---

### Task 2: Add repository method to update support preferences

**Files:**
- Modify: `server/internal/repository/workspace.go`

- [ ] **Step 1: Add UpdateSupportTaskPreferences method**

```go
// UpdateSupportTaskPreferences updates the support task creation preferences for a workspace member.
func (r *WorkspaceRepository) UpdateSupportTaskPreferences(ctx context.Context, memberID string, teamID *string, dialogDismissed *bool) error {
	updates := map[string]interface{}{}
	if teamID != nil {
		updates["support_default_team_id"] = teamID
	}
	if dialogDismissed != nil {
		updates["support_task_dialog_dismissed"] = *dialogDismissed
	}
	if len(updates) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Model(&model.WorkspaceMember{}).Where("id = ?", memberID).Updates(updates).Error
}
```

- [ ] **Step 2: Verify build**

Run: `cd server && go build ./...`
Expected: clean build

- [ ] **Step 3: Commit**

```bash
git add server/internal/repository/workspace.go
git commit -m "feat: add repository method for support task preferences"
```

---

### Task 3: Add request model and service method

**Files:**
- Modify: `server/internal/model/workspace.go` (add request DTO)
- Modify: `server/internal/service/workspace.go`

- [ ] **Step 1: Add request DTO to model file**

Add after the existing workspace request/response DTOs:

```go
// UpdateSupportTaskPreferencesRequest is the payload for updating support task creation preferences.
type UpdateSupportTaskPreferencesRequest struct {
	SupportDefaultTeamID      *string `json:"support_default_team_id"`
	SupportTaskDialogDismissed *bool  `json:"support_task_dialog_dismissed"`
}
```

- [ ] **Step 2: Add service method**

```go
// UpdateSupportTaskPreferences updates support task preferences for the calling member.
func (s *WorkspaceService) UpdateSupportTaskPreferences(ctx context.Context, workspaceID, memberID string, req model.UpdateSupportTaskPreferencesRequest) error {
	return s.workspaceRepo.UpdateSupportTaskPreferences(ctx, memberID, req.SupportDefaultTeamID, req.SupportTaskDialogDismissed)
}
```

- [ ] **Step 3: Verify build**

Run: `cd server && go build ./...`
Expected: clean build

- [ ] **Step 4: Commit**

```bash
git add server/internal/model/workspace.go server/internal/service/workspace.go
git commit -m "feat: add service layer for support task preferences"
```

---

### Task 4: Add handler endpoint and route

**Files:**
- Modify: `server/internal/handler/workspace.go`
- Modify: `server/internal/router/router.go`

- [ ] **Step 1: Add handler method**

```go
// UpdateSupportTaskPreferences updates the caller's support task creation preferences.
func (h *WorkspaceHandler) UpdateSupportTaskPreferences(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "id")
	actor := authorization.ActorFromContext(r.Context())
	if actor == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req model.UpdateSupportTaskPreferencesRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.workspaceService.UpdateSupportTaskPreferences(r.Context(), workspaceID, actor.WorkspaceMemberID, req); err != nil {
		slog.ErrorContext(r.Context(), "update support task preferences", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update preferences")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
```

- [ ] **Step 2: Add route**

In the workspace routes section of `router.go`, add under the existing workspace member routes (inside the `r.Route("/{id}", ...)` group that has `wsAccess` middleware):

```go
r.Patch("/me/support-task-preferences", h.Workspace.UpdateSupportTaskPreferences)
```

- [ ] **Step 3: Verify build**

Run: `cd server && go build ./...`
Expected: clean build

- [ ] **Step 4: Commit**

```bash
git add server/internal/handler/workspace.go server/internal/router/router.go
git commit -m "feat: add endpoint for support task preferences"
```

---

### Task 5: Include preference fields in /me response

**Files:**
- Modify: `server/internal/handler/workspace.go` (GetMe handler, around line 266)

- [ ] **Step 1: Check if preference fields are already included**

The `/me` endpoint returns `WorkspaceAccess` which includes `membership`. Since `WorkspaceMember` already has the new fields with `json` tags, they should be included automatically if the membership object is the full `WorkspaceMember`. Verify this by reading the GetMe handler — if it constructs a custom response object, add the two fields to it.

If the response uses a subset struct, add:
```go
SupportDefaultTeamID      *string `json:"support_default_team_id,omitempty"`
SupportTaskDialogDismissed bool   `json:"support_task_dialog_dismissed"`
```

- [ ] **Step 2: Verify build**

Run: `cd server && go build ./...`
Expected: clean build

- [ ] **Step 3: Commit (if changes needed)**

```bash
git add server/internal/handler/workspace.go
git commit -m "feat: include support task preferences in /me response"
```

---

### Task 6: Add frontend types and API service

**Files:**
- Modify: `frontend/src/lib/types.ts` (WorkspaceAccess.membership)
- Modify: `frontend/src/lib/services/workspacesService.ts`

- [ ] **Step 1: Update WorkspaceAccess membership type**

Add to the `membership` object in `WorkspaceAccess`:

```typescript
support_default_team_id?: string;
support_task_dialog_dismissed: boolean;
```

- [ ] **Step 2: Add API method to workspacesService**

```typescript
updateSupportTaskPreferences: (workspaceId: string, data: {
  support_default_team_id?: string;
  support_task_dialog_dismissed?: boolean;
}) => api.patch<{ status: string }>(`/workspaces/${workspaceId}/me/support-task-preferences`, data),
```

- [ ] **Step 3: Commit**

```bash
git add frontend/src/lib/types.ts frontend/src/lib/services/workspacesService.ts
git commit -m "feat: add frontend types and service for support task preferences"
```

---

### Task 7: Add mutation hook for preferences

**Files:**
- Modify: `frontend/src/hooks/queries/useSession.ts` (or create alongside existing hooks)

- [ ] **Step 1: Add useUpdateSupportTaskPreferences mutation**

```typescript
export function useUpdateSupportTaskPreferences(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (data: {
      support_default_team_id?: string;
      support_task_dialog_dismissed?: boolean;
    }) => workspacesService.updateSupportTaskPreferences(workspaceId, data).then(unwrap),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.workspaces.access(workspaceId) });
    },
  });
}
```

- [ ] **Step 2: Export from barrel**

Add `useUpdateSupportTaskPreferences` to `frontend/src/hooks/queries/index.ts` exports.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/hooks/queries/useSession.ts frontend/src/hooks/queries/index.ts
git commit -m "feat: add mutation hook for support task preferences"
```

---

### Task 8: Create the support task info dialog component

**Files:**
- Create: `frontend/src/components/support/CreateTaskDialog.tsx`

- [ ] **Step 1: Create CreateTaskDialog component**

A dialog that shows:
- Info text explaining AI will generate title, description, type, and priority
- Team picker dropdown (pre-selected with user's first team from `team_memberships`)
- "Don't show this again" checkbox
- "Create Task" button

```typescript
import { useState } from 'react'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { SparkleIcon } from '@/lib/icons'
import type { PMTeam } from '@/lib/pmTypes'

interface CreateTaskDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  teams: PMTeam[]
  defaultTeamId?: string
  isPending: boolean
  onConfirm: (teamId: string, dismissDialog: boolean) => void
}

export function CreateTaskDialog({
  open,
  onOpenChange,
  teams,
  defaultTeamId,
  isPending,
  onConfirm,
}: CreateTaskDialogProps) {
  const [selectedTeamId, setSelectedTeamId] = useState(defaultTeamId ?? teams[0]?.id ?? '')
  const [dontShowAgain, setDontShowAgain] = useState(false)

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Create Task from Conversation</DialogTitle>
          <DialogDescription className="space-y-2 pt-2">
            <span className="flex items-start gap-2 text-sm">
              <SparkleIcon className="h-4 w-4 mt-0.5 shrink-0 text-primary" />
              <span>AI will automatically generate the task title, description, type, and priority from this conversation.</span>
            </span>
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-3 py-2">
          <div className="space-y-1.5">
            <label className="text-sm font-medium">Assign to team</label>
            <Select value={selectedTeamId} onValueChange={setSelectedTeamId}>
              <SelectTrigger>
                <SelectValue placeholder="Select a team" />
              </SelectTrigger>
              <SelectContent>
                {teams.map((team) => (
                  <SelectItem key={team.id} value={team.id}>
                    {team.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        </div>

        <DialogFooter className="flex items-center justify-between sm:justify-between">
          <label className="flex items-center gap-2 text-sm text-muted-foreground cursor-pointer">
            <Checkbox
              checked={dontShowAgain}
              onCheckedChange={(checked) => setDontShowAgain(checked === true)}
            />
            Don't show this again
          </label>
          <Button
            size="sm"
            disabled={!selectedTeamId || isPending}
            onClick={() => onConfirm(selectedTeamId, dontShowAgain)}
          >
            {isPending ? 'Creating...' : 'Create Task'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/components/support/CreateTaskDialog.tsx
git commit -m "feat: add CreateTaskDialog component for support task creation"
```

---

### Task 9: Integrate dialog into MessageThread

**Files:**
- Modify: `frontend/src/components/support/MessageThread.tsx`

- [ ] **Step 1: Update imports and add state**

Add imports for:
- `CreateTaskDialog` from `./CreateTaskDialog`
- `useUpdateSupportTaskPreferences` from hooks
- `useTeams` (or equivalent) to fetch teams list
- `useWorkspaceAccess` to read current preferences

Add state:
```typescript
const [showCreateTaskDialog, setShowCreateTaskDialog] = useState(false)
```

- [ ] **Step 2: Update handleCreateTask logic**

Replace the current `handleCreateTask`:

```typescript
const { data: access } = useWorkspaceAccess(workspaceId)
const { data: teams } = useTeams(workspaceId)
const updatePreferences = useUpdateSupportTaskPreferences(workspaceId)

const handleCreateTask = async () => {
  if (!conversationId || !workspaceSlug) return

  const dismissed = access?.membership?.support_task_dialog_dismissed
  const savedTeamId = access?.membership?.support_default_team_id

  if (!dismissed) {
    // First time — show info dialog with team picker
    setShowCreateTaskDialog(true)
    return
  }

  // Subsequent times — one-click create with saved team
  const created = await createTaskFromConversation.mutateAsync({
    conversationId,
    teamId: savedTeamId,
  })
  toast.success(`Created ${created.task_key ?? 'task'}`, {
    description: created.summary || created.task_name,
  })
  openTaskRoute(navigate as never, location as never, workspaceSlug, created.task_id)
}

const handleCreateTaskConfirm = async (teamId: string, dismissDialog: boolean) => {
  if (!conversationId || !workspaceSlug) return

  // Save preferences
  await updatePreferences.mutateAsync({
    support_default_team_id: teamId,
    support_task_dialog_dismissed: dismissDialog,
  })

  // Create the task
  const created = await createTaskFromConversation.mutateAsync({
    conversationId,
    teamId,
  })
  setShowCreateTaskDialog(false)
  toast.success(`Created ${created.task_key ?? 'task'}`, {
    description: created.summary || created.task_name,
  })
  openTaskRoute(navigate as never, location as never, workspaceSlug, created.task_id)
}
```

- [ ] **Step 3: Add dialog to JSX**

Add before the closing fragment or at end of the component's return:

```tsx
<CreateTaskDialog
  open={showCreateTaskDialog}
  onOpenChange={setShowCreateTaskDialog}
  teams={teams ?? []}
  defaultTeamId={access?.membership?.support_default_team_id ?? access?.team_memberships?.[0]?.team_id}
  isPending={createTaskFromConversation.isPending}
  onConfirm={handleCreateTaskConfirm}
/>
```

- [ ] **Step 4: Commit**

```bash
git add frontend/src/components/support/MessageThread.tsx
git commit -m "feat: integrate CreateTaskDialog into support message thread"
```

---

### Task 10: Update mutation to pass teamId

**Files:**
- Modify: `frontend/src/hooks/queries/useSupport.ts` (useCreateTaskFromConversation)
- Modify: `frontend/src/lib/services/supportService.ts`

- [ ] **Step 1: Update mutation to accept teamId**

Change `useCreateTaskFromConversation` mutation function signature:

```typescript
mutationFn: ({ conversationId, teamId }: { conversationId: string; teamId?: string }) =>
  supportService.createTaskFromConversation(workspaceId, conversationId, {
    team_id: teamId,
  }).then(unwrap),
```

- [ ] **Step 2: Verify supportService already accepts payload**

The existing `supportService.createTaskFromConversation` already accepts an optional `payload: CreateTaskFromConversationRequest = {}` — just ensure the type includes `team_id?: string`.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/hooks/queries/useSupport.ts frontend/src/lib/services/supportService.ts
git commit -m "feat: pass team_id when creating task from conversation"
```

---

### Task 11: Backend — use saved preference as fallback team

**Files:**
- Modify: `server/internal/service/support_inbox.go`

- [ ] **Step 1: Add fallback logic in CreateTaskFromConversation**

After the existing team assignment line (`TeamID: trimOptionalPtr(req.TeamID)`), add fallback logic: if no team was provided in the request, look up the actor's saved preference. If no preference, use the actor's first team membership.

```go
teamID := trimOptionalPtr(req.TeamID)
if teamID == nil {
	// Fallback: use actor's saved support team preference
	member, memberErr := s.workspaceRepo.FindMemberByID(ctx, actorID)
	if memberErr == nil && member != nil && member.SupportDefaultTeamID != nil {
		teamID = member.SupportDefaultTeamID
	}
}
```

Update `createReq.TeamID = teamID`.

Note: `actorID` here is the workspace member ID passed from the handler. Verify the correct lookup method exists or use the appropriate one.

- [ ] **Step 2: Verify build**

Run: `cd server && go build ./...`
Expected: clean build

- [ ] **Step 3: Commit**

```bash
git add server/internal/service/support_inbox.go
git commit -m "feat: fallback to saved team preference when creating task from conversation"
```

---

### Task 12: End-to-end verification

- [ ] **Step 1: Build backend**

Run: `cd server && go build ./...`

- [ ] **Step 2: Build frontend**

Run: `cd frontend && npx tsc -b`

- [ ] **Step 3: Final commit and push**

```bash
git push origin waqar-work
```
