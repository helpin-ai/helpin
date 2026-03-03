import { create } from 'zustand';
import type { WorkspaceMember } from '@/lib/types';
import { workspacesService } from '@/lib/services/workspacesService';

interface SessionState {
  membership: WorkspaceMember | null;
  loading: boolean;
  loadMembership: (workspaceId: string) => Promise<void>;
  isAdmin: () => boolean;
  isManager: () => boolean;
  isOwner: () => boolean;
  canEdit: () => boolean;
}

export const useSessionStore = create<SessionState>((set, get) => ({
  membership: null,
  loading: false,

  loadMembership: async (workspaceId: string) => {
    set({ loading: true });
    const { data } = await workspacesService.getMyMembership(workspaceId);
    if (data) set({ membership: data });
    set({ loading: false });
  },

  isOwner: () => get().membership?.role === 'owner',
  isAdmin: () => ['owner', 'admin'].includes(get().membership?.role || ''),
  isManager: () => ['owner', 'admin', 'manager'].includes(get().membership?.role || ''),
  canEdit: () => ['owner', 'admin', 'manager'].includes(get().membership?.role || ''),
}));
