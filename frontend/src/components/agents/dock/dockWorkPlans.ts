import type { CodingSessionStreamState, RunPlanArtifact } from '@/lib/pmTypes';

export function hasWorkPlanOrigin(plan?: RunPlanArtifact | null): boolean {
  return !!plan?.origin?.event_id && Number.isFinite(Date.parse(plan.origin.created_at));
}

export function dockWorkPlans(stream: CodingSessionStreamState, saved: RunPlanArtifact[] = []): RunPlanArtifact[] {
  const byOrigin = new Map<string, RunPlanArtifact>();
  for (const plan of [...saved, ...(stream.work_plans ?? []), ...(stream.current_plan ? [stream.current_plan] : [])]) {
    if (hasWorkPlanOrigin(plan)) byOrigin.set(plan.origin!.event_id, plan);
  }
  // Do not surface every old plan above a paginated slice of newer messages.
  const first = stream.transcript_messages[0];
  const oldestVisible = first ? Date.parse(first.timestamp) : NaN;
  return [...byOrigin.values()]
    .filter(plan => !Number.isFinite(oldestVisible) || Date.parse(plan.origin!.created_at) >= oldestVisible)
    .sort((a,b) => Date.parse(a.origin!.created_at) - Date.parse(b.origin!.created_at));
}

export function dockPlanBoundary(plan: RunPlanArtifact, entries: { timestamp: number | null; user: boolean; pending?: boolean }[]) {
  const created = Date.parse(plan.origin!.created_at);
  const later = entries.findIndex(entry => entry.pending || (entry.timestamp !== null && entry.timestamp > created));
  return {
    index: later < 0 ? entries.length : later,
    collapsed: entries.some(entry => entry.user && (entry.pending || (entry.timestamp !== null && entry.timestamp > created))),
  };
}
