// @vitest-environment jsdom
import { act, useState } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { useAutomaticIcon } from '../useAutomaticIcon'
const suggest = vi.hoisted(() => vi.fn())
vi.mock('@/lib/services/iconSuggestions', () => ({ iconSuggestionsService: { suggest } }))
;(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true
let root: Root
let container: HTMLDivElement
function Form({ name = 'Billing', open = true, enabled = true }: {name?: string; open?: boolean; enabled?: boolean}) {
 const [icon, setIcon] = useState('folder')
 const { chooseIcon, stop } = useAutomaticIcon({ workspaceId: 'ws', kind: 'collection', name, open, enabled, defaultIcon: 'folder', onChange: setIcon })
 return <><output>{icon}</output><button onClick={() => chooseIcon('shield')}>Manual</button><button onClick={() => chooseIcon('')}>Remove</button><button onClick={stop}>Save</button></>
}
const render = (props: Parameters<typeof Form>[0] = {}) => act(() => root.render(<Form {...props} />))
const tick = () => act(async () => { await vi.advanceTimersByTimeAsync(650) })
beforeEach(() => { vi.useFakeTimers(); suggest.mockReset(); suggest.mockResolvedValue('rocket'); container = document.createElement('div'); root = createRoot(container) })
afterEach(() => { act(() => root.unmount()); vi.useRealTimers() })
it('debounces naming and applies the suggestion', async () => {
 render({name:'Bil'}); render({name:'Billing'}); expect(suggest).not.toHaveBeenCalled()
 await tick(); expect(suggest).toHaveBeenCalledTimes(1); expect(container.querySelector('output')?.textContent).toBe('rocket')
})
it.each(['Manual', 'Remove', 'Save'])('ignores pending answers after %s and stops further requests', async (action) => {
 let resolve!: (value: string) => void
 suggest.mockReturnValue(new Promise<string>(r => { resolve = r }))
 render(); await tick()
 act(() => [...container.querySelectorAll('button')].find(b => b.textContent === action)!.click())
 await act(async () => resolve('rocket'))
 expect(container.querySelector('output')?.textContent).toBe(action === 'Manual' ? 'shield' : action === 'Remove' ? '' : 'folder')
 render({name:'Security'}); await tick(); expect(suggest).toHaveBeenCalledTimes(1)
})
it('ignores stale name responses and restores default for uncertain names', async () => {
 let resolve!: (value: string) => void
 suggest.mockReturnValueOnce(new Promise<string>(r => { resolve = r })).mockResolvedValueOnce('')
 render(); await tick(); render({name:'Something vague'})
 await act(async () => resolve('rocket')); await tick()
 expect(container.querySelector('output')?.textContent).toBe('folder')
})
it('does not request suggestions for existing items or closed dialogs', async () => {
 render({enabled:false}); await tick(); render({open:false}); await tick(); expect(suggest).not.toHaveBeenCalled()
})
it('resets manual override when reopened and ignores answers after closing', async () => {
 render(); act(() => container.querySelector('button')!.click()); render({open:false}); render(); await tick()
 expect(container.querySelector('output')?.textContent).toBe('rocket')
 let resolve!: (value: string) => void
 suggest.mockReturnValueOnce(new Promise<string>(r => { resolve = r }))
 render({name:'Security'}); await tick(); render({open:false}); await act(async () => resolve('shield'))
 expect(container.querySelector('output')?.textContent).toBe('folder')
})
it('keeps default when the request fails', async () => {
 suggest.mockRejectedValue(new Error('unavailable')); render(); await tick(); expect(container.querySelector('output')?.textContent).toBe('folder')
})
