export function getStateBoardPreviewInsertIndex({
  toStateType,
  overId,
  toStateId,
  overIdx,
  columnLength,
  pointerBelowMid,
}: {
  toStateType?: string;
  overId: string;
  toStateId: string;
  overIdx: number;
  columnLength: number;
  pointerBelowMid: boolean;
}) {
  if (toStateType === 'done') {
    return 0;
  }
  if (overId === toStateId) {
    return columnLength;
  }
  if (overIdx < 0) {
    return columnLength;
  }
  return pointerBelowMid ? overIdx + 1 : overIdx;
}

export function getSameStateBoardDropIndex({
  overId,
  stateId,
  overIndex,
  columnLength,
}: {
  overId: string;
  stateId: string;
  overIndex: number;
  columnLength: number;
}) {
  if (overId === stateId) {
    return Math.max(0, columnLength - 1);
  }
  if (overIndex < 0) {
    return Math.max(0, columnLength - 1);
  }
  return overIndex;
}

export function commitDropBeforeClearingPreview<T>({
  commit,
  clearPreview,
}: {
  commit: () => Promise<T>;
  clearPreview: () => void;
}) {
  try {
    const commitPromise = commit();
    clearPreview();
    return commitPromise;
  } catch (error) {
    clearPreview();
    throw error;
  }
}
