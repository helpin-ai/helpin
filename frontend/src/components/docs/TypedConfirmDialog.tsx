import { useState } from 'react'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Input } from '@/components/ui/input'

interface TypedConfirmDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  title: string
  description: string
  confirmText: string
  onConfirm: () => void | Promise<void>
  destructive?: boolean
}

export function TypedConfirmDialog({
  open,
  onOpenChange,
  title,
  description,
  confirmText,
  onConfirm,
  destructive = true,
}: TypedConfirmDialogProps) {
  const [value, setValue] = useState('')
  const matches = value === confirmText

  const handleConfirm = async () => {
    if (!matches) return
    await onConfirm()
    setValue('')
    onOpenChange(false)
  }

  return (
    <AlertDialog open={open} onOpenChange={(v) => { onOpenChange(v); if (!v) setValue('') }}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{title}</AlertDialogTitle>
          <AlertDialogDescription>{description}</AlertDialogDescription>
        </AlertDialogHeader>
        <div className="space-y-2">
          <p className="text-sm text-muted-foreground">
            Type <span className="font-semibold text-foreground">{confirmText}</span> to confirm:
          </p>
          <Input
            value={value}
            onChange={(e) => setValue(e.target.value)}
            onKeyDown={(e) => { if (e.key === 'Enter' && matches) handleConfirm() }}
            placeholder={confirmText}
            autoFocus
          />
        </div>
        <AlertDialogFooter>
          <AlertDialogCancel>Cancel</AlertDialogCancel>
          <AlertDialogAction
            variant={destructive ? 'destructive' : 'default'}
            disabled={!matches}
            onClick={(e) => { e.preventDefault(); handleConfirm() }}
          >
            Delete
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}
