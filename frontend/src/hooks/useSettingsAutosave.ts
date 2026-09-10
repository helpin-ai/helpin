import { useCallback, useEffect, useRef, useState } from 'react'

export type SettingsAutosaveStatus = 'idle' | 'pending' | 'saving' | 'saved' | 'error'

interface SettingsAutosaveOptions<T> {
  scopeKey: string
  enabled: boolean
  value: T
  savedValue: T | null
  save: (value: T) => Promise<unknown>
  delayMs?: number
}

interface Session {
  active: boolean
  baseline: string | null
  inFlight: boolean
  failedValue?: string
  error?: string
  hasSaved: boolean
  timer?: ReturnType<typeof setTimeout>
}

/**
 * Debounced, serialized autosave for immutable JSON-serializable settings.
 * savedValue supplies the initial baseline only. The editor must hydrate its draft
 * once per scope, rather than replace unsaved edits with background query results.
 * Changing scopeKey starts a new session; an already dispatched request cannot be
 * cancelled, but its completion cannot update or schedule writes in the new scope.
 */
export function useSettingsAutosave<T>({
  scopeKey, enabled, value, savedValue, save, delayMs = 800,
}: SettingsAutosaveOptions<T>) {
  const valueKey = JSON.stringify(value)
  const savedKey = savedValue === null ? null : JSON.stringify(savedValue)
  const sessionRef = useRef<Session | null>(null)
  const latest = useRef({ value, save })
  const [revision, setRevision] = useState(0)
  const [feedback, setFeedback] = useState<{
    status: SettingsAutosaveStatus
    error?: string
    isDirty: boolean
  }>({ status: 'idle', isDirty: false })

  useEffect(() => { latest.current = { value, save } })

  useEffect(() => {
    const session: Session = { active: true, baseline: null, inFlight: false, hasSaved: false }
    sessionRef.current = session
    return () => {
      session.active = false
      clearTimeout(session.timer)
    }
  }, [scopeKey])

  useEffect(() => {
    const session = sessionRef.current!
    clearTimeout(session.timer)
    if (session.baseline === null && savedKey !== null) session.baseline = savedKey
    const isDirty = session.baseline !== null && valueKey !== session.baseline
    if (!enabled || session.baseline === null) {
      setFeedback({ status: 'idle', isDirty })
      return
    }
    if (session.inFlight) {
      setFeedback({ status: 'saving', isDirty: true })
      return
    }
    if (!isDirty) {
      session.failedValue = undefined
      session.error = undefined
      setFeedback({ status: session.hasSaved ? 'saved' : 'idle', isDirty: false })
      return
    }
    if (session.failedValue === valueKey) {
      setFeedback({ status: 'error', error: session.error, isDirty: true })
      return
    }
    setFeedback({ status: 'pending', isDirty: true })
    session.timer = setTimeout(() => {
      if (!session.active) return
      session.inFlight = true
      const submitted = latest.current
      setFeedback({ status: 'saving', isDirty: true })
      // Capture the save function with its scope before dispatching the request.
      Promise.resolve().then(() => submitted.save(submitted.value)).then(() => {
        if (!session.active) return
        session.baseline = valueKey
        session.hasSaved = true
        session.failedValue = undefined
        session.error = undefined
      }, (cause: unknown) => {
        if (!session.active) return
        session.failedValue = valueKey
        session.error = cause instanceof Error ? cause.message : 'Unable to save changes. Please try again.'
      }).finally(() => {
        if (!session.active) return
        session.inFlight = false
        setRevision((current) => current + 1)
      })
    }, delayMs)
    return () => clearTimeout(session.timer)
  }, [scopeKey, enabled, valueKey, savedKey, delayMs, revision])

  const retry = useCallback(() => {
    const session = sessionRef.current
    if (!session?.active || session.inFlight) return
    session.failedValue = undefined
    session.error = undefined
    setRevision((current) => current + 1)
  }, [])

  return { ...feedback, retry }
}
