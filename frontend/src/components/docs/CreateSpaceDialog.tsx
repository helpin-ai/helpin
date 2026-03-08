import { useState } from 'react'
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
import { useCreateDocsSpace } from '@/hooks/queries'
import type { SpaceVisibility, SpaceType } from '@/lib/docsTypes'
import { toast } from 'sonner'

interface CreateSpaceDialogProps {
  wsId: string
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function CreateSpaceDialog({ wsId, open, onOpenChange }: CreateSpaceDialogProps) {
  const [name, setName] = useState('')
  const [visibility, setVisibility] = useState<SpaceVisibility>('workspace_wide')
  const [type, setType] = useState<SpaceType>('internal')
  const [restrictToOwners, setRestrictToOwners] = useState(false)

  const createSpace = useCreateDocsSpace(wsId)

  const reset = () => {
    setName('')
    setVisibility('workspace_wide')
    setType('internal')
    setRestrictToOwners(false)
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!name.trim()) return

    try {
      await createSpace.mutateAsync({
        name: name.trim(),
        visibility,
        type,
        restrict_to_owners: restrictToOwners,
      })
      toast.success('Space created')
      reset()
      onOpenChange(false)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to create space')
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>Create Space</DialogTitle>
            <DialogDescription>
              Spaces organize your documentation by team or topic.
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
              <Label>Visibility</Label>
              <Select value={visibility} onValueChange={(v) => setVisibility(v as SpaceVisibility)}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="workspace_wide">Workspace wide</SelectItem>
                  <SelectItem value="team_only">Team only</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="grid gap-2">
              <Label>Type</Label>
              <Select value={type} onValueChange={(v) => setType(v as SpaceType)}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="internal">Internal</SelectItem>
                  <SelectItem value="external_capable">External capable</SelectItem>
                </SelectContent>
              </Select>
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
            <Button type="submit" disabled={!name.trim() || createSpace.isPending}>
              {createSpace.isPending ? 'Creating...' : 'Create Space'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
