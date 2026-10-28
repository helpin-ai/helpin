// @vitest-environment jsdom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { describe, expect, it, vi } from 'vitest'

const setTier = vi.fn()
vi.mock('@/hooks/queries/useSupportSLATiers', () => ({
  useSetCompanySLATier: () => ({ mutateAsync: setTier, isPending: false }),
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
  it('sets or clears the tier of the selected companies, then clears the selection', async () => {
    setTier.mockImplementation(async ({ companyIds }: { companyIds: string[] }) => ({ updated: companyIds.length }))
    const container = document.body.appendChild(document.createElement('div'))
    const root = createRoot(container)
    const clear = vi.fn()
    const tiers = [{ id: 'tier-gold', workspace_id: 'ws', name: 'Gold', description: '', position: 0, company_count: 0, policy_count: 0, created_at: '', updated_at: '' }]
    const ids = Array.from({ length: 450 }, (_, index) => `c${index}`)
    act(() => root.render(<CompanySLATierBulkBar workspaceId="ws" companyIds={ids} tiers={tiers} onClearSelection={clear} />))
    expect(container.textContent).toContain('450 selected')
    const select = container.querySelector('select') as HTMLSelectElement
    await act(async () => {
      select.value = 'tier-gold'
      select.dispatchEvent(new Event('change', { bubbles: true }))
    })
    // Large selections go in batches the server accepts.
    expect(setTier.mock.calls.map(([args]) => args.companyIds.length)).toEqual([200, 200, 50])
    expect(setTier).toHaveBeenLastCalledWith({ companyIds: ids.slice(400), tierId: 'tier-gold' })
    expect(clear).toHaveBeenCalledTimes(1)

    setTier.mockRejectedValueOnce(new Error('denied'))
    await act(async () => {
      select.value = '__none__'
      select.dispatchEvent(new Event('change', { bubbles: true }))
    })
    expect(setTier).toHaveBeenLastCalledWith({ companyIds: ids.slice(0, 200), tierId: null })
    expect(clear).toHaveBeenCalledTimes(1)
    act(() => root.unmount())
  })
})
