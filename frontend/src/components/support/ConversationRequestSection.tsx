import { QuietDropdown } from '@/components/design-system/quiet-dropdown'
import { CustomFieldEditor } from '@/components/custom-fields/CustomFieldEditor'
import { CollapsibleSection } from '@/components/ui/collapsible-section'
import { useCustomFields, useRequestTypes, useUpdateConversationFields } from '@/hooks/queries/useCustomFields'
import { usePermissions, useWorkspaceAccess } from '@/hooks/queries/useSession'
import type { CustomFieldDefinition } from '@/lib/customFields'
import { ArrowDown01Icon, LeftToRightListBulletIcon } from '@/lib/icons'
import type { SupportConversation } from '@/lib/pmTypes'
import { cn } from '@/lib/utils'

const NONE = '__none__'

function FieldRow({
  field,
  value,
  editable,
  note,
  onSave,
}: {
  field: CustomFieldDefinition
  value: unknown
  editable: boolean
  note?: string
  onSave: (value: unknown) => void
}) {
  return (
    <div className="contents" data-conversation-field={field.key}>
      <span className="mt-0.5 min-w-0 self-start truncate text-xs text-muted-foreground" title={[field.description, note].filter(Boolean).join(' · ') || field.display_label}>
        {field.display_label}
        {note ? <span className="ml-1 text-[10px] uppercase tracking-wide text-muted-foreground/70">{note}</span> : null}
      </span>
      <div className="min-w-0">
        <CustomFieldEditor definition={field} value={value} editable={editable} onSave={onSave} />
      </div>
    </div>
  )
}

/**
 * The conversation's request type and field values. The type's fields come
 * first, in its order; other fields follow when they have a value, or all
 * fields when the conversation has no type.
 */
export function ConversationRequestSection({ workspaceId, conversation }: { workspaceId: string; conversation: SupportConversation }) {
  const { requestTypes } = useRequestTypes(workspaceId)
  const { fields } = useCustomFields(workspaceId, 'conversation')
  const { setType, setFields } = useUpdateConversationFields(workspaceId, conversation.id)
  const { data: access } = useWorkspaceAccess(workspaceId)
  const canEdit = usePermissions(access).has('support.edit')
  if (requestTypes.length === 0 && fields.length === 0) return null

  const editable = canEdit && !conversation.merged_into_id
  const values = conversation.custom_fields ?? {}
  const current = requestTypes.find((requestType) => requestType.id === conversation.request_type_id)
  const byKey = new Map(fields.map((field) => [field.key, field]))
  const typeFields = (current?.fields ?? []).flatMap((entry) => {
    const field = byKey.get(entry.key)
    return field ? [{ field, entry }] : []
  })
  const onType = new Set(typeFields.map(({ field }) => field.key))
  const otherFields = fields.filter((field) => !onType.has(field.key) && (!current || values[field.key] !== undefined))
  const choices = requestTypes.filter((requestType) => requestType.enabled || requestType.id === current?.id)
  const save = (key: string) => (value: unknown) => setFields.mutate({ [key]: value })

  return (
    <CollapsibleSection title="Request" icon={LeftToRightListBulletIcon} defaultOpen persistKey="support-details.request">
      <div className="grid grid-cols-[minmax(0,6.5rem)_minmax(0,1fr)] items-center gap-x-2 gap-y-1">
        {requestTypes.length > 0 ? (
          <>
            <span className="text-xs text-muted-foreground">Type</span>
            <QuietDropdown
              label="Request type"
              selected={[current?.id ?? NONE]}
              options={[{ value: NONE, label: 'None' }, ...choices.map((requestType) => ({ value: requestType.id, label: requestType.name }))]}
              disabled={!editable}
              onSelect={(value) => {
                const next = value === NONE ? null : value
                if (next !== (conversation.request_type_id ?? null)) setType.mutate(next)
              }}
              trigger={
                <button
                  type="button"
                  aria-label="Request type"
                  className="flex w-full min-w-0 items-center justify-between gap-1 rounded px-1.5 py-0.5 text-left text-xs transition-colors hover:bg-accent focus-visible:bg-accent focus-visible:outline-none disabled:cursor-default disabled:hover:bg-transparent"
                >
                  <span className={cn('truncate', current ? 'font-medium text-foreground' : 'text-muted-foreground')}>
                    {current?.name ?? (conversation.request_type_id ? 'Deleted type' : 'None')}
                  </span>
                  {editable ? <ArrowDown01Icon className="h-3 w-3 shrink-0 text-muted-foreground" /> : null}
                </button>
              }
            />
          </>
        ) : null}
        {typeFields.map(({ field, entry }) => (
          <FieldRow key={field.id} field={field} value={values[field.key]} editable={editable} note={entry.customer_visible ? undefined : 'Internal'} onSave={save(field.key)} />
        ))}
        {current && otherFields.length > 0 ? (
          <div className="col-span-2 pt-1.5 text-[11px] font-medium text-muted-foreground">Other fields</div>
        ) : null}
        {otherFields.map((field) => (
          <FieldRow key={field.id} field={field} value={values[field.key]} editable={editable} onSave={save(field.key)} />
        ))}
      </div>
    </CollapsibleSection>
  )
}
