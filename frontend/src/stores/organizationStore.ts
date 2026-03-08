import { create } from 'zustand';
import type { OrganizationWithRole } from '@/lib/types';

interface OrganizationState {
  currentOrganization: OrganizationWithRole | null;
  setCurrentOrganization: (org: OrganizationWithRole) => void;
}

const STORAGE_KEY = 'current_organization_id';

export const useOrganizationStore = create<OrganizationState>((set) => ({
  currentOrganization: null,

  setCurrentOrganization: (org: OrganizationWithRole) => {
    localStorage.setItem(STORAGE_KEY, org.id);
    set({ currentOrganization: org });
  },
}));
