import { useEffect, useRef, useState } from 'react'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { onSupportShortcut } from '@/lib/supportShortcuts'
import { PriorityIcon } from '@/lib/pmConstants'
import type { ConversationPriority } from '@/lib/pmTypes'

const CONVERSATION_PRIORITY_OPTIONS: { value: ConversationPriority; label: string }[] = [
  { value: 'urgent', label: 'Urgent' },
  { value: 'high', label: 'High' },
  { value: 'medium', label: 'Medium' },
  { value: 'low', label: 'Low' },
]

/** Compact priority select used in the conversation details. */
export function ConversationPriorityPicker({
  value,
  onChange,
  disabled,
  shortcutConversationId,
}: {
  value: ConversationPriority
  onChange: (priority: ConversationPriority) => void
  disabled?: boolean
  /** Opens on the P shortcut for this conversation while visible. */
  shortcutConversationId?: string
}) {
  const [open, setOpen] = useState(false)
  const triggerRef = useRef<HTMLButtonElement>(null)
  useEffect(() => {
    if (!shortcutConversationId) return
    return onSupportShortcut((detail) => {
      if (detail.action !== 'priority' || detail.conversationId !== shortcutConversationId) return
      // A hidden sidebar leaves the shortcut to nothing rather than a misplaced menu.
      if (!triggerRef.current?.offsetParent) return
      detail.handled = true
      if (!disabled) setOpen(true)
    })
  }, [disabled, shortcutConversationId])

  return (
    <Select open={open} onOpenChange={setOpen} value={value} onValueChange={(next) => onChange(next as ConversationPriority)} disabled={disabled}>
      <SelectTrigger ref={triggerRef} aria-label="Priority" className="h-7 w-auto min-w-[110px] gap-1.5 border-none px-1.5 text-xs font-medium shadow-none hover:bg-quiet-hover">
        <SelectValue />
      </SelectTrigger>
      <SelectContent align="end">
        {CONVERSATION_PRIORITY_OPTIONS.map((option) => (
          <SelectItem key={option.value} value={option.value}>
            <span className="flex items-center gap-2">
              <PriorityIcon priority={option.value} className="h-3.5 w-3.5" />
              {option.label}
            </span>
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  )
}
