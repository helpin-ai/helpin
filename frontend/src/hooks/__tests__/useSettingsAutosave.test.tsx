// @vitest-environment jsdom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useSettingsAutosave } from '../useSettingsAutosave'

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

type Options = Parameters<typeof useSettingsAutosave<string>>[0]
let state: ReturnType<typeof useSettingsAutosave<string>>
let root: Root
let container: HTMLDivElement
let options: Options
function Harness() { state = useSettingsAutosave(options); return null }
function render(patch: Partial<Options> = {}) {
  options = { ...options, ...patch }
  act(() => root.render(<Harness />))
}
async function tick(ms = 800) { await act(async () => { await vi.advanceTimersByTimeAsync(ms) }) }
function deferred() {
  let resolve!: () => void
  let reject!: (error: Error) => void
  const promise = new Promise<void>((yes, no) => { resolve = yes; reject = no })
  return { promise, resolve, reject }
}

beforeEach(() => {
  vi.useFakeTimers()
  container = document.createElement('div')
  root = createRoot(container)
  options = { scopeKey: 'one', enabled: true, value: 'initial', savedValue: 'initial', save: vi.fn().mockResolvedValue(undefined) }
})
afterEach(() => { act(() => root.unmount()); vi.useRealTimers() })

describe('useSettingsAutosave', () => {
  it('does not save hydration and debounces edits into the latest value', async () => {
    render({ savedValue: null, enabled: false })
    await tick()
    render({ savedValue: 'initial', enabled: true })
    await tick()
    expect(options.save).not.toHaveBeenCalled()
    render({ value: 'first' })
    await tick(400)
    render({ value: 'latest' })
    await tick(799)
    expect(options.save).not.toHaveBeenCalled()
    await tick(1)
    expect(options.save).toHaveBeenCalledTimes(1)
    expect(options.save).toHaveBeenCalledWith('latest')
    expect(state.status).toBe('saved')
  })

  it('serializes writes and does not mark newer edits saved on old completion', async () => {
    const first = deferred()
    const save = vi.fn().mockReturnValueOnce(first.promise).mockResolvedValue(undefined)
    render({ save })
    render({ value: 'first' })
    await tick()
    render({ value: 'second' })
    await tick()
    expect(save).toHaveBeenCalledTimes(1)
    await act(async () => first.resolve())
    expect(state.isDirty).toBe(true)
    expect(state.status).not.toBe('saved')
    await tick()
    expect(save.mock.calls.map(([v]) => v)).toEqual(['first', 'second'])
    expect(state.status).toBe('saved')
  })

  it('persists a revert made while the earlier edit was saving', async () => {
    const first = deferred()
    const save = vi.fn().mockReturnValueOnce(first.promise).mockResolvedValue(undefined)
    render({ save })
    render({ value: 'edit' })
    await tick()
    render({ value: 'initial' })
    await act(async () => first.resolve())
    await tick()
    expect(save.mock.calls.map(([v]) => v)).toEqual(['edit', 'initial'])
  })

  it('keeps failures visible without retry loops and supports explicit retry', async () => {
    const save = vi.fn().mockRejectedValueOnce(new Error('Network unavailable')).mockResolvedValue(undefined)
    render({ save })
    render({ value: 'edit' })
    await tick()
    expect(state.status).toBe('error')
    expect(state.error).toBe('Network unavailable')
    await tick(10_000)
    expect(save).toHaveBeenCalledTimes(1)
    act(() => state.retry())
    await tick()
    expect(save).toHaveBeenCalledTimes(2)
    expect(state.status).toBe('saved')
  })

  it('retries with a newly edited value after failure', async () => {
    const save = vi.fn().mockRejectedValueOnce(new Error('Offline')).mockResolvedValue(undefined)
    render({ save }); render({ value: 'first' }); await tick()
    render({ value: 'second' }); await tick()
    expect(save.mock.calls.map(([v]) => v)).toEqual(['first', 'second'])
  })

  it('ignores stale saved values from query refreshes', async () => {
    render(); render({ value: 'edit' }); await tick()
    render({ savedValue: 'stale' }); await tick()
    expect(state.isDirty).toBe(false)
    expect(options.save).toHaveBeenCalledTimes(1)
  })

  it('cancels pending writes on scope change and ignores previous scope completion', async () => {
    const first = deferred()
    const oldSave = vi.fn().mockReturnValue(first.promise)
    const newSave = vi.fn().mockResolvedValue(undefined)
    render({ save: oldSave }); render({ value: 'old edit' }); await tick()
    render({ scopeKey: 'two', value: 'other', savedValue: 'other', save: newSave })
    await act(async () => first.resolve())
    expect(state.status).toBe('idle')
    render({ value: 'new edit' })
    render({ scopeKey: 'three', value: 'third', savedValue: 'third' })
    await tick()
    expect(newSave).not.toHaveBeenCalled()
  })

  it('cancels pending writes when disabled or unmounted', async () => {
    render(); render({ value: 'edit' }); render({ enabled: false }); await tick()
    expect(options.save).not.toHaveBeenCalled()
    render({ enabled: true }); act(() => root.unmount()); root = createRoot(container)
    await tick()
    expect(options.save).not.toHaveBeenCalled()
  })
})
