import type { UpdateStoryRequest } from '@/lib/pmTypes';

export function getFlushablePendingTaskPatch(
  pendingPatch: UpdateStoryRequest,
  descriptionPendingUploads: number,
): UpdateStoryRequest | null {
  if (Object.keys(pendingPatch).length === 0) {
    return null;
  }

  if (pendingPatch.description !== undefined && descriptionPendingUploads > 0) {
    return null;
  }

  return pendingPatch;
}

export function hasPendingTaskSave(
  pendingPatch: UpdateStoryRequest,
  descriptionPendingUploads: number,
): boolean {
  return getFlushablePendingTaskPatch(pendingPatch, descriptionPendingUploads) !== null;
}
