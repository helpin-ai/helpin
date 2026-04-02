import type { UpdateTaskRequest } from '@/lib/pmTypes';

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
