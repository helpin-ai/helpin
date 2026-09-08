import { QuietTextAction } from '@/components/design-system/quiet';
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet';
import { useCRMInboxRecommendation } from '@/hooks/queries/useCRMSituations';
import { PlaybookError, PlaybookLoading } from '../playbooks/PlaybookUI';
import { RecommendationActions } from './SignalActions';
import { RecommendationEvidence } from './SignalContext';

export function RecommendationDrawer({ ws, slug, id, canEdit, onClose, onSignal }: { ws: string; slug: string; id: string; canEdit: boolean; onClose: () => void; onSignal: (id: string) => void }) {
  const detail = useCRMInboxRecommendation(ws, id);
  return <Sheet open onOpenChange={(open) => { if (!open) onClose(); }}><SheetContent className="overflow-y-auto data-[side=right]:w-full data-[side=right]:sm:max-w-[600px]">
    <SheetHeader className="pr-14"><SheetTitle>Review recommendation</SheetTitle><SheetDescription className="sr-only">Review the proposed action and its source evidence.</SheetDescription></SheetHeader>
    <div className="px-6 pb-6">{detail.isPending ? <PlaybookLoading /> : detail.isError ? <PlaybookError error={detail.error} retry={() => void detail.refetch()} /> : <>
      {/* Never swap the drawer while a decision is in flight. The canonical guard also
          checks concurrent projection and lifecycle changes before claiming an action. */}
      {detail.data.linked_situations.length > 0 && <div className="border-b border-quiet-divider-strong pb-4"><p className="mb-2 text-sm text-quiet-text-secondary">This recommendation is now part of a signal.</p>{detail.data.linked_situations.map((signalId, index) => <QuietTextAction key={signalId} onClick={() => onSignal(signalId)}>Open linked signal{detail.data.linked_situations.length > 1 ? ` ${index + 1}` : ''}</QuietTextAction>)}</div>}
      <RecommendationActions ws={ws} slug={slug} actions={[detail.data.action]} canEdit={canEdit} blockedReason={detail.data.linked_situations.length ? 'Continue in the linked signal to review this action.' : undefined} />
      <RecommendationEvidence ws={ws} slug={slug} canEdit={canEdit} evidence={(detail.data.action.signals ?? []).map((source) => ({ ...source, evidence_identity_trust: source.evidence_identity_trust || 'unverified' }))} />
    </>}</div>
  </SheetContent></Sheet>;
}
