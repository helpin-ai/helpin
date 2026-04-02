import { createContext, type MutableRefObject } from 'react';
import type { Agent, Task } from '@/lib/pmTypes';
import type { AssignableMember } from '@/lib/types';
import type { DragPreviewManager } from './KanbanBoard.dnd';

export interface BoardDataContextValue {
  workspaceId: string;
  ownerNameMap: Map<string, string>;
  agentById: Map<string, Agent>;
  assignableMembers: AssignableMember[];
  automatedStateIds: Set<string>;
  findTeamName: (teamId: string | undefined) => string | undefined;
}

export interface BoardCallbacksContextValue {
  onTaskPatched: (task: Task) => void;
  onOpen: (task: Task) => void;
  onCreate: (id: string) => void;
  onCreateForMember: (memberId: string | null) => void;
  onToggleCollapse: (id: string) => void;
  onLoadMore: (id: string) => void;
  onLoadMoreMember: (memberId: string | null) => void;
}

/** Holds stable data that changes infrequently (members, agents). Re-renders consumers on change. */
export const BoardDataContext = createContext<BoardDataContextValue | null>(null);
/**
 * Holds a REF to callbacks — the ref identity never changes so consumers never
 * re-render from callback identity shifts. Read via `ref.current.onOpen(...)`.
 */
export const BoardCallbacksContext = createContext<MutableRefObject<BoardCallbacksContextValue> | null>(null);
export const DragPreviewContext = createContext<DragPreviewManager | null>(null);
