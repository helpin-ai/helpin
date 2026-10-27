import { TeamLabel } from '@/components/workspace/TeamLabel';
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
import { useCreateDocsSpace, useDocsHelpcenterLocales, useUpdateDocsSpace } from '@/hooks/queries'
import { useWorkspaceTeams } from '@/hooks/useWorkspaceTeams'
import type { DocsSpace, HelpcenterAudience, SpaceType } from '@/lib/docsTypes'
import { showAutoTranslateToast } from '@/lib/autoTranslateEntity'
import { toast } from 'sonner'

type TeamAccessMode = 'all_teams' | 'specific_teams'

interface SpaceDialogProps {
  wsId: string
  open: boolean
  onOpenChange: (open: boolean) => void
  /** When provided, dialog operates in edit mode */
  space?: DocsSpace | null
  /** Default type for create mode */
  defaultType?: 'internal' | 'external_capable'
}

export function SpaceDialog({ wsId, open, onOpenChange, space, defaultType }: SpaceDialogProps) {
  const isEdit = !!space

  const [name, setName] = useState('')
  const [teamAccessMode, setTeamAccessMode] = useState<TeamAccessMode>('all_teams')
  const [type, setType] = useState<SpaceType>('internal')
  const [audience, setAudience] = useState<HelpcenterAudience>('public')
  const [selectedTeamIds, setSelectedTeamIds] = useState<string[]>([])

  const { teams } = useWorkspaceTeams(wsId)
  const createSpace = useCreateDocsSpace(wsId)
  const updateSpace = useUpdateDocsSpace(wsId)
  const { data: localesConfig } = useDocsHelpcenterLocales(wsId)
  const nonDefaultLocales = (localesConfig?.enabled_locales ?? []).filter(
    (l) => l !== (localesConfig?.default_locale ?? 'en'),
  )

  // Populate form when opening
  useEffect(() => {
    if (open && space) {
      setName(space.name)
      setType(space.type)
      setAudience(space.helpcenter_audience ?? 'public')
      const tids = space.team_ids ?? []
      setSelectedTeamIds(tids)
      setTeamAccessMode(space.visibility === 'team_only' && tids.length > 0 ? 'specific_teams' : 'all_teams')
    } else if (open && !space) {
      setName('')
      setTeamAccessMode('all_teams')
      setType(defaultType ?? 'internal')
      setAudience('public')
      setSelectedTeamIds([])
    }
  }, [open, space, defaultType])

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
          helpcenter_audience: audience,
          visibility: derivedVisibility,
          team_ids: derivedTeamIds,
          set_team_ids: true,
        })
        toast.success('Space updated')
      } else {
        const created = await createSpace.mutateAsync({
          name: name.trim(),
          visibility: derivedVisibility,
          type,
          helpcenter_audience: audience,
          team_ids: derivedTeamIds,
        })
        if (type === 'external_capable' && nonDefaultLocales.length > 0 && created?.id) {
          showAutoTranslateToast('Space created', wsId, 'space', created.id, nonDefaultLocales)
        } else {
          toast.success('Space created')
        }
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
            <div className="grid gap-2">
              <Label htmlFor="space-name">Name</Label>
              <Input
                id="space-name"
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="e.g. Engineering"
                autoFocus
              />
            </div>

            <div className="grid gap-2">
              <Label>Type</Label>
              <div className="grid grid-cols-2 gap-2">
                <button
                  type="button"
                  onClick={() => setType('internal')}
                  className={`rounded-lg border p-3 text-left transition-colors ${
                    type === 'internal'
                      ? 'border-primary bg-primary/5 ring-1 ring-primary/20'
                      : 'border-border/60 hover:border-border hover:bg-muted/30'
                  }`}
                >
                  <span className="text-sm font-medium">Internal</span>
                  <p className="mt-0.5 text-[11px] text-muted-foreground leading-snug">Only visible within your workspace</p>
                </button>
                <button
                  type="button"
                  onClick={() => setType('external_capable')}
                  className={`rounded-lg border p-3 text-left transition-colors ${
                    type === 'external_capable'
                      ? 'border-primary bg-primary/5 ring-1 ring-primary/20'
                      : 'border-border/60 hover:border-border hover:bg-muted/30'
                  }`}
                >
                  <span className="text-sm font-medium">External</span>
                  <p className="mt-0.5 text-[11px] text-muted-foreground leading-snug">Publishable to your help center</p>
                </button>
              </div>
            </div>

            {type === 'external_capable' && (
              <fieldset className="grid gap-2">
                <legend className="mb-2 text-sm font-medium leading-none">Readers</legend>
                <div className="grid grid-cols-2 gap-2">
                  {([
                    { value: 'public', label: 'Everyone', description: 'On your public help center, the chat widget and AI answers' },
                    { value: 'customers', label: 'Signed-in customers', description: 'Only customers signed in to the customer portal' },
                  ] as const).map((option) => (
                    <button
                      key={option.value}
                      type="button"
                      aria-pressed={audience === option.value}
                      onClick={() => setAudience(option.value)}
                      className={`rounded-lg border p-3 text-left transition-colors ${
                        audience === option.value
                          ? 'border-primary bg-primary/5 ring-1 ring-primary/20'
                          : 'border-border/60 hover:border-border hover:bg-muted/30'
                      }`}
                    >
                      <span className="text-sm font-medium">{option.label}</span>
                      <p className="mt-0.5 text-[11px] text-muted-foreground leading-snug">{option.description}</p>
                    </button>
                  ))}
                </div>
                {isEdit && space.type === 'external_capable' && audience !== (space.helpcenter_audience ?? 'public') && (
                  <p className="text-xs text-muted-foreground">
                    {audience === 'customers'
                      ? 'Published articles leave the public help center and appear in the customer portal when you save.'
                      : 'Published articles become public on your help center when you save.'}
                  </p>
                )}
              </fieldset>
            )}

            {isEdit && space.type === 'external_capable' && type === 'internal' && (
              <p className="text-xs text-muted-foreground">Published articles in this space are unpublished when you save.</p>
            )}

            {/* Team access */}
            <div className="grid gap-2">
              <Label>Team access</Label>
              <div className="grid grid-cols-2 gap-2">
                <button
                  type="button"
                  onClick={() => setTeamAccessMode('all_teams')}
                  className={`rounded-lg border p-3 text-left transition-colors ${
                    teamAccessMode === 'all_teams'
                      ? 'border-primary bg-primary/5 ring-1 ring-primary/20'
                      : 'border-border/60 hover:border-border hover:bg-muted/30'
                  }`}
                >
                  <span className="text-sm font-medium">All teams</span>
                  <p className="mt-0.5 text-[11px] text-muted-foreground leading-snug">Everyone in the workspace</p>
                </button>
                <button
                  type="button"
                  onClick={() => setTeamAccessMode('specific_teams')}
                  className={`rounded-lg border p-3 text-left transition-colors ${
                    teamAccessMode === 'specific_teams'
                      ? 'border-primary bg-primary/5 ring-1 ring-primary/20'
                      : 'border-border/60 hover:border-border hover:bg-muted/30'
                  }`}
                >
                  <span className="text-sm font-medium">Specific teams</span>
                  <p className="mt-0.5 text-[11px] text-muted-foreground leading-snug">Only selected teams can access</p>
                </button>
              </div>
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
                            <TeamLabel team={team} />
                          </button>
                        )
                      })}
                    </div>
                  )}
                </>
              )}
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
