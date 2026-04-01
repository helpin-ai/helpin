import { createContext } from 'react';
import type { Agent, Story } from '@/lib/pmTypes';
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
  onStoryPatched: (story: Story) => void;
  onOpen: (story: Story) => void;
  onCreate: (id: string) => void;
  onCreateForMember: (memberId: string | null) => void;
  onToggleCollapse: (id: string) => void;
  onLoadMore: (id: string) => void;
  onLoadMoreMember: (memberId: string | null) => void;
}

export const BoardDataContext = createContext<BoardDataContextValue | null>(null);
export const BoardCallbacksContext = createContext<BoardCallbacksContextValue | null>(null);
export const DragPreviewContext = createContext<DragPreviewManager | null>(null);
