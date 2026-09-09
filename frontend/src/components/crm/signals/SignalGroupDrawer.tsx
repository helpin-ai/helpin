import { Link } from '@tanstack/react-router';
import { QuietSection } from '@/components/design-system/quiet';
import { SignalArrival, SignalDrawerLayout, repeatsSignalText } from './SignalDrawerLayout';
import { useCRMInboxSignalGroup } from '@/hooks/queries/useCRMSituations';
import { PlaybookError, PlaybookLoading } from '../playbooks/PlaybookUI';
import { RecommendationEvidence } from './SignalContext';
import type { CRMSignalAccountStory } from '@/lib/crmTypes';

// Reading source evidence never creates a situation or enrolls a playbook.
export function SignalGroupDrawer({ ws, slug, id, canEdit, onClose }: { ws: string; slug: string; id: string; canEdit: boolean; onClose: () => void }) {
  const detail = useCRMInboxSignalGroup(ws, id);
  const group = detail.data;
  const summary = group?.signals[0]?.summary;
  const nextStep = group?.needs_judgment ? 'Review the conflicting evidence before choosing a next step.' : group?.recommended_action_label || 'Review the evidence and agree the customer’s next step.';
  return <SignalDrawerLayout title="Customer signal" description="Review the customer evidence and suggested next step." onClose={onClose}>
    {detail.isPending ? <PlaybookLoading /> : detail.isError ? <PlaybookError error={detail.error} retry={() => void detail.refetch()} /> : <>
      <div className="mb-4 flex flex-wrap items-center justify-between gap-2 text-sm"><SignalCustomer slug={slug} group={detail.data} /><SignalArrival at={detail.data.latest_detected_at || detail.data.signals[0]?.detected_at} label={detail.data.signals.length > 1 ? 'Latest signal' : 'Received'} /></div>
      {summary && <p className="text-sm leading-6 text-quiet-text-primary">{summary}</p>}
      {!repeatsSignalText(nextStep, [summary]) && <QuietSection className="px-0 py-4 sm:px-0 lg:px-0" title="Suggested next step">
        <p className={detail.data.needs_judgment ? 'text-sm leading-6 text-quiet-accent' : 'text-sm font-medium leading-6'}>{nextStep}</p>
      </QuietSection>}
      <RecommendationEvidence ws={ws} slug={slug} canEdit={canEdit} displayedText={[summary, nextStep]} evidence={detail.data.signals.map((source) => ({ ...source, evidence_identity_trust: source.evidence_identity_trust || 'unverified' }))} />
    </>}
  </SignalDrawerLayout>;

}

function SignalCustomer({ slug, group }: { slug: string; group: CRMSignalAccountStory }) {
  const className = 'text-quiet-accent hover:underline';
  if (group.entity_type === 'company') return <Link className={className} to="/w/$slug/crm/companies/$companyId" params={{ slug, companyId: group.entity_id }}>{group.account_name}</Link>;
  if (group.entity_type === 'contact') return <Link className={className} to="/w/$slug/crm/contacts/$contactId" params={{ slug, contactId: group.entity_id }}>{group.account_name}</Link>;
  if (group.entity_type === 'deal') return <Link className={className} to="/w/$slug/crm/deals/$dealId" params={{ slug, dealId: group.entity_id }}>{group.account_name}</Link>;
  return <span>{group.account_name || 'Customer unavailable'}</span>;
}
