// @vitest-environment jsdom
import { act, type ReactNode } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { DocsSharePopover } from '../DocsSharePopover'
vi.mock('@/components/ui/quick-tooltip', () => ({
 QuickTooltip: ({ label, children }: { label: string; children: ReactNode }) => <span data-tooltip={label}>{children}</span>,
}))
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
it('keeps the toggle disabled while a sharing change is pending', async () => {
 await render(true,true)
 const control=document.querySelector<HTMLButtonElement>('[role="switch"]')!
 expect(control.disabled).toBe(true)
 await act(async () => control.click())
 expect(toggle).not.toHaveBeenCalled()
})

it('shares the live Help Center URL without offering a standalone link toggle', async () => {
 const liveUrl = 'https://docs.example.com/fr/articles/live-abcd1234'
 await act(async () => root.render(<DocsSharePopover shared={false} disabled={false} onToggle={toggle} helpCenter={{ published: true, liveUrl, canPreview: false, getPreviewUrl: vi.fn() }} />))
 await act(async () => container.querySelector('button')!.click())
 expect(document.querySelector('[role="switch"]')).toBeNull()
 expect(document.querySelector('input')?.value).toBe(liveUrl)
 expect(document.querySelector('a')?.href).toBe(liveUrl)
 const button=document.querySelector<HTMLButtonElement>('[aria-label="Copy live link"]')!
 expect(document.querySelector<HTMLButtonElement>('[aria-label="Copy preview link"]')?.disabled).toBe(true)
 expect(document.querySelector('[data-tooltip="No unpublished changes"]')).toBeTruthy()
 await act(async () => button.click())
 expect(copy).toHaveBeenCalledWith(liveUrl)
})
it('asks users to publish drafts before sharing', async () => {
 await act(async () => root.render(<DocsSharePopover shared={false} disabled={false} onToggle={toggle} helpCenter={{ published: false, liveUrl: null, canPreview: true, getPreviewUrl: vi.fn() }} />))
 await act(async () => container.querySelector('button')!.click())
 expect(document.querySelector<HTMLButtonElement>('[aria-label="Copy live link"]')?.disabled).toBe(true)
 expect(document.querySelector('[data-tooltip="Publish first"]')).toBeTruthy()
 expect(document.querySelector<HTMLButtonElement>('[aria-label="Copy preview link"]')?.disabled).toBe(false)
 expect(document.querySelector('input')).toBeNull()
 expect(document.querySelector('[role="switch"]')).toBeNull()
})
it('allows an existing standalone Help Center link to be turned off', async () => {
 await act(async () => root.render(<DocsSharePopover shared shareToken="old-token" disabled={false} onToggle={toggle} helpCenter={{ published: true, liveUrl: 'https://docs.example.com/articles/live-abcd1234', canPreview: false, getPreviewUrl: vi.fn() }} />))
 await act(async () => container.querySelector('button')!.click())
 await act(async () => document.querySelector<HTMLButtonElement>('[role="switch"]')!.click())
 expect(toggle).toHaveBeenCalledWith(false)
 expect(document.body.textContent).toContain('Existing standalone link')
})

it('copies a fresh preview link while retaining the live link for unpublished changes', async () => {
 const getPreviewUrl = vi.fn().mockResolvedValue('https://docs.example.com/preview/doc?token=fresh')
 await act(async () => root.render(<DocsSharePopover shared={false} disabled={false} onToggle={toggle} helpCenter={{ published: true, liveUrl: 'https://docs.example.com/articles/live-abcd1234', canPreview: true, getPreviewUrl }} />))
 await act(async () => container.querySelector('button')!.click())
 expect(document.querySelector<HTMLButtonElement>('[aria-label="Copy live link"]')!.disabled).toBe(false)
 await act(async () => document.querySelector<HTMLButtonElement>('[aria-label="Copy preview link"]')!.click())
 expect(getPreviewUrl).toHaveBeenCalledTimes(1)
 expect(copy).toHaveBeenCalledWith('https://docs.example.com/preview/doc?token=fresh')
 expect(document.body.textContent).toContain('15 minutes')
})
it('reports preview generation failures without copying a broken URL', async () => {
 await act(async () => root.render(<DocsSharePopover shared={false} disabled={false} onToggle={toggle} helpCenter={{ published: false, liveUrl: null, canPreview: true, getPreviewUrl: vi.fn().mockRejectedValue(new Error('Unavailable')) }} />))
 await act(async () => container.querySelector('button')!.click())
 await act(async () => document.querySelector<HTMLButtonElement>('[aria-label="Copy preview link"]')!.click())
 expect(copy).not.toHaveBeenCalled()
 expect(document.querySelector('[role="alert"]')?.textContent).toContain('Could not create preview link')
})
