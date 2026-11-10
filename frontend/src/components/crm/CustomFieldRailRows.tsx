import { useState } from 'react'
import { CustomFieldEditor } from '@/components/custom-fields/CustomFieldEditor'
import { useCustomFields, useSetCRMFieldValues } from '@/hooks/queries/useCustomFields'
import { customPropertyRows } from '@/lib/crmCustomProperties'
import { CustomPropertyMetadataRows } from './CustomPropertyMetadataRows'
import { Tag01Icon } from '@/lib/icons'

const labelClassName = 'mt-0.5 self-start truncate text-[12px] text-muted-foreground'

/**
 * Custom properties in a detail rail's icon / label / value grid. Defined
 * fields come first, in their order, and are editable with crm.edit; keys
 * nobody defined stay read-only under "Other data".
 */
export function CustomFieldRailRows({
  workspaceId,
  objectType,
  recordId,
  properties,
  editable,
}: {
  workspaceId: string
  objectType: 'crm_contact' | 'crm_company'
  recordId: string
  properties: Record<string, unknown> | null | undefined
  editable: boolean
}) {
  const { fields } = useCustomFields(workspaceId, objectType)
  const setValues = useSetCRMFieldValues(workspaceId, objectType, recordId)
  const [showOther, setShowOther] = useState(false)
  const definedKeys = new Set(fields.map((field) => field.key))
  const otherProperties = Object.fromEntries(Object.entries(properties ?? {}).filter(([key]) => !definedKeys.has(key)))
  const otherCount = customPropertyRows(otherProperties).length
  // Without definitions every key is "other", so show them as before.
  const otherOpen = showOther || fields.length === 0

  return (
    <>
      {fields.map((field) => (
        <div key={field.id} className="contents" data-custom-field={field.key}>
          <Tag01Icon className="mt-0.5 h-3.5 w-3.5 shrink-0 self-start text-muted-foreground" />
          <span className={labelClassName} title={field.description || field.display_label}>{field.display_label}</span>
          <div className="min-w-0">
            <CustomFieldEditor
              definition={field}
              value={properties?.[field.key]}
              editable={editable}
              onSave={(value) => setValues.mutate({ [field.key]: value })}
            />
          </div>
        </div>
      ))}
      {otherCount > 0 && fields.length > 0 ? (
        <div className="col-span-3 pt-1">
          <button
            type="button"
            aria-expanded={showOther}
            className="text-[11px] font-medium text-muted-foreground transition-colors hover:text-foreground"
            onClick={() => setShowOther(!showOther)}
          >
            {showOther ? 'Hide other data' : `Other data (${otherCount})`}
          </button>
        </div>
      ) : null}
      {otherOpen ? <CustomPropertyMetadataRows properties={otherProperties} /> : null}
    </>
  )
}
