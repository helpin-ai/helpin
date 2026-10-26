import { useState, type ReactNode } from 'react'
import { format } from 'date-fns'
import { Button } from '@/components/ui/button'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuLabel, DropdownMenuSeparator, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { snoozePresets } from '@/lib/supportSLA'

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

/** Snooze presets plus a custom date and time. */
export function SnoozeMenu({ trigger, onSnooze, disabled, open, onOpenChange }: SnoozeMenuProps) {
  const [customOpen, setCustomOpen] = useState(false)
  const [customValue, setCustomValue] = useState('')
  const [error, setError] = useState<string | null>(null)
  const presets = snoozePresets()

  const openCustom = () => {
    const tomorrow = new Date()
    tomorrow.setDate(tomorrow.getDate() + 1)
    tomorrow.setHours(9, 0, 0, 0)
    setCustomValue(toLocalInputValue(tomorrow))
    setError(null)
    setCustomOpen(true)
  }

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
    setCustomOpen(false)
    onSnooze(until)
  }

  return (
    <>
      <DropdownMenu open={open} onOpenChange={onOpenChange}>
        <DropdownMenuTrigger asChild disabled={disabled}>{trigger}</DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="w-60">
          <DropdownMenuLabel className="text-xs text-muted-foreground">Snooze until</DropdownMenuLabel>
          {presets.map((preset) => (
            <DropdownMenuItem key={preset.key} onSelect={() => onSnooze(preset.until)} className="justify-between gap-3">
              <span>{preset.label}</span>
              <span className="text-xs text-muted-foreground">{format(preset.until, preset.key === 'later' ? 'HH:mm' : 'EEE HH:mm')}</span>
            </DropdownMenuItem>
          ))}
          <DropdownMenuSeparator />
          <DropdownMenuItem onSelect={openCustom}>Pick date and time…</DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
      <Dialog open={customOpen} onOpenChange={setCustomOpen}>
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
            <Label htmlFor="snooze-until">Date and time</Label>
            <Input id="snooze-until" type="datetime-local" value={customValue} onChange={(event) => setCustomValue(event.target.value)} />
            {error ? <p role="alert" className="text-xs text-destructive">{error}</p> : null}
            <DialogFooter className="pt-2">
              <Button type="button" variant="ghost" onClick={() => setCustomOpen(false)}>Cancel</Button>
              <Button type="submit">Snooze</Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </>
  )
}
