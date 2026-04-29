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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { AiMagicIcon } from '@/lib/icons'
import type { WorkspaceTeam } from '@/lib/types'

interface CreateTaskDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  teams: WorkspaceTeam[]
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
  const [selectedTeamId, setSelectedTeamId] = useState(
    defaultTeamId ?? teams[0]?.id ?? '',
  )
  const [dontShowAgain, setDontShowAgain] = useState(false)

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Create Task from Conversation</DialogTitle>
          <DialogDescription asChild>
            <div className="space-y-2 pt-2">
              <span className="flex items-start gap-2 text-sm">
                <AiMagicIcon className="mt-0.5 h-4 w-4 shrink-0 text-primary" />
                <span>
                  AI will automatically generate the task title, description,
                  type, and priority from this conversation.
                </span>
              </span>
            </div>
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
          <label className="flex cursor-pointer items-center gap-2 text-sm text-muted-foreground">
            <Checkbox
              checked={dontShowAgain}
              onCheckedChange={(checked) =>
                setDontShowAgain(checked === true)
              }
            />
            Don&apos;t show this again
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
