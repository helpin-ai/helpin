// @vitest-environment jsdom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'

import type { TeamEstimateSettings } from '@/lib/types'

let currentConfig: TeamEstimateSettings | null = null

vi.mock('@/stores/workspaceStore', () => ({
  useWorkspaceStore: (selector: (state: { currentWorkspace: { id: string } | null }) => unknown) =>
    selector({ currentWorkspace: { id: 'ws-1' } }),
}))

vi.mock('@/hooks/queries', () => ({
  useTeamEstimateSettingsForTeam: vi.fn(() => currentConfig),
}))

import { EstimatePicker } from '../EstimatePicker'

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true
vi.stubGlobal(
  'ResizeObserver',
  class ResizeObserver {
    observe() {}
    unobserve() {}
    disconnect() {}
  },
)

const enabledConfig: TeamEstimateSettings = {
  id: 'cfg-1',
  team_id: 'team-1',
  enabled: true,
  scale: 'fibonacci',
  extended: false,
  allow_zero: false,
  count_unestimated_as_one: false,
  created_at: '2026-03-24T00:00:00Z',
  updated_at: '2026-03-24T00:00:00Z',
}

describe('EstimatePicker', () => {
  afterEach(() => {
    currentConfig = null
  })

  it('keeps a stable hook order when team estimate settings resolve after the first render', () => {
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)
    const consoleErrorSpy = vi.spyOn(console, 'error').mockImplementation(() => {})

    expect(() => {
      act(() => {
        root.render(
          <EstimatePicker
            value=""
            teamId="team-1"
            onChange={() => {}}
          />,
        )
      })

      currentConfig = enabledConfig

      act(() => {
        root.render(
          <EstimatePicker
            value=""
            teamId="team-1"
            onChange={() => {}}
          />,
        )
      })
    }).not.toThrow()

    const messages = consoleErrorSpy.mock.calls.flat().join(' ')
    expect(messages).not.toContain('controlled to uncontrolled')
    expect(messages).not.toContain('Rendered more hooks')
    expect(messages).not.toContain('Rendered fewer hooks')

    act(() => {
      root.unmount()
    })
    container.remove()
    consoleErrorSpy.mockRestore()
  })
})
