import type { RunPlanArtifact } from '@/lib/pmTypes';

export function shouldShowCodingSessionPlanPanel(plan: RunPlanArtifact | null | undefined): boolean {
  return (plan?.plan?.length ?? 0) > 0;
}

export function shouldShowCodingSessionSidePanel(
  plan: RunPlanArtifact | null | undefined,
  previewCount: number,
): boolean {
  return shouldShowCodingSessionPlanPanel(plan) || previewCount > 0;
}

export function shouldShowFailedCodingSessionRecoveryNotice(
  status: string | null | undefined,
  plan: RunPlanArtifact | null | undefined,
  previewCount: number,
): boolean {
  return status === 'failed' && shouldShowCodingSessionSidePanel(plan, previewCount);
}
