// @vitest-environment jsdom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { describe, expect, it, vi } from 'vitest'

const setTier = vi.fn()
vi.mock('@/hooks/queries/useSupportSLATiers', () => ({
  useSetCompanySLATier: () => ({ mutate: setTier, isPending: false }),
}))
vi.mock('sonner', () => ({ toast: { success: vi.fn() } }))
vi.mock('@/components/ui/select', () => ({
  // A native select keeps the test about the bar, not the popover.
  Select: ({ onValueChange, children }: { onValueChange: (value: string) => void; children: React.ReactNode }) => (
    <select aria-label="Set SLA tier" value="" onChange={(event) => onValueChange(event.target.value)}><option value="" />{children}</select>
  ),
  SelectTrigger: () => null,
  SelectValue: () => null,
  SelectContent: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  SelectItem: ({ value, children }: { value: string; children: React.ReactNode }) => <option value={value}>{children}</option>,
}))

import { CompanySLATierBulkBar } from '../CompanySLATierBulkBar'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

describe('CompanySLATierBulkBar', () => {
  it('sets or clears the tier of the selected companies, then clears the selection', () => {
    const container = document.body.appendChild(document.createElement('div'))
    const root = createRoot(container)
    const clear = vi.fn()
    const tiers = [{ id: 'tier-gold', workspace_id: 'ws', name: 'Gold', description: '', position: 0, company_count: 0, policy_count: 0, created_at: '', updated_at: '' }]
    act(() => root.render(<CompanySLATierBulkBar workspaceId="ws" companyIds={['c1', 'c2']} tiers={tiers} onClearSelection={clear} />))
    expect(container.textContent).toContain('2 selected')
    const select = container.querySelector('select') as HTMLSelectElement
    act(() => {
      select.value = 'tier-gold'
      select.dispatchEvent(new Event('change', { bubbles: true }))
    })
    expect(setTier).toHaveBeenLastCalledWith({ companyIds: ['c1', 'c2'], tierId: 'tier-gold' }, expect.anything())
    setTier.mock.lastCall![1].onSuccess({ updated: 2 })
    expect(clear).toHaveBeenCalled()
    act(() => {
      select.value = '__none__'
      select.dispatchEvent(new Event('change', { bubbles: true }))
    })
    expect(setTier).toHaveBeenLastCalledWith({ companyIds: ['c1', 'c2'], tierId: null }, expect.anything())
    act(() => root.unmount())
  })
})
