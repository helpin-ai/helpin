// @vitest-environment jsdom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { DocsSharePopover } from '../DocsSharePopover'
Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
let root: Root, container: HTMLDivElement
const toggle = vi.fn()
const copy = vi.fn().mockResolvedValue(undefined)
beforeEach(() => {
 toggle.mockReset();copy.mockClear()
 Object.defineProperty(navigator, 'clipboard', { configurable: true, value: { writeText: copy } })
 container = document.createElement('div');document.body.append(container);root=createRoot(container)
})
afterEach(async () => { await act(async () => root.unmount());container.remove() })
async function render(shared=false,disabled=false) {
 await act(async () => root.render(<DocsSharePopover shared={shared} shareToken="token-1" disabled={disabled} onToggle={toggle} />))
 await act(async () => container.querySelector('button')!.click())
}
it('shows an off toggle without a public URL until sharing is enabled', async () => {
 await render()
 const control=document.querySelector<HTMLButtonElement>('[role="switch"]')!
 expect(control.getAttribute('aria-checked')).toBe('false')
 expect(document.querySelector('input')).toBeNull()
 await act(async () => control.click())
 expect(toggle).toHaveBeenCalledWith(true)
})
it('copies the public URL when sharing is enabled', async () => {
 await render(true)
 expect(document.querySelector('input')?.value).toBe(`${window.location.origin}/share/token-1`)
 const button=Array.from(document.querySelectorAll('button')).find(el=>el.textContent==='Copy link')!
 await act(async () => button.click())
 expect(copy).toHaveBeenCalledWith(`${window.location.origin}/share/token-1`)
 expect(document.body.textContent).toContain('Copied')
})
it('keeps the toggle disabled for locked documents or pending changes', async () => {
 await render(true,true)
 const control=document.querySelector<HTMLButtonElement>('[role="switch"]')!
 expect(control.disabled).toBe(true)
 await act(async () => control.click())
 expect(toggle).not.toHaveBeenCalled()
})
