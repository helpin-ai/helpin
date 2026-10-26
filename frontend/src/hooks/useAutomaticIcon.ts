import { useCallback, useLayoutEffect, useRef } from 'react'
import { iconSuggestionsService, type IconSuggestionKind } from '@/lib/services/iconSuggestions'

interface AutomaticIconOptions {
  workspaceId: string
  kind: IconSuggestionKind
  name: string
  open: boolean
  enabled: boolean
  defaultIcon: string
  onChange: (icon: string) => void
}

/** Suggestions belong to a new, untouched form. Saved entities never opt in. */
export function useAutomaticIcon({ workspaceId, kind, name, open, enabled, defaultIcon, onChange }: AutomaticIconOptions) {
  const stopped = useRef(false)
  const change = useRef(onChange)
  const cancel = useRef<(() => void) | null>(null)
  const normalizedName = name.trim().replace(/\s+/g, ' ')

  useLayoutEffect(() => { change.current = onChange })
  useLayoutEffect(() => { stopped.current = false }, [open, workspaceId, kind])

  useLayoutEffect(() => {
    if (!open || !enabled || stopped.current || !workspaceId) return
    change.current(defaultIcon)
    if (!normalizedName || [...normalizedName].length > 160) return

    let cancelled = false
    const timer = window.setTimeout(() => {
      void iconSuggestionsService.suggest(workspaceId, kind, normalizedName)
        .then((icon) => {
          if (!cancelled && !stopped.current && icon) change.current(icon)
        })
        .catch(() => { /* Optional suggestion: retain the default on network failure. */ })
    }, 600)
    const cleanup = () => { cancelled = true; window.clearTimeout(timer) }
    cancel.current = cleanup
    return cleanup
  }, [workspaceId, kind, normalizedName, open, enabled, defaultIcon])

  const stop = useCallback(() => {
    stopped.current = true
    cancel.current?.()
  }, [])
  const chooseIcon = useCallback((icon: string) => {
    stop()
    change.current(icon)
  }, [stop])

  return { chooseIcon, stop }
}
