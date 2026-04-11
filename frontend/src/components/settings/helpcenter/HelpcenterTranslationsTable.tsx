import { useCallback, useEffect, useState } from 'react'
import { Tick01Icon, Loading01Icon, PencilEdit02Icon, PlusSignIcon, MagicWand01Icon, Cancel01Icon } from '@/lib/icons'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { docsService } from '@/lib/services/docsService'
import { getHelpcenterLocaleLabel } from '@/lib/docsTypes'
import { StoredIcon } from '@/components/ui/icon-picker'
import { buildCollectionTreeOptions } from '@/components/docs/CollectionTreePicker'
import type {
  DocsSpace,
  DocsCollection,
  DocsHelpcenterSpaceTranslation,
  DocsHelpcenterCollectionTranslation,
} from '@/lib/docsTypes'

interface TranslationsTableProps {
  workspaceId: string
  defaultLocale: string
  enabledLocales: string[]
}

interface TranslationCell {
  name: string
  exists: boolean
}

interface EditingCell {
  type: 'space' | 'collection'
  id: string
  locale: string
  name: string
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
  const [generatingAll, setGeneratingAll] = useState(false)
  const [generatingCell, setGeneratingCell] = useState<string | null>(null)

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

  const getSpaceTranslation = (spaceId: string, locale: string): TranslationCell => {
    const trans = spaceTranslations.get(spaceId)?.find((t) => t.locale === locale)
    return { name: trans?.name ?? '', exists: !!trans }
  }

  const getCollectionTranslation = (collectionId: string, locale: string): TranslationCell => {
    const trans = collectionTranslations.get(collectionId)?.find((t) => t.locale === locale)
    return { name: trans?.name ?? '', exists: !!trans }
  }

  const handleSaveEdit = async () => {
    if (!editing || !editing.name.trim()) return
    setSaving(true)
    try {
      // Don't send a slug from the client. Translation slugs are
      // frozen after first set by the backend — the server will
      // derive a slug from the name on the very first write and
      // keep that value for every subsequent edit. Sending a
      // client-derived slug here used to silently break localized
      // public URLs on every name edit; see 2026-04-11 Option C.
      if (editing.type === 'space') {
        await docsService.upsertSpaceTranslation(workspaceId, editing.id, {
          locale: editing.locale,
          name: editing.name.trim(),
        })
      } else {
        await docsService.upsertCollectionTranslation(workspaceId, editing.id, {
          locale: editing.locale,
          name: editing.name.trim(),
        })
      }
      toast.success('Translation saved')
      setEditing(null)
      void loadData()
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
        await docsService.generateSpaceTranslation(workspaceId, id, locale)
      } else {
        await docsService.generateCollectionTranslation(workspaceId, id, locale)
      }
      toast.success('Translation generated')
      void loadData()
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to generate')
    } finally {
      setGeneratingCell(null)
    }
  }

  const handleGenerateAll = async () => {
    setGeneratingAll(true)
    try {
      for (const locale of nonDefaultLocales) {
        for (const space of spaces) {
          const st = getSpaceTranslation(space.id, locale)
          if (!st.exists) {
            await docsService.generateSpaceTranslation(workspaceId, space.id, locale)
          }
          for (const coll of collectionsBySpace.get(space.id) ?? []) {
            const ct = getCollectionTranslation(coll.id, locale)
            if (!ct.exists) {
              await docsService.generateCollectionTranslation(workspaceId, coll.id, locale)
            }
          }
        }
      }
      toast.success('All missing translations generated')
      void loadData()
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to generate some translations')
    } finally {
      setGeneratingAll(false)
    }
  }

  const hasMissing = spaces.some((space) =>
    nonDefaultLocales.some((locale) => {
      if (!getSpaceTranslation(space.id, locale).exists) return true
      return (collectionsBySpace.get(space.id) ?? []).some(
        (coll) => !getCollectionTranslation(coll.id, locale).exists,
      )
    }),
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
      <div className="flex items-center justify-between">
        <div>
          <h3 className="text-sm font-medium">Translations</h3>
          <p className="text-xs text-muted-foreground mt-1">Space and collection names for each locale.</p>
        </div>
        {hasMissing && (
          <Button
            type="button"
            size="sm"
            variant="outline"
            className="h-7 gap-1.5 text-xs"
            disabled={generatingAll}
            onClick={() => void handleGenerateAll()}
          >
            <MagicWand01Icon className="h-3 w-3" />
            {generatingAll ? 'Generating...' : 'Generate all missing'}
          </Button>
        )}
      </div>

      <div className="overflow-x-auto rounded-lg border border-border/60">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b bg-muted/30">
              <th className="sticky left-0 z-10 bg-muted/30 px-3 py-2 text-left text-xs font-medium text-muted-foreground w-[200px]">Text</th>
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
              // nested collections appear under their parent and get
              // indented by depth. We look each collection up by id
              // to render the ordered tree while keeping the raw
              // DocsCollection object for translation cells.
              const treeOptions = buildCollectionTreeOptions(space.id, colls)
              const collsById = new Map(colls.map((c) => [c.id, c]))
              const orderedColls = treeOptions
                .map((opt) => {
                  const coll = collsById.get(opt.id)
                  return coll ? { coll, depth: opt.depth } : null
                })
                .filter((entry): entry is { coll: DocsCollection; depth: number } => entry !== null)
              return (
                <>
                  {/* Space row */}
                  <tr key={space.id} className="border-b border-border/40">
                    <td className="sticky left-0 z-10 bg-background px-3 py-2 font-medium">
                      <span className="inline-flex items-center gap-1.5">
                        <StoredIcon name={space.icon} className="h-4 w-4 shrink-0" textClassName="" />
                        <span>{space.name}</span>
                      </span>
                    </td>
                    {nonDefaultLocales.map((locale) => {
                      const cell = getSpaceTranslation(space.id, locale)
                      const cellKey = `space-${space.id}-${locale}`
                      const isEditing = editing?.type === 'space' && editing.id === space.id && editing.locale === locale
                      const isGenerating = generatingCell === cellKey

                      return (
                        <td key={locale} className="px-3 py-1.5">
                          {isEditing ? (
                            <div className="flex items-center gap-1">
                              <Input
                                autoFocus
                                value={editing.name}
                                onChange={(e) => setEditing({ ...editing, name: e.target.value })}
                                onKeyDown={(e) => {
                                  if (e.key === 'Enter') void handleSaveEdit()
                                  if (e.key === 'Escape') setEditing(null)
                                }}
                                className="h-7 text-xs"
                              />
                              <Button size="icon" variant="ghost" className="h-6 w-6 shrink-0" disabled={saving} onClick={() => void handleSaveEdit()}>
                                <Tick01Icon className="h-3 w-3" />
                              </Button>
                              <Button size="icon" variant="ghost" className="h-6 w-6 shrink-0" onClick={() => setEditing(null)}>
                                <Cancel01Icon className="h-3 w-3" />
                              </Button>
                            </div>
                          ) : cell.exists ? (
                            <button
                              type="button"
                              onClick={() => setEditing({ type: 'space', id: space.id, locale, name: cell.name })}
                              className="group flex items-center gap-1 text-xs text-foreground hover:text-primary transition-colors"
                            >
                              <span className="truncate max-w-[140px]">{cell.name}</span>
                              <PencilEdit02Icon className="h-3 w-3 opacity-0 group-hover:opacity-100 text-muted-foreground" />
                            </button>
                          ) : (
                            <button
                              type="button"
                              disabled={isGenerating}
                              onClick={() => void handleGenerateCell('space', space.id, locale)}
                              className="flex items-center gap-1 text-xs text-muted-foreground hover:text-primary transition-colors disabled:opacity-50"
                            >
                              {isGenerating ? <Loading01Icon className="h-3 w-3 animate-spin" /> : <PlusSignIcon className="h-3 w-3" />}
                              Generate
                            </button>
                          )}
                        </td>
                      )
                    })}
                  </tr>

                  {/* Collection rows — rendered in tree order, indented by depth. */}
                  {orderedColls.map(({ coll, depth }) => (
                    <tr key={coll.id} className="border-b border-border/20">
                      <td
                        className="sticky left-0 z-10 bg-background px-3 py-2 font-medium"
                        style={{ paddingLeft: `${12 + depth * 16}px` }}
                      >
                        {coll.name}
                      </td>
                      {nonDefaultLocales.map((locale) => {
                        const cell = getCollectionTranslation(coll.id, locale)
                        const cellKey = `collection-${coll.id}-${locale}`
                        const isEditing = editing?.type === 'collection' && editing.id === coll.id && editing.locale === locale
                        const isGenerating = generatingCell === cellKey

                        return (
                          <td key={locale} className="px-3 py-1.5">
                            {isEditing ? (
                              <div className="flex items-center gap-1">
                                <Input
                                  autoFocus
                                  value={editing.name}
                                  onChange={(e) => setEditing({ ...editing, name: e.target.value })}
                                  onKeyDown={(e) => {
                                    if (e.key === 'Enter') void handleSaveEdit()
                                    if (e.key === 'Escape') setEditing(null)
                                  }}
                                  className="h-7 text-xs"
                                />
                                <Button size="icon" variant="ghost" className="h-6 w-6 shrink-0" disabled={saving} onClick={() => void handleSaveEdit()}>
                                  <Tick01Icon className="h-3 w-3" />
                                </Button>
                                <Button size="icon" variant="ghost" className="h-6 w-6 shrink-0" onClick={() => setEditing(null)}>
                                  <Cancel01Icon className="h-3 w-3" />
                                </Button>
                              </div>
                            ) : cell.exists ? (
                              <button
                                type="button"
                                onClick={() => setEditing({ type: 'collection', id: coll.id, locale, name: cell.name })}
                                className="group flex items-center gap-1 text-xs text-foreground hover:text-primary transition-colors"
                              >
                                <span className="truncate max-w-[140px]">{cell.name}</span>
                                <PencilEdit02Icon className="h-3 w-3 opacity-0 group-hover:opacity-100 text-muted-foreground" />
                              </button>
                            ) : (
                              <button
                                type="button"
                                disabled={isGenerating}
                                onClick={() => void handleGenerateCell('collection', coll.id, locale)}
                                className="flex items-center gap-1 text-xs text-muted-foreground hover:text-primary transition-colors disabled:opacity-50"
                              >
                                {isGenerating ? <Loading01Icon className="h-3 w-3 animate-spin" /> : <PlusSignIcon className="h-3 w-3" />}
                                Generate
                              </button>
                            )}
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
    </div>
  )
}
