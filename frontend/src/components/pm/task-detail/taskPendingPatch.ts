import type { UpdateTaskRequest } from '@/lib/pmTypes';

export function taskPatchSignature(patch: UpdateTaskRequest): string {
  return JSON.stringify(
    Object.entries(patch).sort(([left], [right]) => left.localeCompare(right)),
  );
}

export function isBlockedFailedTaskPatch(
  pendingPatch: UpdateTaskRequest,
  failedPatchSignature: string | null,
): boolean {
  return failedPatchSignature === taskPatchSignature(pendingPatch);
}

export function getFlushablePendingTaskPatch(
  pendingPatch: UpdateTaskRequest,
  descriptionPendingUploads: number,
): UpdateTaskRequest | null {
  if (Object.keys(pendingPatch).length === 0) {
    return null;
  }

  if (pendingPatch.description !== undefined && descriptionPendingUploads > 0) {
    return null;
  }

  return pendingPatch;
}

export function hasPendingTaskSave(
  pendingPatch: UpdateTaskRequest,
  descriptionPendingUploads: number,
): boolean {
  return getFlushablePendingTaskPatch(pendingPatch, descriptionPendingUploads) !== null;
}
