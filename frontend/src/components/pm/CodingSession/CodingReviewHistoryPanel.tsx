import { AgentRunArtifactView } from '@/components/pm/AgentRunArtifactView';
import type { AgentRunArtifact } from '@/lib/pmTypes';
import { formatCodingSessionRelative } from './codingSessionUtils';

export interface CodingReviewHistoryItem {
  artifact: AgentRunArtifact;
  decisionArtifact?: AgentRunArtifact | null;
}

export function CodingReviewHistoryPanel({ reviewArtifacts }: { reviewArtifacts: CodingReviewHistoryItem[] }) {
  if (reviewArtifacts.length === 0) return null;

  return (
    <section className="min-w-0 overflow-hidden rounded-xl border border-border bg-card shadow-sm" data-coding-session-review-history-panel>
      <div className="flex items-center justify-between border-b border-border px-4 py-3">
        <div className="text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
          Review history
        </div>
        <div className="text-[11px] text-muted-foreground">
          {reviewArtifacts.length} {reviewArtifacts.length === 1 ? 'review' : 'reviews'}
        </div>
      </div>
      <div className="min-w-0 space-y-3 p-3">
        {reviewArtifacts.map(({ artifact, decisionArtifact }) => (
          <div key={artifact.id} className="min-w-0 space-y-2">
            <div className="px-1 text-[11px] text-muted-foreground">
              {formatCodingSessionRelative(artifact.created_at)}
            </div>
            <AgentRunArtifactView artifact={artifact} reviewDecisionArtifact={decisionArtifact} maxContentHeight="max-h-96" />
          </div>
        ))}
      </div>
    </section>
  );
}
