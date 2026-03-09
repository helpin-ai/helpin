import { useNavigate } from '@tanstack/react-router';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { SequenceDetailView } from '@/components/crm/SequenceDetail';
import { useTitle } from '@/hooks/useTitle';

export function SequenceDetailPage({ sequenceId }: { sequenceId: string }) {
  const { currentWorkspace } = useWorkspaceStore();
  const wsId = currentWorkspace?.id ?? '';
  const wsSlug = currentWorkspace?.slug ?? '';
  const navigate = useNavigate();

  useTitle('Sequence');

  return (
    <div className="mx-auto max-w-5xl px-4 md:px-6 pt-4">
      <SequenceDetailView
        workspaceId={wsId}
        sequenceId={sequenceId}
        onBack={() => navigate({ to: '/w/$slug/crm/sequences', params: { slug: wsSlug } })}
      />
    </div>
  );
}
