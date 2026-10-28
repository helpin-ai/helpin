import { toast } from 'sonner'
import { QuietTextAction } from '@/components/design-system/quiet'
import { SidebarPopoverSelect } from '@/components/pm/SidebarPopoverSelect'
import { useConfirm } from '@/components/ui/confirm-dialog'
import {
  useCompanySLA,
  useCustomizeCompanySLA,
  useRemoveCompanySLA,
  useSetCompanySLATier,
  useSLATiers,
} from '@/hooks/queries/useSupportSLATiers'
import type { SupportCompanySLA } from '@/lib/pmTypes'
import { formatSLATarget } from '@/lib/supportSLAPolicyForm'

const NO_TIER = '__none__'

function policyTargets(policy: SupportCompanySLA['policies'][number]): string {
  const targets = [
    ['First response', formatSLATarget(policy.first_response_minutes)],
    ['Next response', formatSLATarget(policy.next_response_minutes)],
    ['Resolution', formatSLATarget(policy.resolution_minutes)],
  ].filter(([, value]) => value).map(([label, value]) => `${label} ${value}`)
  const priorities = policy.priorities.length
    ? policy.priorities.map((priority) => priority.charAt(0).toUpperCase() + priority.slice(1)).join(', ')
    : 'Any priority'
  return `${priorities}: ${targets.join(' · ')}${policy.business_hours ? ' (business hours)' : ''}`
}

/**
 * A company's SLA in its detail rail: the tier it carries and the policies
 * that can apply to its conversations. Support admins change the tier and
 * create or remove a custom SLA for the company.
 */
export function CompanySLASection({
  workspaceId,
  workspaceSlug,
  companyId,
  companyName,
  canManage,
}: {
  workspaceId: string
  workspaceSlug: string
  companyId: string
  companyName: string
  canManage: boolean
}) {
  const { data: sla, isLoading, error } = useCompanySLA(workspaceId, companyId)
  const { data: tiers } = useSLATiers(workspaceId)
  const setTier = useSetCompanySLATier(workspaceId)
  const customize = useCustomizeCompanySLA(workspaceId)
  const removeCustom = useRemoveCompanySLA(workspaceId)
  const confirm = useConfirm()

  // Nothing to show until the workspace uses SLA policies or tiers.
  if (isLoading || error || !sla || (sla.policies.length === 0 && (tiers ?? []).length === 0 && !sla.tier_id)) return null

  const tierOptions = [{ value: NO_TIER, label: 'No tier' }, ...(tiers ?? []).map((tier) => ({ value: tier.id, label: tier.name }))]
  const settingsHref = `/w/${workspaceSlug}/settings/support-sla`

  const remove = async () => {
    const confirmed = await confirm({
      title: `Remove the custom SLA of ${companyName}?`,
      description: 'Its company policies are deleted. Open conversations move to the next matching policy, such as the tier’s.',
      confirmText: 'Remove custom SLA',
      variant: 'destructive',
    })
    if (confirmed) removeCustom.mutate(companyId)
  }

  return (
    <section aria-labelledby="company-sla-title" className="-mx-4 mt-5 border-t border-border/60 px-4 pt-4 sm:-mx-6 sm:px-6 lg:-mx-5 lg:px-5">
      <h3 id="company-sla-title" className="text-xs font-semibold uppercase tracking-wide text-foreground/70">Support SLA</h3>
      <div className="mt-2 grid grid-cols-[72px_1fr] items-center gap-x-2 gap-y-1.5 text-[12px]">
        <span className="text-muted-foreground">Tier</span>
        {canManage && (tiers ?? []).length > 0 ? (
          <SidebarPopoverSelect
            value={sla.tier_id ?? NO_TIER}
            options={tierOptions}
            searchPlaceholder="Search tiers"
            disabled={setTier.isPending}
            triggerLabel="SLA tier"
            onChange={(value) => {
              const tierId = value === NO_TIER ? null : value
              if (tierId !== sla.tier_id) setTier.mutate({ companyIds: [companyId], tierId })
            }}
            renderTrigger={() => <span className={sla.tier_name ? 'truncate' : 'truncate text-muted-foreground'}>{sla.tier_name ?? 'No tier'}</span>}
          />
        ) : (
          <span className={sla.tier_name ? 'truncate px-1.5' : 'truncate px-1.5 text-muted-foreground'}>{sla.tier_name ?? 'No tier'}</span>
        )}
      </div>

      {sla.policies.length === 0 ? (
        <p className="mt-2 text-xs text-muted-foreground">No SLA policy applies to this company’s conversations.</p>
      ) : (
        <ul className="mt-2 space-y-1.5" aria-label="SLA policies for this company">
          {sla.policies.map((policy) => (
            <li key={policy.id} className="text-xs" data-testid="company-sla-policy">
              <div className="flex items-center gap-1.5">
                <span className="truncate font-medium">{policy.name}</span>
                {policy.custom ? <span className="shrink-0 text-[11px] text-muted-foreground">Custom</span> : null}
              </div>
              <div className="text-muted-foreground">{policyTargets(policy)}</div>
            </li>
          ))}
        </ul>
      )}
      <p className="mt-2 text-[11px] text-muted-foreground">The first policy that matches a conversation applies.</p>

      {canManage ? (
        <div className="mt-2 flex flex-wrap gap-x-3 gap-y-1">
          {sla.has_custom ? (
            <QuietTextAction className="h-auto px-0 text-xs" disabled={removeCustom.isPending} onClick={() => void remove()}>Remove custom SLA</QuietTextAction>
          ) : sla.policies.length > 0 ? (
            <QuietTextAction className="h-auto px-0 text-xs" disabled={customize.isPending} onClick={() => customize.mutate(companyId, {
              onSuccess: () => toast.success(`Custom SLA created for ${companyName}`, { description: 'It applies to their new conversations. Adjust its policies in SLA settings.' }),
            })}>
              {customize.isPending ? 'Creating…' : 'Customize for this company'}
            </QuietTextAction>
          ) : null}
          <a className="text-xs text-muted-foreground underline-offset-2 hover:text-foreground hover:underline" href={settingsHref}>Edit policies</a>
        </div>
      ) : null}
    </section>
  )
}
