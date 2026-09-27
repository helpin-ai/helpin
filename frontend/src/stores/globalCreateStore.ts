import { create } from 'zustand';

type CreateModal = 'task' | 'epic' | 'sprint' | 'objective' | 'docs_document' | 'docs_space' | 'docs_collection' | 'crm_contact' | 'crm_company' | 'crm_deal' | null;

export interface ObjectiveCreateDates { startDate: string; endDate: string }

interface GlobalCreateState {
  initialObjectiveDates: ObjectiveCreateDates | undefined;
  activeModal: CreateModal;
  initialTeamId: string | undefined;
  initialOwnerMemberId: string | undefined;
  initialSprintId: string | undefined;
  /** Optional space ID context for docs collection creation */
  initialSpaceId: string | undefined;
  /** Optional collection ID context for docs document creation */
  initialCollectionId: string | undefined;
  /**
   * Optional parent collection ID for docs collection creation. When
   * set, the create-collection dialog opens with this collection
   * preselected as the parent so the new row lands as a sub-collection.
   * Left undefined for top-level collection creates.
   */
  initialParentCollectionId: string | undefined;
  openCreate: (modal: Exclude<CreateModal, null>, options?: { objectiveDates?: ObjectiveCreateDates; teamId?: string; ownerMemberId?: string; sprintId?: string; spaceId?: string; collectionId?: string; parentCollectionId?: string }) => void;
  closeCreate: () => void;
}

export const useGlobalCreateStore = create<GlobalCreateState>((set) => ({
  activeModal: null,
  initialObjectiveDates: undefined,
  initialTeamId: undefined,
  initialOwnerMemberId: undefined,
  initialSprintId: undefined,
  initialSpaceId: undefined,
  initialCollectionId: undefined,
  initialParentCollectionId: undefined,
  openCreate: (modal, options) => set({ activeModal: modal, initialObjectiveDates: options?.objectiveDates, initialTeamId: options?.teamId, initialOwnerMemberId: options?.ownerMemberId, initialSprintId: options?.sprintId, initialSpaceId: options?.spaceId, initialCollectionId: options?.collectionId, initialParentCollectionId: options?.parentCollectionId }),
  closeCreate: () => set({ activeModal: null, initialObjectiveDates: undefined, initialTeamId: undefined, initialOwnerMemberId: undefined, initialSprintId: undefined, initialSpaceId: undefined, initialCollectionId: undefined, initialParentCollectionId: undefined }),
}));
