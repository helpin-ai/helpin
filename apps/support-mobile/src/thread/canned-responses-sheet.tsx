import { useEffect, useMemo, useState, type ReactNode } from 'react'
import { ArrowLeft, Pencil, Plus, Search, Trash2, Zap } from 'lucide-react'
import {
  useCreateSupportCannedResponse,
  useDeleteSupportCannedResponse,
  useUpdateSupportCannedResponse,
  type SupportCannedResponse,
} from '@helpin-ai/support-core'
import { filterShortcuts, stripShortcutContent } from '@/components/support/shortcutFiltering'
import { SHORTCUT_VARIABLES } from '@/components/support/shortcutVariables'
import {
  DEFAULT_SHORTCUT_CATEGORY,
  normalizeShortcutCategory,
  shortcutCategoryOptions,
} from '@/components/support/shortcutCategories'
import { Sheet } from '@mobile/ui/sheet'
import { Pressable } from '@mobile/ui/pressable'
import { Spinner } from '@mobile/ui/spinner'
import { toast } from 'sonner'

export interface CannedResponsesSheetProps {
  workspaceId: string
  open: boolean
  onOpenChange: (open: boolean) => void
  responses: SupportCannedResponse[]
  loading?: boolean
  canManage?: boolean
  onSelect: (response: SupportCannedResponse) => void
}

interface ShortcutForm {
  shortCode: string
  category: string
  content: string
}

type SheetMode = 'list' | 'create' | 'edit'

const EMPTY_FORM: ShortcutForm = { shortCode: '', category: DEFAULT_SHORTCUT_CATEGORY, content: '' }

export function validateShortcutCode(shortCode: string): string | null {
  const value = shortCode.trim()
  if (!value) return 'Shortcut is required'
  if (!value.startsWith('!')) return 'Shortcut must start with !'
  if (/\s/.test(value)) return 'Shortcut cannot contain spaces'
  if (value.length < 2) return 'Shortcut must have at least one character after !'
  return null
}

export function CannedResponsesSheet({
  workspaceId,
  open,
  onOpenChange,
  responses,
  loading,
  canManage = false,
  onSelect,
}: CannedResponsesSheetProps) {
  const [query, setQuery] = useState('')
  const [mode, setMode] = useState<SheetMode>('list')
  const [editingId, setEditingId] = useState<string | null>(null)
  const [form, setForm] = useState<ShortcutForm>(EMPTY_FORM)
  const [submitAttempted, setSubmitAttempted] = useState(false)
  const [deleteConfirm, setDeleteConfirm] = useState(false)
  const createShortcut = useCreateSupportCannedResponse(workspaceId)
  const updateShortcut = useUpdateSupportCannedResponse(workspaceId)
  const deleteShortcut = useDeleteSupportCannedResponse(workspaceId)
  const pending = createShortcut.isPending || updateShortcut.isPending || deleteShortcut.isPending
  const filtered = useMemo(() => filterShortcuts(responses, query, 50), [responses, query])
  const categories = useMemo(() => shortcutCategoryOptions(responses), [responses])
  const codeError = validateShortcutCode(form.shortCode)
  const duplicate = responses.some((response) => response.id !== editingId && response.short_code === form.shortCode.trim())
  const contentMissing = stripShortcutContent(form.content).length === 0
  const canSubmit = !codeError && !duplicate && !contentMissing && !pending

  useEffect(() => {
    if (!open) {
      setQuery('')
      setMode('list')
      setEditingId(null)
      setForm(EMPTY_FORM)
      setSubmitAttempted(false)
      setDeleteConfirm(false)
    }
  }, [open])

  const openCreate = () => {
    setForm(EMPTY_FORM)
    setEditingId(null)
    setSubmitAttempted(false)
    setDeleteConfirm(false)
    setMode('create')
  }

  const openEdit = (response: SupportCannedResponse) => {
    setForm({
      shortCode: response.short_code,
      category: normalizeShortcutCategory(response.tag),
      content: response.content,
    })
    setEditingId(response.id)
    setSubmitAttempted(false)
    setDeleteConfirm(false)
    setMode('edit')
  }

  const backToList = () => {
    setMode('list')
    setEditingId(null)
    setForm(EMPTY_FORM)
    setSubmitAttempted(false)
    setDeleteConfirm(false)
  }

  const submit = async () => {
    setSubmitAttempted(true)
    if (!canSubmit) return
    const shortCode = form.shortCode.trim()
    const payload = {
      short_code: shortCode,
      content: form.content.trim(),
      tag: normalizeShortcutCategory(form.category),
      title: shortCode,
    }
    try {
      if (mode === 'edit' && editingId) {
        await updateShortcut.mutateAsync({ responseId: editingId, payload })
        toast.success('Shortcut updated')
      } else {
        await createShortcut.mutateAsync(payload)
        toast.success('Shortcut created')
      }
      backToList()
    } catch {
      toast.error(mode === 'edit' ? 'Could not update shortcut' : 'Could not create shortcut')
    }
  }

  const remove = async () => {
    if (!editingId) return
    if (!deleteConfirm) {
      setDeleteConfirm(true)
      return
    }
    try {
      await deleteShortcut.mutateAsync(editingId)
      toast.success('Shortcut deleted')
      backToList()
    } catch {
      toast.error('Could not delete shortcut')
    }
  }

  return (
    <Sheet open={open} onOpenChange={onOpenChange} title="Canned responses" className="h-[78vh]">
      {mode === 'list' ? (
        <div className="flex min-h-0 flex-1 flex-col px-4">
          <div className="flex items-center justify-between gap-2 px-1 pb-2">
            <h2 className="text-headline font-semibold">Canned responses</h2>
            {canManage && (
              <Pressable
                aria-label="Create shortcut"
                onPress={openCreate}
                className="flex h-9 min-h-0 w-9 min-w-0 items-center justify-center rounded-full text-primary active:bg-primary/10"
              >
                <Plus className="h-5 w-5" />
              </Pressable>
            )}
          </div>
          <div className="flex items-center gap-2 rounded-xl border border-input bg-background px-3 py-2">
            <Search className="h-4 w-4 shrink-0 text-muted-foreground" />
            <input
              autoFocus
              aria-label="Search shortcuts"
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              placeholder="Search shortcuts…"
              className="min-w-0 flex-1 bg-transparent text-body text-foreground outline-none placeholder:text-muted-foreground"
            />
          </div>

          <div className="mt-2 min-h-0 flex-1 overflow-y-auto">
            {loading ? (
              <div className="flex justify-center py-8"><Spinner /></div>
            ) : filtered.length === 0 ? (
              <div className="space-y-3 px-1 py-8 text-center">
                <p className="text-footnote text-muted-foreground">
                  {responses.length === 0 ? 'No canned responses yet.' : 'No matches.'}
                </p>
                {canManage && responses.length === 0 && (
                  <Pressable
                    onPress={openCreate}
                    className="mx-auto h-auto min-h-9 w-auto min-w-0 rounded-full px-4 text-footnote font-medium text-primary"
                  >
                    Create your first shortcut
                  </Pressable>
                )}
              </div>
            ) : (
              filtered.map((response) => (
                <div key={response.id} className="flex items-center gap-1 rounded-xl active:bg-muted">
                  <Pressable
                    haptic="selection"
                    onPress={() => onSelect(response)}
                    aria-label={response.short_code}
                    className="flex h-auto min-h-14 min-w-0 flex-1 flex-col gap-0.5 px-2 py-2.5 text-left"
                  >
                    <span className="flex items-center gap-1.5 text-footnote font-semibold text-primary">
                      <Zap className="h-3.5 w-3.5" />
                      {response.short_code}
                      <span className="font-normal text-muted-foreground">· {normalizeShortcutCategory(response.tag)}</span>
                    </span>
                    <span className="line-clamp-2 text-footnote text-muted-foreground">{stripShortcutContent(response.content)}</span>
                  </Pressable>
                  {canManage && (
                    <Pressable
                      aria-label={`Edit ${response.short_code}`}
                      onPress={() => openEdit(response)}
                      className="flex h-10 w-10 shrink-0 items-center justify-center rounded-full text-muted-foreground"
                    >
                      <Pencil className="h-4 w-4" />
                    </Pressable>
                  )}
                </div>
              ))
            )}
          </div>
        </div>
      ) : (
        <div className="flex min-h-0 flex-1 flex-col">
          <div className="flex items-center gap-2 border-b border-border/60 px-4 pb-3">
            <Pressable
              aria-label="Back to shortcuts"
              onPress={backToList}
              className="flex h-9 w-9 items-center justify-center rounded-full text-muted-foreground"
            >
              <ArrowLeft className="h-4 w-4" />
            </Pressable>
            <h2 className="text-headline font-semibold">{mode === 'create' ? 'New shortcut' : `Edit ${form.shortCode}`}</h2>
          </div>
          <div className="min-h-0 flex-1 space-y-3 overflow-y-auto px-4 py-4">
            <ShortcutField label="Shortcut" required>
              <input
                aria-label="Shortcut"
                autoFocus
                value={form.shortCode}
                onChange={(event) => setForm((current) => ({ ...current, shortCode: event.target.value }))}
                placeholder="!hello"
                className="h-11 w-full rounded-xl border border-input bg-background px-3 font-mono text-body outline-none focus:border-ring"
              />
              {submitAttempted && codeError && <p role="alert" className="text-caption text-destructive">{codeError}</p>}
              {submitAttempted && !codeError && duplicate && <p role="alert" className="text-caption text-destructive">That shortcut already exists.</p>}
            </ShortcutField>
            <ShortcutField label="Category">
              <input
                aria-label="Category"
                list="mobile-shortcut-categories"
                value={form.category}
                onChange={(event) => setForm((current) => ({ ...current, category: event.target.value }))}
                className="h-11 w-full rounded-xl border border-input bg-background px-3 text-body outline-none focus:border-ring"
              />
              <datalist id="mobile-shortcut-categories">
                {categories.map((category) => <option key={category} value={category} />)}
              </datalist>
            </ShortcutField>
            <ShortcutField label="Message" required>
              <textarea
                aria-label="Shortcut message"
                value={form.content}
                onChange={(event) => setForm((current) => ({ ...current, content: event.target.value }))}
                placeholder="Write the saved reply…"
                className="min-h-36 w-full resize-none rounded-xl border border-input bg-background p-3 text-body outline-none focus:border-ring"
              />
              {submitAttempted && contentMissing && <p role="alert" className="text-caption text-destructive">Message is required.</p>}
            </ShortcutField>
            <div>
              <p className="mb-1.5 text-caption text-muted-foreground">Insert variable</p>
              <div className="flex flex-wrap gap-1.5">
                {SHORTCUT_VARIABLES.map((variable) => (
                  <Pressable
                    key={variable.label}
                    onPress={() => setForm((current) => ({ ...current, content: `${current.content}${variable.token}` }))}
                    className="h-auto min-h-8 w-auto min-w-0 rounded-full bg-muted px-2.5 font-mono text-caption text-foreground"
                  >
                    {variable.label}
                  </Pressable>
                ))}
              </div>
            </div>
          </div>
          <div className="flex items-center gap-2 border-t border-border/60 px-4 pt-3">
            {mode === 'edit' && (
              <Pressable
                disabled={pending}
                onPress={() => void remove()}
                className="flex h-11 w-auto min-w-0 items-center gap-1.5 rounded-xl px-3 text-footnote font-medium text-destructive"
              >
                <Trash2 className="h-4 w-4" />
                {deleteConfirm ? 'Confirm delete' : 'Delete'}
              </Pressable>
            )}
            <Pressable
              disabled={pending}
              onPress={backToList}
              className="ml-auto flex h-11 w-auto min-w-0 items-center rounded-xl px-3 text-footnote font-medium text-muted-foreground"
            >
              Cancel
            </Pressable>
            <Pressable
              disabled={pending}
              onPress={() => void submit()}
              className="flex h-11 w-auto min-w-20 items-center justify-center rounded-xl bg-primary px-4 text-footnote font-medium text-primary-foreground"
            >
              {pending ? <Spinner size={16} /> : mode === 'create' ? 'Create' : 'Save'}
            </Pressable>
          </div>
        </div>
      )}
    </Sheet>
  )
}

function ShortcutField({ label, required, children }: { label: string; required?: boolean; children: ReactNode }) {
  return (
    <label className="block space-y-1.5 text-footnote font-medium">
      <span>{label}{required ? ' *' : ''}</span>
      {children}
    </label>
  )
}
