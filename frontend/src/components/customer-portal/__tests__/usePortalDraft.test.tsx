// @vitest-environment jsdom
import { act, useEffect } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { clearPortalDrafts, usePortalDraft } from '../usePortalDraft'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

let root: Root
let container: HTMLDivElement
const probe: { current?: ReturnType<typeof usePortalDraft> } = {}
function Probe({ customerID = 'alice' }: { customerID?: string }) {
  const draft = usePortalDraft('acme', customerID)
  useEffect(() => { probe.current = draft })
  return null
}
beforeEach(() => {
  window.sessionStorage.clear()
  container = document.createElement('div')
  document.body.append(container)
  root = createRoot(container)
})
afterEach(() => {
  act(() => root.unmount())
  container.remove()
})

describe('usePortalDraft', () => {
  it('restores the draft for the browser tab until it is cleared', async () => {
    await act(async () => root.render(<Probe />))
    await act(async () => probe.current!.setDraft({ subject: 'Export fails', message: 'On Safari' }))
    act(() => root.unmount())
    root = createRoot(container)
    await act(async () => root.render(<Probe />))
    expect(probe.current!.draft).toEqual({ subject: 'Export fails', message: 'On Safari' })
    await act(async () => probe.current!.clearDraft())
    act(() => root.unmount())
    root = createRoot(container)
    await act(async () => root.render(<Probe />))
    expect(probe.current!.draft).toEqual({ subject: '', message: '' })
  })

  it('ignores a malformed stored draft', async () => {
    window.sessionStorage.setItem('helpin.portal.acme.alice.new-request.draft', '{not json')
    await act(async () => root.render(<Probe />))
    expect(probe.current!.draft).toEqual({ subject: '', message: '' })
  })

  it('isolates drafts when the signed-in customer changes', async () => {
    window.sessionStorage.setItem('helpin.portal.acme.new-request.draft', JSON.stringify({ subject: 'Legacy secret', message: 'Private' }))
    await act(async () => root.render(<Probe />))
    expect(probe.current!.draft).toEqual({ subject: '', message: '' })
    await act(async () => probe.current!.setDraft({ subject: 'Alice secret', message: 'Private' }))
    await act(async () => root.render(<Probe customerID="bob" />))
    expect(probe.current!.draft).toEqual({ subject: '', message: '' })
    await act(async () => root.render(<Probe customerID="alice" />))
    expect(probe.current!.draft.subject).toBe('Alice secret')
  })

  it('clears scoped and legacy drafts for the portal on sign-out', async () => {
    await act(async () => root.render(<Probe />))
    await act(async () => probe.current!.setDraft({ subject: 'Private', message: 'Details' }))
    window.sessionStorage.setItem('helpin.portal.acme.new-request.draft', '{}')
    window.sessionStorage.setItem('helpin.portal.other.alice.new-request.draft', '{}')
    clearPortalDrafts('acme')
    act(() => root.unmount())
    root = createRoot(container)
    await act(async () => root.render(<Probe />))
    expect(probe.current!.draft).toEqual({ subject: '', message: '' })
    expect(window.sessionStorage.getItem('helpin.portal.acme.new-request.draft')).toBeNull()
    expect(window.sessionStorage.getItem('helpin.portal.other.alice.new-request.draft')).not.toBeNull()
  })
})
