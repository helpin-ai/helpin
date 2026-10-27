// @vitest-environment jsdom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { DocsSpace } from '@/lib/docsTypes'

const mocks = vi.hoisted(() => ({
  create: vi.fn(),
  update: vi.fn(),
}))

vi.mock('@/hooks/queries', () => ({
  useCreateDocsSpace: () => ({ mutateAsync: mocks.create, isPending: false }),
  useUpdateDocsSpace: () => ({ mutateAsync: mocks.update, isPending: false }),
  useDocsHelpcenterLocales: () => ({ data: { default_locale: 'en', enabled_locales: ['en'] } }),
}))
vi.mock('@/hooks/useWorkspaceTeams', () => ({ useWorkspaceTeams: () => ({ teams: [] }) }))
vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }))

import { SpaceDialog } from '../SpaceDialog'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

const space: DocsSpace = {
  id: 'space-1', workspace_id: 'ws', team_ids: [], name: 'Guides', slug: 'guides', visibility: 'workspace_wide',
  type: 'external_capable', helpcenter_audience: 'public', is_system: false, position: 0, created_by: 'u',
  created_at: '2026-10-27T00:00:00Z', updated_at: '2026-10-27T00:00:00Z',
}

let root: Root
let container: HTMLDivElement
beforeEach(() => {
  container = document.createElement('div')
  document.body.append(container)
  root = createRoot(container)
  mocks.update.mockResolvedValue(space)
})
afterEach(() => {
  act(() => root.unmount())
  container.remove()
  vi.resetAllMocks()
})

function button(label: string) {
  return Array.from(document.body.querySelectorAll<HTMLButtonElement>('button')).find((candidate) => candidate.textContent?.startsWith(label))
}

describe('SpaceDialog', () => {
  it('limits an external space to signed-in customers and explains what moves', async () => {
    await act(async () => root.render(<SpaceDialog wsId="ws" open onOpenChange={() => {}} space={space} />))
    expect(document.body.textContent).toContain('Readers')
    expect(button('Everyone')?.getAttribute('aria-pressed')).toBe('true')
    await act(async () => button('Signed-in customers')?.click())
    expect(document.body.textContent).toContain('Published articles leave the public help center')
    await act(async () => button('Save Changes')?.click())
    expect(mocks.update).toHaveBeenCalledWith(expect.objectContaining({ id: 'space-1', helpcenter_audience: 'customers' }))
  })

  it('asks no audience for an internal space', async () => {
    await act(async () => root.render(<SpaceDialog wsId="ws" open onOpenChange={() => {}} space={{ ...space, type: 'internal' }} />))
    expect(document.body.textContent).not.toContain('Readers')
  })
})
