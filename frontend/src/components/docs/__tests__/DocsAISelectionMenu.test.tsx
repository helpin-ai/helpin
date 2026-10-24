// @vitest-environment jsdom
import { act, useRef, useState } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { Editor } from '@tiptap/core'
import StarterKit from '@tiptap/starter-kit'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { DocsAISelectionMenu } from '../DocsAISelectionMenu'
const mocks = vi.hoisted(() => ({ rewrite: vi.fn(), classify: vi.fn() }))
vi.mock('@/lib/services/docsAIService', () => ({ rewriteDocsSelection: mocks.rewrite }))
vi.mock('@edition/errors', () => ({ getUpgradeRequiredReason: mocks.classify }))
vi.mock('@edition', () => ({ UpgradeRequiredDialog: ({ open }: { open: boolean }) => open ? <div>Upgrade dialog</div> : null }))
Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
let editor: Editor, container: HTMLDivElement, root: Root
function Harness() {
 const [open, setOpen] = useState(false)
 const ref = useRef<HTMLDivElement>(null)
 return <DocsAISelectionMenu editor={editor} workspaceId="workspace" open={open} onOpenChange={setOpen} contentRef={ref} />
}
async function click(text: string) {
 const button = Array.from(document.querySelectorAll('button')).find(el => el.textContent?.trim() === text)
 expect(button).toBeTruthy()
 await act(async () => button!.click())
}
beforeEach(async () => {
 mocks.rewrite.mockReset().mockResolvedValue('<p><strong>team</strong></p>')
 mocks.classify.mockReset().mockReturnValue(null)
 editor = new Editor({ extensions: [StarterKit], content: '<p>Hello world today.</p>' })
 editor.commands.setTextSelection({ from: 7, to: 12 })
 container = document.createElement('div'); document.body.append(container); root = createRoot(container)
 await act(async () => root.render(<Harness />))
})
afterEach(async () => { await act(async () => root.unmount()); editor.destroy(); container.remove() })
describe('Docs AI selection menu', () => {
 it('previews without saving and replaces only after confirmation', async () => {
  await click('Edit with AI'); await click('Make shorter')
  expect(editor.getText()).toBe('Hello world today.')
  expect(mocks.rewrite).toHaveBeenCalledWith('workspace', expect.objectContaining({ content: 'world', operation: 'shorten' }), expect.any(AbortSignal))
  await click('Replace selection')
  expect(editor.getText().trim()).toBe('Hello team today.')
 })
 it('discards a preview without changing the original', async () => {
  await click('Edit with AI'); await click('Expand'); await click('Discard')
  expect(editor.getText()).toBe('Hello world today.')
 })
 it('omits Translate and keeps the panel out of the toolbar with viewport bounds', async () => {
  await click('Edit with AI')
  expect(document.body.textContent).not.toContain('Translate')
  const panel = document.querySelector('[data-slot="popover-content"]')!
  expect(container.contains(panel)).toBe(false)
  expect(panel.getAttribute('data-align')).toBe('end')
  expect(panel.className).toContain('max-w-')
  expect(panel.className).toContain('z-[60]')
 })
 it('aborts a dismissed request and ignores its late result', async () => {
  let finish!: (html: string) => void
  mocks.rewrite.mockImplementation(() => new Promise(resolve => { finish = resolve }))
  await click('Edit with AI'); await click('Expand'); await click('Cancel')
  expect(mocks.rewrite.mock.calls[0][2].aborted).toBe(true)
  await act(async () => finish('<p>late</p>'))
  expect(editor.getText()).toBe('Hello world today.')
  expect(document.body.textContent).not.toContain('Replace selection')
 })
 it('retries using the original selection and refuses to apply over intervening edits', async () => {
  await click('Edit with AI'); await click('Make shorter'); await click('Try again')
  expect(mocks.rewrite.mock.calls[1][1].content).toBe('world')
  await act(async () => editor.commands.insertContentAt({ from: 7, to: 12 }, 'changed'))
  await click('Replace selection')
  expect(editor.getText()).toContain('changed')
  expect(document.body.textContent).toContain('selected text changed')
 })
 it('offers tone actions without item icons', async () => {
  await click('Edit with AI')
  const tone = Array.from(document.querySelectorAll('button')).find(button => button.textContent?.startsWith('Change tone'))!
  await act(async () => tone.click())
  await click('Professional')
  expect(mocks.rewrite.mock.calls[0][1].operation).toBe('professional')
 })
 it('shows the upgrade dialog for usage failures', async () => {
  mocks.rewrite.mockRejectedValue(new Error('AI usage exhausted'))
  mocks.classify.mockReturnValue({ kind: 'ai_usage' })
  await click('Edit with AI'); await click('Expand')
  expect(document.body.textContent).toContain('Upgrade dialog')
  expect(document.body.textContent).not.toContain('AI usage exhausted')
 })
})
