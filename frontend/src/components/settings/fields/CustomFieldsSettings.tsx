import { useState } from 'react'
import { QuietIconAction, QuietTextAction } from '@/components/design-system/quiet'
import { useConfirm } from '@/components/ui/confirm-dialog'
import { Skeleton } from '@/components/ui/skeleton'
import {
  useArchiveCustomField,
  useCreateCustomField,
  useCustomFields,
  useReorderCustomFields,
  useUpdateCustomField,
} from '@/hooks/queries/useCustomFields'
import { customPropertyLabel, isSystemCRMCustomProperty } from '@/lib/crmCustomProperties'
import type { CRMCustomPropertySummary } from '@/lib/crmTypes'
import {
  customFieldTypeLabel,
  suggestDefinition,
  type CustomFieldDefinition,
  type CustomFieldDefinitionInput,
  type CustomFieldObjectType,
} from '@/lib/customFields'
import { ArchiveIcon, ArrowDown01Icon, ArrowUp01Icon, PencilEdit01Icon, PlusSignIcon, ArchiveRestoreIcon } from '@/lib/icons'
import { CustomFieldDialog, type CustomFieldDraft } from './CustomFieldDialog'

function plural(count: number, singular: string, pluralForm: string) {
  return `${count.toLocaleString()} ${count === 1 ? singular : pluralForm}`
}

function usageText(field: CustomFieldDefinition, recordNoun: [string, string]) {
  if (field.usage_count === undefined) return null
  if (field.usage_count === 0) return 'Not used yet'
  const count = plural(field.usage_count, recordNoun[0], recordNoun[1])
  return `${field.usage_sampled ? 'At least ' : ''}${count}`
}

/**
 * Field definitions for one object type: label, type and options for keys
 * the workspace stores. CRM pages also list keys found in the data so they
 * can be defined.
 */
export function CustomFieldsSettings({
  workspaceId,
  objectType,
  editable,
  title,
  description,
  recordNoun,
  discovered,
}: {
  workspaceId: string
  objectType: CustomFieldObjectType
  editable: boolean
  title: string
  description: string
  recordNoun: [string, string]
  discovered?: CRMCustomPropertySummary[]
}) {
  const { fields, isLoading, error } = useCustomFields(workspaceId, objectType, { archived: true, usage: true })
  const createField = useCreateCustomField(workspaceId, objectType)
  const updateField = useUpdateCustomField(workspaceId, objectType)
  const archiveField = useArchiveCustomField(workspaceId, objectType)
  const reorder = useReorderCustomFields(workspaceId, objectType)
  const confirm = useConfirm()
  const [draft, setDraft] = useState<CustomFieldDraft | null>(null)
  const [showArchived, setShowArchived] = useState(false)

  const active = fields.filter((field) => !field.archived_at)
  const archived = fields.filter((field) => field.archived_at)
  const definedKeys = new Set(fields.map((field) => field.key))
  const undefinedKeys = (discovered ?? []).filter((summary) => !definedKeys.has(summary.key) && !isSystemCRMCustomProperty(summary.key))
  const titleId = `custom-fields-${objectType}`

  const save = async (input: CustomFieldDefinitionInput) => {
    try {
      if (draft?.mode === 'edit') await updateField.mutateAsync({ id: draft.field.id, input })
      else await createField.mutateAsync(input)
      return true
    } catch {
      return false
    }
  }

  const move = (index: number, direction: -1 | 1) => {
    const ids = active.map((field) => field.id)
    const target = index + direction
    if (target < 0 || target >= ids.length) return
    ;[ids[index], ids[target]] = [ids[target], ids[index]]
    reorder.mutate([...ids, ...archived.map((field) => field.id)])
  }

  const archive = async (field: CustomFieldDefinition) => {
    const confirmed = await confirm({
      title: `Archive ${field.display_label}?`,
      description: 'It leaves forms, panels and filter menus. Stored values are kept, and you can restore the field at any time.',
      confirmText: 'Archive field',
    })
    if (confirmed) archiveField.mutate({ id: field.id, archived: true })
  }

  if (isLoading) return <Skeleton className="h-24 w-full" />
  if (error) {
    return <p className="text-sm text-destructive">Could not load fields: {error instanceof Error ? error.message : 'unknown error'}</p>
  }

  return (
    <section className="space-y-6" aria-labelledby={titleId}>
      <div className="space-y-3">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div className="min-w-0 max-w-2xl">
            <h2 id={titleId} className="text-sm font-semibold text-quiet-text-primary">{title}</h2>
            <p className="mt-0.5 text-sm text-muted-foreground">{description}</p>
          </div>
          {editable ? (
            <QuietTextAction onClick={() => setDraft({ mode: 'create' })}>
              <PlusSignIcon className="h-4 w-4" />
              New field
            </QuietTextAction>
          ) : null}
        </div>

        {active.length === 0 ? (
          <p className="rounded-lg border border-dashed border-border px-4 py-3 text-sm text-muted-foreground">No fields yet.</p>
        ) : (
          <ol className="divide-y divide-border rounded-lg border border-border" aria-label={title}>
            {active.map((field, index) => (
              <li key={field.id} className="flex flex-wrap items-center gap-3 px-4 py-2.5" data-testid="custom-field">
                <div className="min-w-0 flex-1">
                  <div className="flex min-w-0 items-baseline gap-2">
                    <span className="truncate text-sm font-medium">{field.display_label}</span>
                    <code className="truncate text-[11px] text-muted-foreground">{field.key}</code>
                  </div>
                  <div className="truncate text-xs text-muted-foreground">
                    {[customFieldTypeLabel(field.field_type), usageText(field, recordNoun), field.description || null].filter(Boolean).join(' · ')}
                    {field.mismatch_count ? (
                      <span className="text-amber-700 dark:text-amber-300"> · {plural(field.mismatch_count, 'value doesn’t', 'values don’t')} fit</span>
                    ) : null}
                  </div>
                </div>
                {editable ? (
                  <div className="flex items-center gap-0.5">
                    <QuietIconAction aria-label={`Move ${field.display_label} up`} disabled={index === 0 || reorder.isPending} onClick={() => move(index, -1)}>
                      <ArrowUp01Icon className="h-4 w-4" />
                    </QuietIconAction>
                    <QuietIconAction aria-label={`Move ${field.display_label} down`} disabled={index === active.length - 1 || reorder.isPending} onClick={() => move(index, 1)}>
                      <ArrowDown01Icon className="h-4 w-4" />
                    </QuietIconAction>
                    <QuietIconAction aria-label={`Edit ${field.display_label}`} onClick={() => setDraft({ mode: 'edit', field })}>
                      <PencilEdit01Icon className="h-4 w-4" />
                    </QuietIconAction>
                    <QuietIconAction aria-label={`Archive ${field.display_label}`} onClick={() => void archive(field)}>
                      <ArchiveIcon className="h-4 w-4" />
                    </QuietIconAction>
                  </div>
                ) : null}
              </li>
            ))}
          </ol>
        )}

        {archived.length > 0 ? (
          <div className="space-y-2">
            <QuietTextAction aria-expanded={showArchived} onClick={() => setShowArchived(!showArchived)}>
              {showArchived ? 'Hide' : 'Show'} {plural(archived.length, 'archived field', 'archived fields')}
            </QuietTextAction>
            {showArchived ? (
              <ul className="divide-y divide-border rounded-lg border border-border" aria-label="Archived fields">
                {archived.map((field) => (
                  <li key={field.id} className="flex flex-wrap items-center gap-3 px-4 py-2.5 text-muted-foreground">
                    <div className="min-w-0 flex-1">
                      <span className="truncate text-sm">{field.display_label}</span>{' '}
                      <code className="text-[11px]">{field.key}</code>
                    </div>
                    {editable ? (
                      <QuietTextAction onClick={() => archiveField.mutate({ id: field.id, archived: false })}>
                        <ArchiveRestoreIcon className="h-4 w-4" />
                        Restore
                      </QuietTextAction>
                    ) : null}
                  </li>
                ))}
              </ul>
            ) : null}
          </div>
        ) : null}
      </div>

      {discovered ? (
        <div className="space-y-3">
          <div className="max-w-2xl">
            <h3 className="text-sm font-semibold text-quiet-text-primary">Found in your data</h3>
            <p className="mt-0.5 text-sm text-muted-foreground">
              Keys that integrations and the API store but nobody has defined. Define one to give it a label and type; its values stay as they are.
            </p>
          </div>
          {undefinedKeys.length === 0 ? (
            <p className="text-sm text-muted-foreground">Every key in use is defined.</p>
          ) : (
            <ul className="divide-y divide-border rounded-lg border border-border" aria-label="Keys found in your data">
              {undefinedKeys.map((summary) => (
                <li key={summary.key} className="flex flex-wrap items-center gap-3 px-4 py-2.5" data-testid="discovered-key">
                  <div className="min-w-0 flex-1">
                    <div className="flex min-w-0 items-baseline gap-2">
                      <span className="truncate text-sm">{customPropertyLabel(summary.key)}</span>
                      <code className="truncate text-[11px] text-muted-foreground">{summary.key}</code>
                    </div>
                    <div className="truncate text-xs text-muted-foreground">
                      {[plural(summary.count, recordNoun[0], recordNoun[1]), summary.values.slice(0, 3).map((entry) => entry.value).join(', ') || null].filter(Boolean).join(' · ')}
                    </div>
                  </div>
                  {editable ? (
                    <QuietTextAction onClick={() => setDraft({ mode: 'create', preset: suggestDefinition(summary), keyLocked: true })}>
                      Define
                    </QuietTextAction>
                  ) : null}
                </li>
              ))}
            </ul>
          )}
        </div>
      ) : null}

      <CustomFieldDialog
        draft={draft}
        saving={createField.isPending || updateField.isPending}
        onOpenChange={(open) => { if (!open) setDraft(null) }}
        onSave={save}
      />
    </section>
  )
}
