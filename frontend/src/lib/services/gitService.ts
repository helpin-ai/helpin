import { api } from '../api';
import type {
  GitIntegration,
  GitHubInstallURLResponse,
  GitRepository,
  TaskDeliveryTarget,
  TaskGitLink,
  CreateGitIntegrationRequest,
  CreateBranchRequest,
  UpdateGitRepositoryRequest,
  UpdateTaskDeliveryTargetRequest,
} from '../pmTypes';

const qs = (workspaceId: string) => `?workspace_id=${encodeURIComponent(workspaceId)}`;

export const gitService = {
  getGitHubInstallURL: (workspaceId: string) =>
    api.get<GitHubInstallURLResponse>(`/git/github/install-url${qs(workspaceId)}`),
  listIntegrations: (workspaceId: string) =>
    api.get<GitIntegration[]>(`/git/integrations${qs(workspaceId)}`),
  createIntegration: (workspaceId: string, payload: CreateGitIntegrationRequest) =>
    api.post<GitIntegration>(`/git/integrations${qs(workspaceId)}`, payload),
  syncRepositories: (workspaceId: string, integrationId: string) =>
    api.post<GitRepository[]>(`/git/integrations/${integrationId}/sync${qs(workspaceId)}`, {}),
  listRepositories: (workspaceId: string, options?: { all?: boolean }) =>
    api.get<GitRepository[]>(`/git/repositories${qs(workspaceId)}${options?.all ? '&all=true' : ''}`),
  updateRepository: (workspaceId: string, repoId: string, payload: UpdateGitRepositoryRequest) =>
    api.put<GitRepository>(`/git/repositories/${repoId}${qs(workspaceId)}`, payload),
  getTaskGitLinks: (workspaceId: string, taskId: string) =>
    api.get<TaskGitLink[]>(`/pm/tasks/${taskId}/git-links${qs(workspaceId)}`),
  getTaskDeliveryTarget: (workspaceId: string, taskId: string) =>
    api.get<TaskDeliveryTarget>(`/pm/tasks/${taskId}/delivery-target${qs(workspaceId)}`),
  updateTaskDeliveryTarget: (workspaceId: string, taskId: string, payload: UpdateTaskDeliveryTargetRequest) =>
    api.put<TaskDeliveryTarget>(`/pm/tasks/${taskId}/delivery-target${qs(workspaceId)}`, payload),
  createBranch: (workspaceId: string, taskId: string, payload: CreateBranchRequest) =>
    api.post<TaskGitLink>(`/pm/tasks/${taskId}/create-branch${qs(workspaceId)}`, payload),
  /** @deprecated Use getTaskGitLinks instead */
  getStoryGitLinks: (workspaceId: string, taskId: string) =>
    api.get<TaskGitLink[]>(`/pm/tasks/${taskId}/git-links${qs(workspaceId)}`),
  /** @deprecated Use getTaskDeliveryTarget instead */
  getStoryDeliveryTarget: (workspaceId: string, taskId: string) =>
    api.get<TaskDeliveryTarget>(`/pm/tasks/${taskId}/delivery-target${qs(workspaceId)}`),
  /** @deprecated Use updateTaskDeliveryTarget instead */
  updateStoryDeliveryTarget: (workspaceId: string, taskId: string, payload: UpdateTaskDeliveryTargetRequest) =>
    api.put<TaskDeliveryTarget>(`/pm/tasks/${taskId}/delivery-target${qs(workspaceId)}`, payload),
};
