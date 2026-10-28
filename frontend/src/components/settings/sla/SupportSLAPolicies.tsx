import { useMemo, useState } from 'react'
import { QuietEmptyState } from '@/components/design-system/quiet'
import { Button } from '@/components/ui/button'
import { useConfirm } from '@/components/ui/confirm-dialog'
import { Skeleton } from '@/components/ui/skeleton'
import { Switch } from '@/components/ui/switch'
import { useChatSettings, useInboxScopes } from '@/hooks/queries/useSupport'
import {
  useCreateSLAPolicy,
  useDeleteSLAPolicy,
  useReorderSLAPolicies,
  useSLAPolicies,
  useUpdateSLAPolicy,
} from '@/hooks/queries/useSupportOperations'
import { ArrowDown01Icon, ArrowUp01Icon, PencilEdit01Icon, PlusSignIcon, Delete01Icon } from '@/lib/icons'
import { SUPPORT_SLA_NO_TIER, type SupportSLAPolicy, type SupportSLAPolicyInput } from '@/lib/pmTypes'
import { useSLATiers } from '@/hooks/queries/useSupportSLATiers'
import { useWorkspaceMembers } from '@/hooks/queries/useWorkspaces'
import { formatSLATarget, policyToDraft, draftToPolicyInput } from '@/lib/supportSLAPolicyForm'
import { SupportSLAPolicyDialog } from './SupportSLAPolicyDialog'

const CHANNEL_LABELS: Record<string, string> = { widget: 'Chat', email: 'Email', portal: 'Portal', api: 'API', internal: 'Started by team' }

function capitalize(value: string): string {
  return value.charAt(0).toUpperCase() + value.slice(1)
}

/** Who a policy is for, as shown above runs of policies with the same scope. */
function policyScope(policy: SupportSLAPolicy, tierNames: Map<string, string>): string {
  if (policy.company_tier_ids.length > 0) {
    return policy.company_tier_ids.map((id) => (id === SUPPORT_SLA_NO_TIER ? 'Companies without a tier' : tierNames.get(id) ?? 'Deleted tier')).join(', ')
  }
  if (policy.companies.length > 0) return policy.companies.map((company) => company.name).join(', ')
  return 'All companies'
}

export function SupportSLAPolicies({ workspaceId, editable }: { workspaceId: string; editable: boolean }) {
  const { data: policies, isLoading, error } = useSLAPolicies(workspaceId)
  const { data: scopes } = useInboxScopes(workspaceId)
  const { data: installation } = useChatSettings(workspaceId)
  const { data: tiers } = useSLATiers(workspaceId)
  const { data: members } = useWorkspaceMembers(workspaceId)
  // The server checks that each one can work on support conversations.
  const teammates = useMemo(() => (members ?? [])
    .filter((member) => member.user_id && member.role !== 'viewer')
    .map((member) => ({ id: member.user_id, name: member.full_name || member.email || 'Teammate' })), [members])
  const tierNames = useMemo(() => new Map((tiers ?? []).map((tier) => [tier.id, tier.name])), [tiers])
  const createPolicy = useCreateSLAPolicy(workspaceId)
  const updatePolicy = useUpdateSLAPolicy(workspaceId)
  const deletePolicy = useDeleteSLAPolicy(workspaceId)
  const reorder = useReorderSLAPolicies(workspaceId)
  const confirm = useConfirm()
  const [dialogOpen, setDialogOpen] = useState(false)
  const [editing, setEditing] = useState<SupportSLAPolicy | null>(null)

  const inboxes = useMemo(() => (scopes?.mailboxes ?? []).map((mailbox) => ({ id: mailbox.id, name: mailbox.name })), [scopes])
  const inboxNames = useMemo(() => new Map([['shared', 'Main inbox'], ...inboxes.map((inbox) => [inbox.id, inbox.name] as const)]), [inboxes])
  const businessHoursConfigured = Boolean(installation?.settings?.business_hours_enabled)

  const openEditor = (policy: SupportSLAPolicy | null) => {
    setEditing(policy)
    setDialogOpen(true)
  }

  const save = async (input: SupportSLAPolicyInput) => {
    try {
      if (editing) await updatePolicy.mutateAsync({ id: editing.id, input })
      else await createPolicy.mutateAsync(input)
      return true
    } catch {
      return false
    }
  }

  const setEnabled = (policy: SupportSLAPolicy, enabled: boolean) => {
    const { input } = draftToPolicyInput({ ...policyToDraft(policy), enabled })
    if (input) updatePolicy.mutate({ id: policy.id, input })
  }

  const move = (index: number, direction: -1 | 1) => {
    if (!policies) return
    const ids = policies.map((policy) => policy.id)
    const target = index + direction
    if (target < 0 || target >= ids.length) return
    ;[ids[index], ids[target]] = [ids[target], ids[index]]
    reorder.mutate(ids)
  }

  const remove = async (policy: SupportSLAPolicy) => {
    const confirmed = await confirm({
      title: `Delete "${policy.name}"?`,
      description: 'Open conversations it governs lose their targets; another matching policy applies if one exists.',
      confirmText: 'Delete policy',
      variant: 'destructive',
    })
    if (confirmed) deletePolicy.mutate(policy.id)
  }

  if (isLoading) return <Skeleton className="h-40 w-full" />
  if (error) {
    return <p className="text-sm text-destructive">Could not load SLA policies: {error instanceof Error ? error.message : 'unknown error'}</p>
  }

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="min-w-0 max-w-2xl">
          <h2 className="text-sm font-semibold text-quiet-text-primary">Policies</h2>
          <p className="mt-0.5 text-sm text-muted-foreground">
            Policies are checked from top to bottom; the first enabled match sets a conversation's targets, so keep company and tier policies above general ones. Missed targets are added to the conversation timeline and notify its assignee.
          </p>
        </div>
        {editable ? (
          <Button size="sm" onClick={() => openEditor(null)}>
            <PlusSignIcon className="h-4 w-4" />
            New policy
          </Button>
        ) : null}
      </div>

      {!policies || policies.length === 0 ? (
        <QuietEmptyState
          title="No SLA policies yet"
          description="Set response and resolution targets, for example a one-hour first response for urgent conversations."
          action={editable ? <Button size="sm" variant="outline" onClick={() => openEditor(null)}>Create a policy</Button> : undefined}
        />
      ) : (
        <ol className="divide-y divide-border rounded-lg border border-border" aria-label="SLA policies in evaluation order">
          {policies.map((policy, index) => {
            const scope = policyScope(policy, tierNames)
            const scopeChanged = index === 0 || scope !== policyScope(policies[index - 1], tierNames)
            const targets = [
              ['First response', formatSLATarget(policy.first_response_minutes)],
              ['Next response', formatSLATarget(policy.next_response_minutes)],
              ['Resolution', formatSLATarget(policy.resolution_minutes)],
            ].filter(([, value]) => value)
            const conditions = [
              policy.priorities.length ? policy.priorities.map(capitalize).join(', ') : null,
              policy.mailbox_ids.length ? policy.mailbox_ids.map((id) => inboxNames.get(id) ?? 'Archived inbox').join(', ') : null,
              policy.channels.length ? policy.channels.map((channel) => CHANNEL_LABELS[channel] ?? channel).join(', ') : null,
              policy.company_tier_ids.length > 0 && policy.companies.length > 0 ? policy.companies.map((company) => company.name).join(', ') : null,
            ].filter(Boolean)
            return (
              <li key={policy.id} data-testid="sla-policy">
                {scopeChanged ? (
                  <div className="px-4 pb-0 pt-3 text-[11.5px] font-semibold uppercase tracking-[0.03em] text-muted-foreground" data-testid="sla-policy-scope">
                    {scope}
                  </div>
                ) : null}
                <div className="flex flex-wrap items-center gap-3 px-4 py-3">
                  <span className="w-5 shrink-0 text-xs tabular-nums text-muted-foreground">{index + 1}</span>
                  <div className="min-w-0 flex-1">
                    <div className="flex flex-wrap items-center gap-2">
                      <span className="truncate text-sm font-medium">{policy.name}</span>
                      {!policy.enabled ? <span className="rounded-full bg-muted px-2 py-0.5 text-[11px] text-muted-foreground">Off</span> : null}
                      {policy.business_hours ? <span className="rounded-full bg-muted px-2 py-0.5 text-[11px] text-muted-foreground">Business hours</span> : null}
                      {policy.warn_before_minutes ? <span className="rounded-full bg-muted px-2 py-0.5 text-[11px] text-muted-foreground">Warns {formatSLATarget(policy.warn_before_minutes)} before</span> : null}
                    </div>
                    <div className="mt-0.5 text-xs text-muted-foreground">
                      {targets.map(([label, value]) => `${label} ${value}`).join(' · ')}
                      {' — '}
                      {conditions.length ? conditions.join(' · ') : scope === 'All companies' ? 'All conversations' : 'Any priority, inbox and channel'}
                    </div>
                  </div>
                  {editable ? (
                    <div className="flex items-center gap-1">
                      <Switch checked={policy.enabled} onCheckedChange={(enabled) => setEnabled(policy, enabled)} aria-label={`${policy.enabled ? 'Turn off' : 'Turn on'} ${policy.name}`} />
                      <Button variant="ghost" size="icon" className="h-7 w-7" aria-label={`Move ${policy.name} up`} disabled={index === 0 || reorder.isPending} onClick={() => move(index, -1)}>
                        <ArrowUp01Icon className="h-4 w-4" />
                      </Button>
                      <Button variant="ghost" size="icon" className="h-7 w-7" aria-label={`Move ${policy.name} down`} disabled={index === policies.length - 1 || reorder.isPending} onClick={() => move(index, 1)}>
                        <ArrowDown01Icon className="h-4 w-4" />
                      </Button>
                      <Button variant="ghost" size="icon" className="h-7 w-7" aria-label={`Edit ${policy.name}`} onClick={() => openEditor(policy)}>
                        <PencilEdit01Icon className="h-4 w-4" />
                      </Button>
                      <Button variant="ghost" size="icon" className="h-7 w-7 text-destructive" aria-label={`Delete ${policy.name}`} onClick={() => void remove(policy)}>
                        <Delete01Icon className="h-4 w-4" />
                      </Button>
                    </div>
                  ) : null}
                </div>
              </li>
            )
          })}
        </ol>
      )}

      <SupportSLAPolicyDialog
        open={dialogOpen}
        onOpenChange={setDialogOpen}
        policy={editing}
        workspaceId={workspaceId}
        inboxes={inboxes}
        tiers={tiers ?? []}
        teammates={teammates}
        businessHoursConfigured={businessHoursConfigured}
        saving={createPolicy.isPending || updatePolicy.isPending}
        onSave={save}
      />
    </div>
  )
}
