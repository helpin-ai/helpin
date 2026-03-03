import { create } from 'zustand';
import type { EpicWorkflowState, WorkflowWithStates } from '@/lib/pmTypes';
import { pmWorkflowService } from '@/lib/services/pmWorkflowService';

interface PMWorkflowState {
  workflows: WorkflowWithStates[];
  currentWorkflow: WorkflowWithStates | null;
  epicStates: EpicWorkflowState[];
  loading: boolean;
  error: string | null;
  loadWorkflows: (workspaceId: string) => Promise<void>;
  selectWorkflow: (workflowId: string) => void;
  loadEpicStates: (workspaceId: string) => Promise<void>;
}

export const usePMWorkflowStore = create<PMWorkflowState>((set, get) => ({
  workflows: [],
  currentWorkflow: null,
  epicStates: [],
  loading: false,
  error: null,

  loadWorkflows: async (workspaceId: string) => {
    set({ loading: true, error: null });
    const { data, error } = await pmWorkflowService.list(workspaceId);
    if (error || !data) {
      set({ loading: false, error: error ?? 'Failed to load workflows' });
      return;
    }
    const existing = get().currentWorkflow;
    const current = existing
      ? data.find((workflow) => workflow.workflow.id === existing.workflow.id) ?? data[0] ?? null
      : data[0] ?? null;
    set({ workflows: data, currentWorkflow: current, loading: false });
  },

  selectWorkflow: (workflowId: string) => {
    const next = get().workflows.find((workflow) => workflow.workflow.id === workflowId) ?? null;
    set({ currentWorkflow: next });
  },

  loadEpicStates: async (workspaceId: string) => {
    const { data, error } = await pmWorkflowService.listEpicStates(workspaceId);
    if (error || !data) {
      set({ error: error ?? 'Failed to load epic states' });
      return;
    }
    set({ epicStates: data });
  },
}));
