import { create } from 'zustand';

type CreateModal = 'story' | 'epic' | 'sprint' | 'objective' | null;

interface GlobalCreateState {
  activeModal: CreateModal;
  openCreate: (modal: Exclude<CreateModal, null>) => void;
  closeCreate: () => void;
}

export const useGlobalCreateStore = create<GlobalCreateState>((set) => ({
  activeModal: null,
  openCreate: (modal) => set({ activeModal: modal }),
  closeCreate: () => set({ activeModal: null }),
}));
