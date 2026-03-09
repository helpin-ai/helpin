import { useEffect, useState } from 'react'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { useCreateDocsSpace, useUpdateDocsSpace } from '@/hooks/queries'
import { useWorkspaceTeams } from '@/hooks/useWorkspaceTeams'
import type { DocsSpace, SpaceType } from '@/lib/docsTypes'
import { toast } from 'sonner'

type TeamAccessMode = 'all_teams' | 'specific_teams'

interface SpaceDialogProps {
  wsId: string
  open: boolean
  onOpenChange: (open: boolean) => void
  /** When provided, dialog operates in edit mode */
  space?: DocsSpace | null
}

export function SpaceDialog({ wsId, open, onOpenChange, space }: SpaceDialogProps) {
  const isEdit = !!space

  const [name, setName] = useState('')
  const [teamAccessMode, setTeamAccessMode] = useState<TeamAccessMode>('all_teams')
  const [type, setType] = useState<SpaceType>('internal')
  const [restrictToOwners, setRestrictToOwners] = useState(false)
  const [selectedTeamIds, setSelectedTeamIds] = useState<string[]>([])

  const { teams } = useWorkspaceTeams(wsId)
  const createSpace = useCreateDocsSpace(wsId)
  const updateSpace = useUpdateDocsSpace(wsId)

  // Populate form when opening
  useEffect(() => {
    if (open && space) {
      setName(space.name)
      setType(space.type)
      setRestrictToOwners(space.restrict_to_owners)
      const tids = space.team_ids ?? []
      setSelectedTeamIds(tids)
      setTeamAccessMode(space.visibility === 'team_only' && tids.length > 0 ? 'specific_teams' : 'all_teams')
    } else if (open && !space) {
      setName('')
      setTeamAccessMode('all_teams')
      setType('internal')
      setRestrictToOwners(false)
      setSelectedTeamIds([])
    }
  }, [open, space])

  const toggleTeam = (teamId: string) => {
    setSelectedTeamIds((prev) =>
      prev.includes(teamId) ? prev.filter((id) => id !== teamId) : [...prev, teamId],
    )
  }

  // Derive visibility from team access mode
  const derivedVisibility = teamAccessMode === 'specific_teams' ? 'team_only' : 'workspace_wide'
  const derivedTeamIds = teamAccessMode === 'specific_teams' ? selectedTeamIds : []

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!name.trim()) return
    if (teamAccessMode === 'specific_teams' && selectedTeamIds.length === 0) {
      toast.error('Select at least one team for specific team access')
      return
    }

    try {
      if (isEdit) {
        await updateSpace.mutateAsync({
          id: space.id,
          name: name.trim(),
          type,
          visibility: derivedVisibility,
          restrict_to_owners: restrictToOwners,
          team_ids: derivedTeamIds,
          set_team_ids: true,
        })
        toast.success('Space updated')
      } else {
        await createSpace.mutateAsync({
          name: name.trim(),
          visibility: derivedVisibility,
          type,
          restrict_to_owners: restrictToOwners,
          team_ids: derivedTeamIds,
        })
        toast.success('Space created')
      }
      onOpenChange(false)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : `Failed to ${isEdit ? 'update' : 'create'} space`)
    }
  }

  const isPending = isEdit ? updateSpace.isPending : createSpace.isPending

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>{isEdit ? 'Edit Space' : 'Create Space'}</DialogTitle>
            <DialogDescription>
              {isEdit
                ? 'Update space settings and team access.'
                : 'Spaces organize your documentation by team or topic.'}
            </DialogDescription>
          </DialogHeader>

          <div className="grid gap-4 py-4">
            <div className="flex items-end gap-2">
              <div className="grid flex-1 gap-2">
                <Label htmlFor="space-name">Name</Label>
                <Input
                  id="space-name"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  placeholder="e.g. Engineering"
                  autoFocus
                />
              </div>
              <div className="grid w-[130px] shrink-0 gap-2">
                <Label>Type</Label>
                <Select value={type} onValueChange={(v) => setType(v as SpaceType)}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="internal">Internal</SelectItem>
                    <SelectItem value="external_capable">External</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </div>

            {/* Team access — merged visibility + team picker */}
            <div className="grid gap-2">
              <Label>Team access</Label>
              <Select value={teamAccessMode} onValueChange={(v) => setTeamAccessMode(v as TeamAccessMode)}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all_teams">All teams</SelectItem>
                  <SelectItem value="specific_teams">Specific teams</SelectItem>
                </SelectContent>
              </Select>
              {teamAccessMode === 'specific_teams' && (
                <>
                  <p className="text-xs text-muted-foreground">
                    Only members of selected teams can access this space.
                  </p>
                  {teams.length === 0 ? (
                    <p className="text-xs text-muted-foreground italic">No teams created yet.</p>
                  ) : (
                    <div className="flex flex-wrap gap-1.5 mt-1">
                      {teams.map((team) => {
                        const selected = selectedTeamIds.includes(team.id)
                        return (
                          <button
                            key={team.id}
                            type="button"
                            onClick={() => toggleTeam(team.id)}
                            className={`rounded-md border px-2.5 py-1 text-xs transition-colors ${
                              selected
                                ? 'border-primary bg-primary/10 text-primary font-medium'
                                : 'border-border text-muted-foreground hover:border-primary/50 hover:text-foreground'
                            }`}
                          >
                            {team.name}
                          </button>
                        )
                      })}
                    </div>
                  )}
                </>
              )}
            </div>

            <div className="flex items-center justify-between">
              <Label htmlFor="restrict-owners" className="text-sm">
                Restrict editing to owners
              </Label>
              <Switch
                id="restrict-owners"
                checked={restrictToOwners}
                onCheckedChange={setRestrictToOwners}
              />
            </div>
          </div>

          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
              Cancel
            </Button>
            <Button type="submit" disabled={!name.trim() || isPending}>
              {isPending ? (isEdit ? 'Saving...' : 'Creating...') : isEdit ? 'Save Changes' : 'Create Space'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
