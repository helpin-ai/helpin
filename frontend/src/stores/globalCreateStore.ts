import { create } from 'zustand';

type CreateModal = 'story' | 'epic' | 'sprint' | 'objective' | null;

interface GlobalCreateState {
  activeModal: CreateModal;
  initialTeamId: string | undefined;
  openCreate: (modal: Exclude<CreateModal, null>, options?: { teamId?: string }) => void;
  closeCreate: () => void;
}

export const useGlobalCreateStore = create<GlobalCreateState>((set) => ({
  activeModal: null,
  initialTeamId: undefined,
  openCreate: (modal, options) => set({ activeModal: modal, initialTeamId: options?.teamId }),
  closeCreate: () => set({ activeModal: null, initialTeamId: undefined }),
}));
