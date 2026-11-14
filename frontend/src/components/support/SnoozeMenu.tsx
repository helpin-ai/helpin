import { useId, useState, type ReactNode } from 'react'
import { format } from 'date-fns'
import { Button } from '@/components/ui/button'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuLabel, DropdownMenuSeparator, DropdownMenuSub, DropdownMenuSubContent, DropdownMenuSubTrigger, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { snoozePresets } from '@/lib/supportSLA'
import { Clock01Icon } from '@/lib/icons'

function toLocalInputValue(date: Date): string {
  return format(date, "yyyy-MM-dd'T'HH:mm")
}

interface SnoozeMenuProps {
  trigger: ReactNode
  onSnooze: (until: Date) => void
  disabled?: boolean
  open?: boolean
  onOpenChange?: (open: boolean) => void
}

interface SnoozeOptionsProps {
  onSnooze: (until: Date) => void
  disabled?: boolean
  onCustomSelect?: () => void
}

function SnoozeOptions({ onSnooze, disabled, onCustomSelect }: SnoozeOptionsProps) {
  return (
    <>
      {snoozePresets().map((preset) => (
        <DropdownMenuItem key={preset.key} disabled={disabled} onSelect={() => onSnooze(preset.until)} className="justify-between gap-3 text-[13px]">
          <span>{preset.label}</span>
          <span className="text-xs text-muted-foreground">{format(preset.until, preset.key === 'later' ? 'HH:mm' : 'EEE HH:mm')}</span>
        </DropdownMenuItem>
      ))}
      {onCustomSelect && <>
        <DropdownMenuSeparator />
        <DropdownMenuItem disabled={disabled} onSelect={onCustomSelect}>Pick date and time…</DropdownMenuItem>
      </>}
    </>
  )
}

/** Shared by the conversation and bulk-action overflow menus. */
export function SnoozeSubmenu(props: SnoozeOptionsProps) {
  return (
    <DropdownMenuSub>
      <DropdownMenuSubTrigger className="text-[13px]" disabled={props.disabled}>
        <Clock01Icon className="h-3.5 w-3.5" />Snooze
      </DropdownMenuSubTrigger>
      <DropdownMenuSubContent className="w-60">
        <SnoozeOptions {...props} />
      </DropdownMenuSubContent>
    </DropdownMenuSub>
  )
}

/** Mount outside the menu content so selecting a custom date can close the menu. */
export function SnoozeDateTimeDialog({ onSnooze, onClose }: { onSnooze: (until: Date) => void; onClose: () => void }) {
  const inputId = useId()
  const [customValue, setCustomValue] = useState(() => {
    const tomorrow = new Date()
    tomorrow.setDate(tomorrow.getDate() + 1)
    tomorrow.setHours(9, 0, 0, 0)
    return toLocalInputValue(tomorrow)
  })
  const [error, setError] = useState<string | null>(null)

  const submitCustom = () => {
    const until = new Date(customValue)
    if (Number.isNaN(until.getTime()) || until.getTime() < Date.now() + 60_000) {
      setError('Choose a time at least a minute from now.')
      return
    }
    if (until.getTime() > Date.now() + 365 * 24 * 60 * 60_000) {
      setError('Choose a time within the next year.')
      return
    }
    onClose()
    onSnooze(until)
  }

  return (
    <Dialog open onOpenChange={(open) => { if (!open) onClose() }}>
      <DialogContent className="sm:max-w-sm">
        <DialogHeader>
          <DialogTitle>Snooze until</DialogTitle>
          <DialogDescription>The conversation returns to the inbox at this time, or sooner if the customer replies.</DialogDescription>
        </DialogHeader>
        <form
          className="space-y-2"
          onSubmit={(event) => {
            event.preventDefault()
            submitCustom()
          }}
        >
          <Label htmlFor={inputId}>Date and time</Label>
          <Input id={inputId} type="datetime-local" value={customValue} onChange={(event) => setCustomValue(event.target.value)} />
          {error ? <p role="alert" className="text-xs text-destructive">{error}</p> : null}
          <DialogFooter className="pt-2">
            <Button type="button" variant="ghost" onClick={onClose}>Cancel</Button>
            <Button type="submit">Snooze</Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

/** Snooze presets plus a custom date and time. */
export function SnoozeMenu({ trigger, onSnooze, disabled, open, onOpenChange }: SnoozeMenuProps) {
  const [customOpen, setCustomOpen] = useState(false)

  return (
    <>
      <DropdownMenu open={open} onOpenChange={onOpenChange}>
        <DropdownMenuTrigger asChild disabled={disabled}>{trigger}</DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="w-60">
          <DropdownMenuLabel className="text-xs text-muted-foreground">Snooze until</DropdownMenuLabel>
          <SnoozeOptions onSnooze={onSnooze} disabled={disabled} onCustomSelect={() => setCustomOpen(true)} />
        </DropdownMenuContent>
      </DropdownMenu>
      {customOpen && <SnoozeDateTimeDialog onSnooze={onSnooze} onClose={() => setCustomOpen(false)} />}
    </>
  )
}
