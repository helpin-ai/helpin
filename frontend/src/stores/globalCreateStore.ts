import { create } from 'zustand';

type CreateModal = 'story' | 'epic' | 'sprint' | 'objective' | 'docs_document' | 'docs_space' | 'docs_collection' | null;

interface GlobalCreateState {
  activeModal: CreateModal;
  initialTeamId: string | undefined;
  /** Optional space ID context for docs collection creation */
  initialSpaceId: string | undefined;
  openCreate: (modal: Exclude<CreateModal, null>, options?: { teamId?: string; spaceId?: string }) => void;
  closeCreate: () => void;
}

export const useGlobalCreateStore = create<GlobalCreateState>((set) => ({
  activeModal: null,
  initialTeamId: undefined,
  initialSpaceId: undefined,
  openCreate: (modal, options) => set({ activeModal: modal, initialTeamId: options?.teamId, initialSpaceId: options?.spaceId }),
  closeCreate: () => set({ activeModal: null, initialTeamId: undefined, initialSpaceId: undefined }),
}));
