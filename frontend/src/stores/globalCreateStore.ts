import { create } from 'zustand';

type CreateModal = 'story' | 'epic' | 'sprint' | 'objective' | 'docs_document' | 'docs_space' | 'docs_collection' | 'crm_contact' | 'crm_company' | 'crm_deal' | null;

interface GlobalCreateState {
  activeModal: CreateModal;
  initialTeamId: string | undefined;
  /** Optional space ID context for docs collection creation */
  initialSpaceId: string | undefined;
  /** Optional collection ID context for docs document creation */
  initialCollectionId: string | undefined;
  openCreate: (modal: Exclude<CreateModal, null>, options?: { teamId?: string; spaceId?: string; collectionId?: string }) => void;
  closeCreate: () => void;
}

export const useGlobalCreateStore = create<GlobalCreateState>((set) => ({
  activeModal: null,
  initialTeamId: undefined,
  initialSpaceId: undefined,
  initialCollectionId: undefined,
  openCreate: (modal, options) => set({ activeModal: modal, initialTeamId: options?.teamId, initialSpaceId: options?.spaceId, initialCollectionId: options?.collectionId }),
  closeCreate: () => set({ activeModal: null, initialTeamId: undefined, initialSpaceId: undefined, initialCollectionId: undefined }),
}));
