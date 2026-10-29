import { useState } from 'react'
import { QuietIconAction, QuietTextAction } from '@/components/design-system/quiet'
import { useConfirm } from '@/components/ui/confirm-dialog'
import { Skeleton } from '@/components/ui/skeleton'
import { Switch } from '@/components/ui/switch'
import {
  useCreateRequestType,
  useCustomFields,
  useDeleteRequestType,
  useReorderRequestTypes,
  useRequestTypes,
  useUpdateRequestType,
} from '@/hooks/queries/useCustomFields'
import { useSupportMailboxes } from '@/hooks/queries/useSupport'
import type { SupportRequestType, SupportRequestTypeInput } from '@/lib/customFields'
import { ArrowDown01Icon, ArrowUp01Icon, Delete01Icon, PencilEdit01Icon, PlusSignIcon } from '@/lib/icons'
import { RequestTypeDialog } from './RequestTypeDialog'

function plural(count: number, singular: string, pluralForm: string) {
  return `${count.toLocaleString()} ${count === 1 ? singular : pluralForm}`
}

function toInput(requestType: SupportRequestType, change: Partial<SupportRequestTypeInput> = {}): SupportRequestTypeInput {
  return {
    name: requestType.name, description: requestType.description, fields: requestType.fields,
    default_priority: requestType.default_priority ?? null, mailbox_id: requestType.mailbox_id ?? null,
    enabled: requestType.enabled, ...change,
  }
}

/** Request types customers choose on the portal, in the order they see them. */
export function RequestTypesSettings({ workspaceId, editable }: { workspaceId: string; editable: boolean }) {
  const { requestTypes, isLoading, error } = useRequestTypes(workspaceId)
  const { fields } = useCustomFields(workspaceId, 'conversation')
  const { data: mailboxes } = useSupportMailboxes(workspaceId)
  const createType = useCreateRequestType(workspaceId)
  const updateType = useUpdateRequestType(workspaceId)
  const deleteType = useDeleteRequestType(workspaceId)
  const reorder = useReorderRequestTypes(workspaceId)
  const confirm = useConfirm()
  const [dialogOpen, setDialogOpen] = useState(false)
  const [editing, setEditing] = useState<SupportRequestType | null>(null)
  const inboxes = (mailboxes ?? []).filter((mailbox) => mailbox.active !== false).map((mailbox) => ({ id: mailbox.id, name: mailbox.name }))
  const labels = new Map(fields.map((field) => [field.key, field.display_label]))

  const openEditor = (requestType: SupportRequestType | null) => {
    setEditing(requestType)
    setDialogOpen(true)
  }

  const save = async (input: SupportRequestTypeInput) => {
    try {
      if (editing) await updateType.mutateAsync({ id: editing.id, input })
      else await createType.mutateAsync(input)
      return true
    } catch {
      return false
    }
  }

  const move = (index: number, direction: -1 | 1) => {
    const ids = requestTypes.map((requestType) => requestType.id)
    ;[ids[index], ids[index + direction]] = [ids[index + direction], ids[index]]
    reorder.mutate(ids)
  }

  const remove = async (requestType: SupportRequestType) => {
    const confirmed = await confirm({
      title: `Delete ${requestType.name}?`,
      description: 'Customers can no longer choose it. SLA policies that match only this type are turned off.',
      confirmText: 'Delete request type',
      variant: 'destructive',
    })
    if (confirmed) deleteType.mutate(requestType.id)
  }

  if (isLoading) return <Skeleton className="h-24 w-full" />
  if (error) {
    return <p className="text-sm text-destructive">Could not load request types: {error instanceof Error ? error.message : 'unknown error'}</p>
  }

  return (
    <section className="space-y-3" aria-labelledby="request-types-title">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0 max-w-2xl">
          <h2 id="request-types-title" className="text-sm font-semibold text-quiet-text-primary">Request types</h2>
          <p className="mt-0.5 text-sm text-muted-foreground">
            Customers choose one on the portal and fill in its fields. Without any, the portal asks for a subject and message only.
          </p>
        </div>
        {editable ? (
          <QuietTextAction onClick={() => openEditor(null)}>
            <PlusSignIcon className="h-4 w-4" />
            New request type
          </QuietTextAction>
        ) : null}
      </div>

      {requestTypes.length === 0 ? (
        <p className="rounded-lg border border-dashed border-border px-4 py-3 text-sm text-muted-foreground">No request types yet.</p>
      ) : (
        <ol className="divide-y divide-border rounded-lg border border-border" aria-label="Request types">
          {requestTypes.map((requestType, index) => {
            const fieldNames = requestType.fields.map((field) => labels.get(field.key)).filter(Boolean)
            return (
              <li key={requestType.id} className="flex flex-wrap items-center gap-3 px-4 py-2.5" data-testid="request-type">
                <div className="min-w-0 flex-1">
                  <div className="truncate text-sm font-medium">{requestType.name}</div>
                  <div className="truncate text-xs text-muted-foreground">
                    {[fieldNames.length ? fieldNames.join(', ') : 'No fields', plural(requestType.conversation_count, 'conversation', 'conversations')].join(' · ')}
                  </div>
                </div>
                {editable ? (
                  <div className="flex items-center gap-0.5">
                    <Switch
                      className="mr-2"
                      checked={requestType.enabled}
                      aria-label={`${requestType.enabled ? 'Turn off' : 'Turn on'} ${requestType.name}`}
                      onCheckedChange={(enabled) => updateType.mutate({ id: requestType.id, input: toInput(requestType, { enabled }) })}
                    />
                    <QuietIconAction aria-label={`Move ${requestType.name} up`} disabled={index === 0 || reorder.isPending} onClick={() => move(index, -1)}>
                      <ArrowUp01Icon className="h-4 w-4" />
                    </QuietIconAction>
                    <QuietIconAction aria-label={`Move ${requestType.name} down`} disabled={index === requestTypes.length - 1 || reorder.isPending} onClick={() => move(index, 1)}>
                      <ArrowDown01Icon className="h-4 w-4" />
                    </QuietIconAction>
                    <QuietIconAction aria-label={`Edit ${requestType.name}`} onClick={() => openEditor(requestType)}>
                      <PencilEdit01Icon className="h-4 w-4" />
                    </QuietIconAction>
                    {requestType.conversation_count === 0 ? (
                      <QuietIconAction aria-label={`Delete ${requestType.name}`} className="hover:text-destructive" onClick={() => void remove(requestType)}>
                        <Delete01Icon className="h-4 w-4" />
                      </QuietIconAction>
                    ) : null}
                  </div>
                ) : (
                  <span className="text-xs text-muted-foreground">{requestType.enabled ? 'On' : 'Off'}</span>
                )}
              </li>
            )
          })}
        </ol>
      )}

      <RequestTypeDialog
        open={dialogOpen}
        requestType={editing}
        workspaceId={workspaceId}
        fields={fields}
        inboxes={inboxes}
        saving={createType.isPending || updateType.isPending}
        onOpenChange={setDialogOpen}
        onSave={save}
      />
    </section>
  )
}
