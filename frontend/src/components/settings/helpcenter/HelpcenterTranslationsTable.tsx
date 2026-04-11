import { useCallback, useEffect, useState } from 'react'
import {
  Cancel01Icon,
  Loading01Icon,
  MagicWand01Icon,
  PencilEdit02Icon,
  PlusSignIcon,
  Tick01Icon,
} from '@/lib/icons'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { docsService } from '@/lib/services/docsService'
import { getHelpcenterLocaleLabel } from '@/lib/docsTypes'
import { StoredIcon } from '@/components/ui/icon-picker'
import { buildCollectionTreeOptions } from '@/components/docs/CollectionTreePicker'
import { AutoTranslateMissingDialog } from '@/components/settings/helpcenter/AutoTranslateMissingDialog'
import type {
  DocsCollection,
  DocsHelpcenterCollectionTranslation,
  DocsHelpcenterSpaceTranslation,
  DocsSpace,
} from '@/lib/docsTypes'

type Field = 'name' | 'description'

interface TranslationsTableProps {
  workspaceId: string
  defaultLocale: string
  enabledLocales: string[]
}

interface TranslationCell {
  /** The text to display for the selected field. Empty string when the row exists but the field is empty. */
  value: string
  /** True when a translation row exists for this entity/locale — drives edit vs generate UI. */
  rowExists: boolean
}

interface EditingCell {
  type: 'space' | 'collection'
  id: string
  locale: string
  field: Field
  value: string
}

/**
 * Replace (or insert) a translation entry in a per-entity map while
 * leaving the rest of the map untouched. Used to splice local state
 * in place after a save instead of re-fetching the whole table.
 */
function upsertTranslationInMap<T extends { locale: string }>(
  map: Map<string, T[]>,
  entityId: string,
  translation: T,
): Map<string, T[]> {
  const next = new Map(map)
  const current = next.get(entityId) ?? []
  const without = current.filter((t) => t.locale !== translation.locale)
  next.set(entityId, [...without, translation])
  return next
}

export function HelpcenterTranslationsTable({
  workspaceId,
  defaultLocale,
  enabledLocales,
}: TranslationsTableProps) {
  const nonDefaultLocales = enabledLocales.filter((l) => l !== defaultLocale)
  const [spaces, setSpaces] = useState<DocsSpace[]>([])
  const [collectionsBySpace, setCollectionsBySpace] = useState<Map<string, DocsCollection[]>>(new Map())
  const [spaceTranslations, setSpaceTranslations] = useState<Map<string, DocsHelpcenterSpaceTranslation[]>>(new Map())
  const [collectionTranslations, setCollectionTranslations] = useState<Map<string, DocsHelpcenterCollectionTranslation[]>>(new Map())
  const [loading, setLoading] = useState(true)
  const [editing, setEditing] = useState<EditingCell | null>(null)
  const [saving, setSaving] = useState(false)
  const [autoTranslateOpen, setAutoTranslateOpen] = useState(false)
  const [generatingCell, setGeneratingCell] = useState<string | null>(null)
  const [field, setField] = useState<Field>('name')

  const loadData = useCallback(async () => {
    setLoading(true)
    try {
      const spacesRes = await docsService.listSpaces(workspaceId)
      const externalSpaces = (spacesRes.data ?? []).filter((s) => s.type === 'external_capable')
      setSpaces(externalSpaces)

      const collsMap = new Map<string, DocsCollection[]>()
      const spaceTransMap = new Map<string, DocsHelpcenterSpaceTranslation[]>()
      const collTransMap = new Map<string, DocsHelpcenterCollectionTranslation[]>()

      for (const space of externalSpaces) {
        const [collsRes, stRes] = await Promise.all([
          docsService.listCollections(workspaceId, space.id),
          docsService.listSpaceTranslations(workspaceId, space.id),
        ])
        collsMap.set(space.id, collsRes.data ?? [])
        spaceTransMap.set(space.id, stRes.data ?? [])

        for (const coll of collsRes.data ?? []) {
          const ctRes = await docsService.listCollectionTranslations(workspaceId, coll.id)
          collTransMap.set(coll.id, ctRes.data ?? [])
        }
      }

      setCollectionsBySpace(collsMap)
      setSpaceTranslations(spaceTransMap)
      setCollectionTranslations(collTransMap)
    } finally {
      setLoading(false)
    }
  }, [workspaceId])

  useEffect(() => { void loadData() }, [loadData])

  const getSpaceRow = (spaceId: string, locale: string) =>
    spaceTranslations.get(spaceId)?.find((t) => t.locale === locale) ?? null
  const getCollectionRow = (collectionId: string, locale: string) =>
    collectionTranslations.get(collectionId)?.find((t) => t.locale === locale) ?? null

  const getSpaceCell = (spaceId: string, locale: string): TranslationCell => {
    const row = getSpaceRow(spaceId, locale)
    if (!row) return { value: '', rowExists: false }
    const value = field === 'name' ? row.name : row.description ?? ''
    return { value, rowExists: true }
  }

  const getCollectionCell = (collectionId: string, locale: string): TranslationCell => {
    const row = getCollectionRow(collectionId, locale)
    if (!row) return { value: '', rowExists: false }
    const value = field === 'name' ? row.name : row.description ?? ''
    return { value, rowExists: true }
  }

  const handleSaveEdit = async () => {
    if (!editing) return
    // Names must be non-empty — slug derivation depends on it.
    // Descriptions may be cleared by saving an empty value.
    if (editing.field === 'name' && !editing.value.trim()) return

    setSaving(true)
    try {
      // Build a full payload for the row: always include name (either
      // the new typed value when editing name, or the stored name
      // when editing description) so the backend never silently
      // blanks out the other field. Slug is intentionally omitted —
      // the backend owns slug selection (Option C, 2026-04-11).
      const existingSpace = editing.type === 'space' ? getSpaceRow(editing.id, editing.locale) : null
      const existingCollection = editing.type === 'collection' ? getCollectionRow(editing.id, editing.locale) : null

      if (editing.type === 'space') {
        const payload = {
          locale: editing.locale,
          name: editing.field === 'name' ? editing.value.trim() : existingSpace?.name ?? '',
          description: editing.field === 'description' ? editing.value : existingSpace?.description ?? undefined,
        }
        const res = await docsService.upsertSpaceTranslation(workspaceId, editing.id, payload)
        if (res.error) throw new Error(res.error)
        if (res.data) {
          setSpaceTranslations((prev) => upsertTranslationInMap(prev, editing.id, res.data!))
        }
      } else {
        const payload = {
          locale: editing.locale,
          name: editing.field === 'name' ? editing.value.trim() : existingCollection?.name ?? '',
          description: editing.field === 'description' ? editing.value : existingCollection?.description ?? undefined,
        }
        const res = await docsService.upsertCollectionTranslation(workspaceId, editing.id, payload)
        if (res.error) throw new Error(res.error)
        if (res.data) {
          setCollectionTranslations((prev) => upsertTranslationInMap(prev, editing.id, res.data!))
        }
      }
      toast.success('Translation saved')
      setEditing(null)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to save')
    } finally {
      setSaving(false)
    }
  }

  const handleGenerateCell = async (type: 'space' | 'collection', id: string, locale: string) => {
    const key = `${type}-${id}-${locale}`
    setGeneratingCell(key)
    try {
      if (type === 'space') {
        const res = await docsService.generateSpaceTranslation(workspaceId, id, locale)
        if (res.error) throw new Error(res.error)
        if (res.data) {
          setSpaceTranslations((prev) => upsertTranslationInMap(prev, id, res.data!))
        }
      } else {
        const res = await docsService.generateCollectionTranslation(workspaceId, id, locale)
        if (res.error) throw new Error(res.error)
        if (res.data) {
          setCollectionTranslations((prev) => upsertTranslationInMap(prev, id, res.data!))
        }
      }
      toast.success('Translation generated')
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to generate')
    } finally {
      setGeneratingCell(null)
    }
  }

  // The auto-translate bulk action is owned by a dialog now (see
  // AutoTranslateMissingDialog). The dialog fires one request per
  // non-default locale in parallel and splices returned rows into
  // the local maps via callbacks.
  const handleSpliceSpaceTranslation = useCallback(
    (row: DocsHelpcenterSpaceTranslation) => {
      setSpaceTranslations((prev) => upsertTranslationInMap(prev, row.space_id, row))
    },
    [],
  )
  const handleSpliceCollectionTranslation = useCallback(
    (row: DocsHelpcenterCollectionTranslation) => {
      setCollectionTranslations((prev) => upsertTranslationInMap(prev, row.collection_id, row))
    },
    [],
  )

  if (nonDefaultLocales.length === 0) return null

  if (loading) {
    return (
      <div className="flex items-center gap-2 py-4 text-sm text-muted-foreground">
        <Loading01Icon className="h-4 w-4 animate-spin" />
        Loading translations...
      </div>
    )
  }

  if (spaces.length === 0) {
    return (
      <p className="text-xs text-muted-foreground py-2">No external spaces to translate.</p>
    )
  }

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between gap-3">
        <div className="max-w-xl">
          <h3 className="text-sm font-medium">Translations</h3>
          <p className="mt-1 text-xs text-muted-foreground">
            Translate space and collection names and descriptions for each enabled locale. Slugs are derived from the first saved name and then frozen, so public help-center URLs stay stable.
          </p>
        </div>
        <div className="flex shrink-0 items-center gap-2">
          <div
            role="tablist"
            aria-label="Translation field"
            className="inline-flex rounded-md border border-border/60 p-0.5 text-xs"
          >
            {(['name', 'description'] as const).map((option) => (
              <button
                key={option}
                type="button"
                role="tab"
                aria-selected={field === option}
                onClick={() => {
                  setField(option)
                  setEditing(null)
                }}
                className={`rounded px-2.5 py-1 font-medium transition-colors ${
                  field === option
                    ? 'bg-muted text-foreground'
                    : 'text-muted-foreground hover:text-foreground'
                }`}
              >
                {option === 'name' ? 'Names' : 'Descriptions'}
              </button>
            ))}
          </div>
          <Button
            type="button"
            size="sm"
            variant="outline"
            className="h-7 gap-1.5 text-xs"
            onClick={() => setAutoTranslateOpen(true)}
          >
            <MagicWand01Icon className="h-3 w-3" />
            Auto-translate missing
          </Button>
        </div>
      </div>

      <div className="overflow-x-auto rounded-lg border border-border/60">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b bg-muted/30">
              <th className="sticky left-0 z-10 bg-muted/30 px-3 py-2 text-left text-xs font-medium text-muted-foreground w-[200px]">
                {defaultLocale.toUpperCase()} (source)
              </th>
              {nonDefaultLocales.map((locale) => (
                <th key={locale} className="px-3 py-2 text-left text-xs font-medium text-muted-foreground min-w-[180px]">
                  {getHelpcenterLocaleLabel(locale)}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {spaces.map((space) => {
              const colls = collectionsBySpace.get(space.id) ?? []
              // Fold the flat collection list into tree order so
              // nested collections appear under their parent. Depth
              // is intentionally NOT used for indentation here —
              // the sticky Text column is narrow and depth-based
              // padding ate into the already-tight source description
              // space. Parent-then-child ordering alone is enough to
              // communicate hierarchy in this table.
              const treeOptions = buildCollectionTreeOptions(space.id, colls)
              const collsById = new Map(colls.map((c) => [c.id, c]))
              const orderedColls = treeOptions
                .map((opt) => collsById.get(opt.id))
                .filter((coll): coll is DocsCollection => coll !== undefined)
              return (
                <>
                  {/* Space row — hidden entirely in Descriptions mode
                      because DocsSpace has no description field to
                      translate and the placeholder row added more
                      noise than it removed. Names mode keeps the
                      full space row with per-locale editable name
                      cells. */}
                  {field === 'name' && (
                    <tr key={space.id} className="border-b border-border/40">
                      <td className="sticky left-0 z-10 bg-background px-3 py-2 font-medium align-top">
                        <span className="inline-flex items-center gap-1.5">
                          <StoredIcon name={space.icon} className="h-4 w-4 shrink-0" textClassName="" />
                          <span>{space.name}</span>
                        </span>
                      </td>
                      {nonDefaultLocales.map((locale) => {
                        const cell = getSpaceCell(space.id, locale)
                        const cellKey = `space-${space.id}-${locale}`
                        const isEditing =
                          editing?.type === 'space' && editing.id === space.id && editing.locale === locale
                        const isGenerating = generatingCell === cellKey

                        return (
                          <td key={locale} className="px-3 py-1.5">
                            {renderCell({
                              isEditing,
                              editing,
                              field,
                              saving,
                              cell,
                              isGenerating,
                              onStartEdit: () =>
                                setEditing({ type: 'space', id: space.id, locale, field, value: cell.value }),
                              onChangeEdit: (value) => editing && setEditing({ ...editing, value }),
                              onSave: handleSaveEdit,
                              onCancel: () => setEditing(null),
                              onGenerate: () => void handleGenerateCell('space', space.id, locale),
                            })}
                          </td>
                        )
                      })}
                    </tr>
                  )}

                  {/* Collection rows — rendered in tree order. */}
                  {orderedColls.map((coll) => (
                    <tr key={coll.id} className="border-b border-border/20">
                      <td
                        className="sticky left-0 z-10 bg-background px-3 py-2 align-top"
                      >
                        {field === 'description' ? (
                          // Descriptions mode: the translator is
                          // comparing source description → target
                          // description in the cells to the right,
                          // so the source description is primary
                          // text and the collection name becomes a
                          // small identifier underneath.
                          <>
                            <div className="line-clamp-2 text-xs font-medium">
                              {coll.description?.trim() || (
                                <span className="text-muted-foreground/60">—</span>
                              )}
                            </div>
                            <div className="mt-0.5 text-[10px] font-normal text-muted-foreground/70">
                              {coll.name}
                            </div>
                          </>
                        ) : (
                          <div className="font-medium">{coll.name}</div>
                        )}
                      </td>
                      {nonDefaultLocales.map((locale) => {
                        const cell = getCollectionCell(coll.id, locale)
                        const cellKey = `collection-${coll.id}-${locale}`
                        const isEditing = editing?.type === 'collection' && editing.id === coll.id && editing.locale === locale
                        const isGenerating = generatingCell === cellKey

                        return (
                          <td key={locale} className="px-3 py-1.5">
                            {renderCell({
                              isEditing,
                              editing,
                              field,
                              saving,
                              cell,
                              isGenerating,
                              onStartEdit: () =>
                                setEditing({ type: 'collection', id: coll.id, locale, field, value: cell.value }),
                              onChangeEdit: (value) => editing && setEditing({ ...editing, value }),
                              onSave: handleSaveEdit,
                              onCancel: () => setEditing(null),
                              onGenerate: () => void handleGenerateCell('collection', coll.id, locale),
                            })}
                          </td>
                        )
                      })}
                    </tr>
                  ))}
                </>
              )
            })}
          </tbody>
        </table>
      </div>

      <AutoTranslateMissingDialog
        wsId={workspaceId}
        locales={nonDefaultLocales}
        open={autoTranslateOpen}
        onOpenChange={setAutoTranslateOpen}
        onSpaceTranslationCreated={handleSpliceSpaceTranslation}
        onCollectionTranslationCreated={handleSpliceCollectionTranslation}
      />
    </div>
  )
}

interface RenderCellArgs {
  isEditing: boolean
  editing: EditingCell | null
  field: Field
  saving: boolean
  cell: TranslationCell
  isGenerating: boolean
  onStartEdit: () => void
  onChangeEdit: (value: string) => void
  onSave: () => void
  onCancel: () => void
  onGenerate: () => void
}

/**
 * renderCell draws one translation cell based on its current state.
 * - Editing → inline Input (name) or Textarea (description) with save/cancel
 * - Row exists → current value (or em-dash for empty descriptions) + edit pencil
 * - Row missing → Generate button that triggers AI fill for that locale
 */
function renderCell({
  isEditing,
  editing,
  field,
  saving,
  cell,
  isGenerating,
  onStartEdit,
  onChangeEdit,
  onSave,
  onCancel,
  onGenerate,
}: RenderCellArgs) {
  if (isEditing && editing) {
    if (field === 'description') {
      return (
        <div className="flex items-start gap-1">
          <Textarea
            autoFocus
            rows={3}
            value={editing.value}
            onChange={(e) => onChangeEdit(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) onSave()
              if (e.key === 'Escape') onCancel()
            }}
            placeholder="Description (optional)"
            className="min-h-[72px] text-xs"
          />
          <div className="flex flex-col gap-1">
            <Button
              size="icon"
              variant="ghost"
              className="h-6 w-6 shrink-0"
              disabled={saving}
              onClick={onSave}
            >
              <Tick01Icon className="h-3 w-3" />
            </Button>
            <Button size="icon" variant="ghost" className="h-6 w-6 shrink-0" onClick={onCancel}>
              <Cancel01Icon className="h-3 w-3" />
            </Button>
          </div>
        </div>
      )
    }
    return (
      <div className="flex items-center gap-1">
        <Input
          autoFocus
          value={editing.value}
          onChange={(e) => onChangeEdit(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter') onSave()
            if (e.key === 'Escape') onCancel()
          }}
          className="h-7 text-xs"
        />
        <Button size="icon" variant="ghost" className="h-6 w-6 shrink-0" disabled={saving} onClick={onSave}>
          <Tick01Icon className="h-3 w-3" />
        </Button>
        <Button size="icon" variant="ghost" className="h-6 w-6 shrink-0" onClick={onCancel}>
          <Cancel01Icon className="h-3 w-3" />
        </Button>
      </div>
    )
  }

  if (cell.rowExists) {
    return (
      <button
        type="button"
        onClick={onStartEdit}
        className="group flex items-center gap-1 text-left text-xs text-foreground hover:text-primary transition-colors"
      >
        {cell.value ? (
          <span className="truncate max-w-[160px]">{cell.value}</span>
        ) : (
          <span className="text-muted-foreground/60">—</span>
        )}
        <PencilEdit02Icon className="h-3 w-3 opacity-0 group-hover:opacity-100 text-muted-foreground" />
      </button>
    )
  }

  return (
    <button
      type="button"
      disabled={isGenerating}
      onClick={onGenerate}
      className="flex items-center gap-1 text-xs text-muted-foreground hover:text-primary transition-colors disabled:opacity-50"
    >
      {isGenerating ? <Loading01Icon className="h-3 w-3 animate-spin" /> : <PlusSignIcon className="h-3 w-3" />}
      Generate
    </button>
  )
}
