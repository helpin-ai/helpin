// @vitest-environment jsdom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { TooltipProvider } from '@/components/ui/tooltip'

const { saved, update } = vi.hoisted(() => ({
  saved: { config: {} as Record<string, unknown> },
  update: vi.fn(async (_workspace: string, patch: Record<string, unknown>) => {
    saved.config = { ...saved.config, ...patch }
    return { data: saved.config }
  }),
}))
vi.mock('@/lib/services/docsService', () => ({ docsService: {
  getHelpcenterConfig: async () => ({ data: saved.config }),
  listSpaces: async () => ({ data: [] }),
  updateHelpcenterConfig: update,
}}))
vi.mock('@/hooks/queries', () => ({
  useDocsHelpcenterLocales: () => ({ data: undefined }),
  useUpdateDocsHelpcenterLocales: () => ({ mutateAsync: vi.fn() }),
}))
vi.mock('@tanstack/react-router', () => ({ Link: () => null }))
vi.mock('../HelpAssistantSetting', () => ({ HelpAssistantSetting: () => null }))
vi.mock('../helpcenter/HelpcenterLocalesCard', () => ({ HelpcenterLocalesCard: () => null }))
vi.mock('../helpcenter/HelpcenterTranslationsTable', () => ({ HelpcenterTranslationsTable: () => null }))
vi.mock('../helpcenter/HelpcenterCustomDomainStatus', () => ({ HelpcenterCustomDomainStatus: () => null }))

import { HelpcenterTab } from '../HelpcenterTab'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
let root: Root
let container: HTMLDivElement
beforeEach(() => vi.stubGlobal('ResizeObserver', class { observe() {} unobserve() {} disconnect() {} }))
afterEach(() => { act(() => root?.unmount()); container?.remove(); vi.clearAllMocks(); vi.unstubAllGlobals() })

describe('Help Center demo mode', () => {
  it.each([true, false])('preserves indexing preference %s through save, reload, and leaving demo mode', async (indexing) => {
    saved.config = { brand_name: 'Example', subdomain: 'example', demo_mode: false, search_indexing_enabled: indexing }
    container = document.createElement('div')
    document.body.appendChild(container)
    const mount = async () => {
      root = createRoot(container)
      await act(async () => root.render(<TooltipProvider><HelpcenterTab workspaceId="workspace" workspaceName="Example" workspaceSlug="example" /></TooltipProvider>))
    }
    const submit = async () => act(async () => { container.querySelector('form')!.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true })) })
    await mount()
    await act(async () => container.querySelector<HTMLButtonElement>('#hc-demo-mode')!.click())
    expect(container.querySelector('#hc-indexing')!.getAttribute('aria-checked')).toBe('false')
    expect(container.querySelector<HTMLButtonElement>('#hc-indexing')!.disabled).toBe(true)
    await submit()
    expect(update.mock.lastCall?.[1]).toMatchObject({ demo_mode: true, search_indexing_enabled: indexing })
    act(() => root.unmount())
    await mount()
    await act(async () => container.querySelector<HTMLButtonElement>('#hc-demo-mode')!.click())
    expect(container.querySelector('#hc-indexing')!.getAttribute('aria-checked')).toBe(String(indexing))
    await submit()
    expect(update.mock.lastCall?.[1]).toMatchObject({ demo_mode: false, search_indexing_enabled: indexing })
  })
})
