import { useEffect, useRef, useState, type RefObject } from 'react'
import type { Editor } from '@tiptap/core'
import type { Transaction } from '@tiptap/pm/state'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { Button } from '@/components/ui/button'
import { MagicWand01Icon, Loading01Icon } from '@/lib/icons'
import { UpgradeRequiredDialog } from '@edition'
import { getUpgradeRequiredReason, type UpgradeRequiredReason } from '@edition/errors'
import { rewriteDocsSelection, type DocsAIAction } from '@/lib/services/docsAIService'
import { captureAISelection, mapAISelection, replaceAISelection, sanitizeAIRewrite, type AISelection } from './docsAISelection'

const actions: { operation: DocsAIAction; label: string }[] = [
  { operation: 'improve', label: 'Improve writing' },
  { operation: 'fix_grammar', label: 'Fix spelling & grammar' },
  { operation: 'shorten', label: 'Make shorter' },
  { operation: 'expand', label: 'Expand' },
  { operation: 'simplify', label: 'Simplify language' },
]
const tones: { operation: DocsAIAction; label: string }[] = [
  { operation: 'professional', label: 'Professional' },
  { operation: 'friendly', label: 'Friendly' },
  { operation: 'casual', label: 'Casual' },
]
const rowClass = 'w-full rounded-md px-3 py-2 text-left text-sm hover:bg-accent focus-visible:bg-accent outline-none'

export function DocsAISelectionMenu({ editor, workspaceId, open, onOpenChange, contentRef }: {
  editor: Editor
  workspaceId: string
  open: boolean
  onOpenChange: (open: boolean) => void
  contentRef: RefObject<HTMLDivElement | null>
}) {
  const selection = useRef<AISelection | null>(null)
  const request = useRef<AbortController | null>(null)
  const [instructions, setInstructions] = useState('')
  const [toneMenu, setToneMenu] = useState(false)
  const [busy, setBusy] = useState(false)
  const [candidate, setCandidate] = useState('')
  const [error, setError] = useState('')
  const [chosen, setChosen] = useState<{ operation: DocsAIAction; label: string } | null>(null)
  const [upgradeReason, setUpgradeReason] = useState<UpgradeRequiredReason | null>(null)

  useEffect(() => {
    const track = ({ transaction }: { transaction: Transaction }) => {
      if (selection.current) selection.current = mapAISelection(selection.current, transaction)
    }
    editor.on('transaction', track)
    return () => { editor.off('transaction', track); request.current?.abort() }
  }, [editor, workspaceId])

  useEffect(() => {
    if (!open) { request.current?.abort(); request.current = null; selection.current = null }
  }, [open])

  const changeOpen = (next: boolean) => {
    request.current?.abort()
    request.current = null
    setBusy(false)
    setCandidate('')
    setError('')
    setChosen(null)
    setToneMenu(false)
    if (next) {
      setInstructions('')
      try { selection.current = captureAISelection(editor) }
      catch (cause) { selection.current = null; setError(cause instanceof Error ? cause.message : 'Select text to edit with AI.') }
    } else selection.current = null
    onOpenChange(next)
  }

  const run = async (action: { operation: DocsAIAction; label: string }) => {
    if (busy || !selection.current) return
    if (!selection.current.valid) { setError('The selected text changed. Select it again to start a new rewrite.'); return }
    request.current?.abort()
    const controller = new AbortController()
    request.current = controller
    setBusy(true); setError(''); setCandidate(''); setChosen(action)
    try {
      const html = await rewriteDocsSelection(workspaceId, {
        content: selection.current.html, operation: action.operation,
        ...(action.operation === 'custom' ? { instructions: instructions.trim() } : {}),
      }, controller.signal)
      if (controller.signal.aborted) return
      const clean = sanitizeAIRewrite(html)
      const element = document.createElement('div'); element.innerHTML = clean
      if (!element.textContent?.trim()) throw new Error('AI returned no text. Try again.')
      setCandidate(clean)
    } catch (cause) {
      if (controller.signal.aborted) return
      const reason = getUpgradeRequiredReason(cause)
      if (reason) {
        // Hide the popover but keep its toolbar mounted while the modal owns focus.
        setUpgradeReason(reason)
      } else setError(cause instanceof Error ? cause.message : 'Could not rewrite the selection. Try again.')
    } finally {
      if (!controller.signal.aborted) setBusy(false)
    }
  }

  const replace = () => {
    if (!selection.current) return
    try { replaceAISelection(editor, selection.current, candidate); changeOpen(false) }
    catch (cause) { setError(cause instanceof Error ? cause.message : 'Could not replace the selection.') }
  }

  return <>
    <Popover open={open && upgradeReason === null} onOpenChange={changeOpen}>
      <PopoverTrigger asChild>
        <button type="button" onMouseDown={event => event.preventDefault()} className={`flex items-center gap-1.5 whitespace-nowrap rounded px-2 py-2 text-sm font-medium transition-colors ${open ? 'bg-accent text-foreground' : 'text-muted-foreground hover:bg-accent hover:text-foreground'}`}>
          <MagicWand01Icon className="h-4 w-4" aria-hidden="true" />
          Edit with AI
        </button>
      </PopoverTrigger>
      <PopoverContent
        ref={contentRef} align="end" side="bottom" sideOffset={6} collisionPadding={8}
        className={`z-[60] max-w-[min(calc(100vw-1rem),var(--radix-popover-content-available-width))] max-h-[var(--radix-popover-content-available-height)] gap-0 overflow-y-auto overscroll-contain rounded-lg p-1 shadow-lg ${chosen ? 'w-[30rem]' : 'w-64'}`}
        aria-label="Edit selected text with AI"
        onCloseAutoFocus={event => event.preventDefault()}
      >
        {chosen ? <div className="flex min-h-0 flex-col gap-3 p-2">
          <div className="shrink-0 text-sm font-medium">{chosen.label}</div>
          {busy && <div role="status" className="flex items-center gap-2 py-3 text-sm text-muted-foreground"><Loading01Icon className="h-4 w-4 animate-spin" />Rewriting…</div>}
          {candidate && <div className="min-h-0 overflow-y-auto overscroll-contain break-words whitespace-pre-wrap rounded-md border p-3 text-sm leading-relaxed [&_p]:mb-2 [&_p:last-child]:mb-0 [&_ul]:list-disc [&_ul]:pl-5 [&_ol]:list-decimal [&_ol]:pl-5 [&_pre]:whitespace-pre-wrap [&_pre]:break-all" onClick={event => { if ((event.target as HTMLElement).closest('a')) event.preventDefault() }} dangerouslySetInnerHTML={{ __html: candidate }} />}
          {error && <p role="alert" className="text-sm text-destructive">{error}</p>}
          <div className="flex shrink-0 flex-wrap gap-2">
            {candidate && <Button size="sm" onClick={replace}>Replace selection</Button>}
            {!busy && <Button size="sm" variant="outline" onClick={() => void run(chosen)}>Try again</Button>}
            <Button size="sm" variant="ghost" onClick={() => changeOpen(false)}>{busy ? 'Cancel' : 'Discard'}</Button>
          </div>
        </div> : <>
          <form className="m-1 mb-2 flex min-w-0 gap-1 rounded-md border p-1" onSubmit={event => { event.preventDefault(); if (instructions.trim()) void run({ operation: 'custom', label: 'Custom edit' }) }}>
            <input aria-label="AI editing instruction" placeholder="Ask AI to edit…" value={instructions} onChange={event => setInstructions(event.target.value)} maxLength={2000} className="min-w-0 flex-1 bg-transparent px-2 py-1.5 text-sm outline-none" />
            <button type="submit" aria-label="Submit AI instruction" disabled={!instructions.trim() || !!error} className="rounded px-2 hover:bg-accent disabled:opacity-40">↑</button>
          </form>
          {error ? <p role="alert" className="px-3 py-2 text-sm text-destructive">{error}</p> : toneMenu ? <>
            <button className={rowClass + ' text-muted-foreground'} onClick={() => setToneMenu(false)}>Back</button>
            {tones.map(action => <button key={action.operation} className={rowClass} onClick={() => void run(action)}>{action.label}</button>)}
          </> : <>
            {actions.map(action => <button key={action.operation} className={rowClass} onClick={() => void run(action)}>{action.label}</button>)}
            <div className="my-1 border-t" />
            <button className={rowClass + ' flex items-center justify-between'} onClick={() => setToneMenu(true)}>Change tone<span aria-hidden="true">›</span></button>
          </>}
        </>}
      </PopoverContent>
    </Popover>
    <UpgradeRequiredDialog open={upgradeReason !== null} onOpenChange={next => { if (!next) { setUpgradeReason(null); changeOpen(false) } }} reason={upgradeReason} />
  </>
}
