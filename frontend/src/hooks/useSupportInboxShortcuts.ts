import { useEffect, useRef } from 'react'
import { dispatchSupportShortcut, supportShortcutForEvent } from '@/lib/supportShortcuts'
import { useSupportInboxStore } from '@/stores/supportInboxStore'

/**
 * Listens for inbox keyboard shortcuts and hands each one to the component
 * that owns the action: the list moves, the thread resolves or assigns, the
 * composer opens a reply or note. Shortcuts never fire while typing.
 */
export function useSupportInboxShortcuts({ enabled, onCheatsheet }: { enabled: boolean; onCheatsheet: () => void }) {
  const onCheatsheetRef = useRef(onCheatsheet)
  useEffect(() => {
    onCheatsheetRef.current = onCheatsheet
  }, [onCheatsheet])

  useEffect(() => {
    if (!enabled) return
    const handleKeyDown = (event: KeyboardEvent) => {
      const action = supportShortcutForEvent(event)
      if (!action) return
      if (action === 'cheatsheet') {
        event.preventDefault()
        onCheatsheetRef.current()
        return
      }
      const conversationId = useSupportInboxStore.getState().selectedConversationId
      if (dispatchSupportShortcut(action, conversationId)) event.preventDefault()
    }
    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [enabled])
}
