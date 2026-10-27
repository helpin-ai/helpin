// @vitest-environment jsdom
import { act, useEffect } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { usePortalDraft } from '../usePortalDraft'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

let root: Root
let container: HTMLDivElement
const probe: { current?: ReturnType<typeof usePortalDraft> } = {}
function Probe() {
  const draft = usePortalDraft('acme.new-request')
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
    window.sessionStorage.setItem('helpin.portal.acme.new-request.draft', '{not json')
    await act(async () => root.render(<Probe />))
    expect(probe.current!.draft).toEqual({ subject: '', message: '' })
  })
})
