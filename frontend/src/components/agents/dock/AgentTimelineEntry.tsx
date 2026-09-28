import type { CodingSessionActor, CodingSessionTranscriptMessage, AgentRunPauseReason } from '@/lib/pmTypes';
import type { TranscriptSegment } from '@/components/agents/transcript';
import type { RenderSegmentOptions } from '@/components/agents/transcript/segmentRenderers';
import { DockActivityTimeline } from './DockActivityTimeline';
import { DockAnswerSegment } from './DockAnswerSegment';
import { DockDecisionRow } from './DockDecisionRow';
import type { DockWorkingGroupEntry } from './dockWorkingGroups';
import type { AgentLiveProgress } from './agentProgress';
import { cn } from '@/lib/utils';

/** Shared timeline presentation for the dock and full agent run sheet. */
export function AgentTimelineEntry({
  segment,
  workingGroup,
  liveProgress,
  runStatus,
  pauseReason,
  resolveActor,
  animate = false,
  separator = false,
  options,
}: {
  segment: TranscriptSegment;
  workingGroup?: DockWorkingGroupEntry;
  liveProgress?: AgentLiveProgress | null;
  runStatus?: string;
  pauseReason?: AgentRunPauseReason;
  resolveActor: (message: CodingSessionTranscriptMessage) => CodingSessionActor | null;
  animate?: boolean;
  separator?: boolean;
  options?: Partial<RenderSegmentOptions>;
}) {
  if (workingGroup) return <DockActivityTimeline progress={liveProgress} group={workingGroup} runStatus={runStatus} pauseReason={pauseReason} resolveActor={resolveActor} />;
  if (segment.kind === 'review_decision') return <DockDecisionRow message={segment.message} actor={resolveActor(segment.message)} />;
  return <div className={cn(separator && 'mt-3 border-t border-border/60 pt-4')}>
    <DockAnswerSegment
      segment={segment}
      animate={animate}
      options={{
        expandable: true,
        showToolContext: true,
        userPresentation: 'signature',
        fallbackUserLabel: 'You',
        resolveActor,
        ...options,
      }}
    />
  </div>;
}
