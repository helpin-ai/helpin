import { useCallback, useState } from 'react';
import type { AnnotationShape } from './annotationTypes';

const MAX_HISTORY = 50;

interface HistoryState {
  revisions: AnnotationShape[][];
  index: number;
}

export interface AnnotationHistory {
  shapes: AnnotationShape[];
  canUndo: boolean;
  canRedo: boolean;
  /** Commits a new revision. Pass a function to derive from the current shapes. */
  commit: (next: AnnotationShape[] | ((current: AnnotationShape[]) => AnnotationShape[])) => void;
  undo: () => void;
  redo: () => void;
  reset: (shapes: AnnotationShape[]) => void;
}

/**
 * Undo/redo over the shapes array. The whole array is snapshotted per revision — shape counts
 * here are small (tens), so a diff-based approach would be complexity without benefit.
 *
 * Revisions and the cursor live in one state object so every transition is a pure functional
 * update; keeping them apart would need refs read during render.
 */
export function useAnnotationHistory(initial: AnnotationShape[]): AnnotationHistory {
  const [state, setState] = useState<HistoryState>({ revisions: [initial], index: 0 });

  const commit = useCallback(
    (next: AnnotationShape[] | ((current: AnnotationShape[]) => AnnotationShape[])) => {
      setState((current) => {
        const shapes = current.revisions[current.index] ?? [];
        const resolved = typeof next === 'function' ? next(shapes) : next;
        // Drop any redo branch, then cap the stack from the front.
        const kept = current.revisions.slice(0, current.index + 1).concat([resolved]);
        const trimmed = kept.length > MAX_HISTORY ? kept.slice(kept.length - MAX_HISTORY) : kept;
        return { revisions: trimmed, index: trimmed.length - 1 };
      });
    },
    [],
  );

  const undo = useCallback(() => {
    setState((current) => ({ ...current, index: Math.max(0, current.index - 1) }));
  }, []);

  const redo = useCallback(() => {
    setState((current) => ({
      ...current,
      index: Math.min(current.revisions.length - 1, current.index + 1),
    }));
  }, []);

  const reset = useCallback((shapes: AnnotationShape[]) => {
    setState({ revisions: [shapes], index: 0 });
  }, []);

  return {
    shapes: state.revisions[state.index] ?? [],
    canUndo: state.index > 0,
    canRedo: state.index < state.revisions.length - 1,
    commit,
    undo,
    redo,
    reset,
  };
}
