import { useState, type ReactNode } from 'react'
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
import { Loading01Icon } from '@/lib/icons'

interface TypedConfirmDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  title: string
  description: ReactNode
  confirmText: string
  onConfirm: () => void | Promise<void>
  destructive?: boolean
  disabled?: boolean
}

export function TypedConfirmDialog({
  open,
  onOpenChange,
  title,
  description,
  confirmText,
  onConfirm,
  destructive = true,
  disabled = false,
}: TypedConfirmDialogProps) {
  const [value, setValue] = useState('')
  const [pending, setPending] = useState(false)
  const matches = value === confirmText && !disabled && !pending

  const handleConfirm = async () => {
    if (!matches) return
    setPending(true)
    try {
      await onConfirm()
      setValue('')
      onOpenChange(false)
    } finally {
      setPending(false)
    }
  }

  return (
    <AlertDialog
      open={open}
      onOpenChange={(v) => {
        if (pending) return // prevent dismiss while operation in progress
        onOpenChange(v)
        if (!v) setValue('')
      }}
    >
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
            disabled={pending}
          />
        </div>
        <AlertDialogFooter>
          <AlertDialogCancel disabled={pending}>Cancel</AlertDialogCancel>
          <AlertDialogAction
            variant={destructive ? 'destructive' : 'default'}
            disabled={!matches}
            onClick={(e) => { e.preventDefault(); handleConfirm() }}
          >
            {pending ? (
              <>
                <Loading01Icon className="mr-1.5 h-3.5 w-3.5 animate-spin" />
                Deleting...
              </>
            ) : (
              'Delete'
            )}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}
