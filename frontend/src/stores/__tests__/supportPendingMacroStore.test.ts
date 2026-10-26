// @vitest-environment jsdom
import { beforeEach, describe, expect, it } from 'vitest'
import { usePendingMacroStore } from '../supportPendingMacroStore'

describe('usePendingMacroStore', () => {
  beforeEach(() => {
    localStorage.clear()
    usePendingMacroStore.setState({ pending: {} })
  })

  it('keeps a pending macro per conversation and persists it with drafts', () => {
    usePendingMacroStore.getState().setPending('c1', { macroId: 'm1', marker: 'thanks' })
    usePendingMacroStore.getState().setPending('c2', { macroId: 'm2', marker: 'refund' })
    expect(usePendingMacroStore.getState().pending.c1).toEqual({ macroId: 'm1', marker: 'thanks' })
    expect(JSON.parse(localStorage.getItem('support_pending_macros')!)).toHaveProperty('c2')
  })

  it('clears one conversation and removes empty storage', () => {
    usePendingMacroStore.getState().setPending('c1', { macroId: 'm1', marker: 'thanks' })
    usePendingMacroStore.getState().setPending('c1', null)
    expect(usePendingMacroStore.getState().pending).toEqual({})
    expect(localStorage.getItem('support_pending_macros')).toBeNull()
  })
})
