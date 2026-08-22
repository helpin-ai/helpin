import { useCallback, useEffect, useRef, useState } from 'react'

export interface UseConfirmPressResult {
  /** Whether the first tap has armed the confirm state (show "Tap again..."). */
  armed: boolean
  /** Call on every tap. Arms on the first call, runs `action` and disarms on the second (within the window). */
  trigger: (action: () => void) => void
}

/**
 * Inline double-tap-to-confirm state for destructive actions (e.g. sign out)
 * that want a "Tap again to confirm" affordance instead of a modal. Auto-
 * disarms after `timeoutMs` if the second tap never comes.
 */
export function useConfirmPress(timeoutMs = 3000): UseConfirmPressResult {
  const [armed, setArmed] = useState(false)
  const armedRef = useRef(false)
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  useEffect(
    () => () => {
      if (timerRef.current) clearTimeout(timerRef.current)
    },
    [],
  )

  const trigger = useCallback(
    (action: () => void) => {
      if (armedRef.current) {
        if (timerRef.current) clearTimeout(timerRef.current)
        armedRef.current = false
        setArmed(false)
        action()
        return
      }
      armedRef.current = true
      setArmed(true)
      timerRef.current = setTimeout(() => {
        armedRef.current = false
        setArmed(false)
      }, timeoutMs)
    },
    [timeoutMs],
  )

  return { armed, trigger }
}
