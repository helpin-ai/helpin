import { create } from 'zustand';
import type { OrganizationWithRole } from '@/lib/types';
import { organizationsService } from '@/lib/services/organizationsService';

interface OrganizationState {
  organizations: OrganizationWithRole[];
  currentOrganization: OrganizationWithRole | null;
  loading: boolean;
  error: string | null;
  loadOrganizations: () => Promise<void>;
  setCurrentOrganization: (org: OrganizationWithRole) => void;
}

const STORAGE_KEY = 'current_organization_id';

export const useOrganizationStore = create<OrganizationState>((set, get) => ({
  organizations: [],
  currentOrganization: null,
  loading: false,
  error: null,

  loadOrganizations: async () => {
    set({ loading: true });
    const { data, error } = await organizationsService.list();
    const orgs = data ?? [];

    // Restore or re-validate previously selected org.
    const savedId = get().currentOrganization?.id ?? localStorage.getItem(STORAGE_KEY);
    let current: OrganizationWithRole | null = null;
    if (orgs.length > 0) {
      current = (savedId ? orgs.find((o) => o.id === savedId) : null) ?? orgs[0];
    }

    set({
      organizations: orgs,
      currentOrganization: current,
      error: error ?? null,
      loading: false,
    });
  },

  setCurrentOrganization: (org: OrganizationWithRole) => {
    localStorage.setItem(STORAGE_KEY, org.id);
    set({ currentOrganization: org });
  },
}));
