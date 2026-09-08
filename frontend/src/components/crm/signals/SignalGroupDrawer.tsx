import { Link } from '@tanstack/react-router';
import { QuietSection } from '@/components/design-system/quiet';
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet';
import { useCRMInboxSignalGroup } from '@/hooks/queries/useCRMSituations';
import { PlaybookError, PlaybookLoading } from '../playbooks/PlaybookUI';
import { RecommendationEvidence } from './SignalContext';
import type { CRMSignalAccountStory } from '@/lib/crmTypes';

// Reading source evidence never creates a situation or enrolls a playbook.
export function SignalGroupDrawer({ ws, slug, id, canEdit, onClose }: { ws: string; slug: string; id: string; canEdit: boolean; onClose: () => void }) {
  const detail = useCRMInboxSignalGroup(ws, id);
  return <Sheet open onOpenChange={(open) => { if (!open) onClose(); }}><SheetContent className="overflow-y-auto data-[side=right]:w-full data-[side=right]:sm:max-w-[600px]">
    <SheetHeader className="pr-14"><SheetTitle>Customer signal</SheetTitle><SheetDescription className="sr-only">Review the customer evidence and suggested next step.</SheetDescription></SheetHeader>
    <div className="px-6 pb-6">{detail.isPending ? <PlaybookLoading /> : detail.isError ? <PlaybookError error={detail.error} retry={() => void detail.refetch()} /> : <>
      <div className="pb-4 text-sm"><SignalCustomer slug={slug} group={detail.data} /></div>
      <p className="text-sm leading-7 text-quiet-text-primary">{detail.data.signals[0]?.summary}</p>
      <QuietSection className="px-0 sm:px-0 lg:px-0" title="Suggested next step">
        <p className="text-sm leading-7">{detail.data.needs_judgment ? 'Review the conflicting evidence before choosing a next step.' : detail.data.recommended_action_label || 'Review the evidence and agree the customer’s next step.'}</p>
      </QuietSection>
      <RecommendationEvidence ws={ws} slug={slug} canEdit={canEdit} evidence={detail.data.signals.map((source) => ({ ...source, evidence_identity_trust: source.evidence_identity_trust || 'unverified' }))} />
    </>}</div>
  </SheetContent></Sheet>;
}

function SignalCustomer({ slug, group }: { slug: string; group: CRMSignalAccountStory }) {
  const className = 'text-quiet-accent hover:underline';
  if (group.entity_type === 'company') return <Link className={className} to="/w/$slug/crm/companies/$companyId" params={{ slug, companyId: group.entity_id }}>{group.account_name}</Link>;
  if (group.entity_type === 'contact') return <Link className={className} to="/w/$slug/crm/contacts/$contactId" params={{ slug, contactId: group.entity_id }}>{group.account_name}</Link>;
  if (group.entity_type === 'deal') return <Link className={className} to="/w/$slug/crm/deals/$dealId" params={{ slug, dealId: group.entity_id }}>{group.account_name}</Link>;
  return <span>{group.account_name || 'Customer unavailable'}</span>;
}
