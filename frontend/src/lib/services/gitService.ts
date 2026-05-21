import { API_BASE, api } from '../api';
import type {
  GitIntegration,
  GitIntegrationDetail,
  GitHubInstallURLResponse,
  GitBranch,
  GitAvailableRepo,
  GitRepository,
  TaskDeliveryTarget,
  TaskGitLink,
  CreateGitIntegrationRequest,
  CreateBranchRequest,
  UpdateGitRepositoryRequest,
  UpdateTaskDeliveryTargetRequest,
  WireGitRepositoriesConflictResponse,
  WireGitRepositoriesRequest,
  WireGitRepositoriesResponse,
} from '../pmTypes';

const qs = (workspaceId: string) => `?workspace_id=${encodeURIComponent(workspaceId)}`;

async function gitRawRequest<T>(path: string, options: RequestInit = {}) {
  try {
    const res = await fetch(`${API_BASE}${path}`, {
      ...options,
      credentials: 'include',
      headers: {
        'Content-Type': 'application/json',
        ...options.headers,
      },
    });

    if (res.status === 204) {
      return { data: null as T, error: null, status: res.status };
    }
    const json = await res.json().catch(() => null);
    if (!res.ok) {
      return {
        data: json as T | null,
        error: (json as { error?: string } | null)?.error || res.statusText,
        status: res.status,
      };
    }
    return { data: json as T, error: null, status: res.status };
  } catch (error) {
    return {
      data: null as T | null,
      error: error instanceof Error ? error.message : 'Network error',
      status: 0,
    };
  }
}

export const gitService = {
  getGitHubInstallURL: (workspaceId: string, options?: { forceInstall?: boolean }) =>
    api.get<GitHubInstallURLResponse>(`/git/github/install-url${qs(workspaceId)}${options?.forceInstall ? '&force_install=true' : ''}`),
  listIntegrations: (workspaceId: string) =>
    api.get<GitIntegration[]>(`/git/integrations${qs(workspaceId)}`),
  getIntegration: (workspaceId: string, integrationId: string) =>
    api.get<GitIntegrationDetail>(`/git/integrations/${integrationId}${qs(workspaceId)}`),
  createIntegration: (workspaceId: string, payload: CreateGitIntegrationRequest) =>
    api.post<GitIntegration>(`/git/integrations${qs(workspaceId)}`, payload),
  deleteIntegration: (workspaceId: string, integrationId: string) =>
    api.del<{ status: string }>(`/git/integrations/${integrationId}${qs(workspaceId)}`),
  syncRepositories: (workspaceId: string, integrationId: string) =>
    api.post<GitRepository[]>(`/git/integrations/${integrationId}/sync${qs(workspaceId)}`, {}),
  listAvailableRepos: (workspaceId: string, integrationId: string) =>
    api.get<GitAvailableRepo[]>(`/git/integrations/${integrationId}/available-repos${qs(workspaceId)}`),
  wireRepositories: (workspaceId: string, integrationId: string, payload: WireGitRepositoriesRequest) =>
    gitRawRequest<WireGitRepositoriesResponse | WireGitRepositoriesConflictResponse>(
      `/git/integrations/${integrationId}/repositories${qs(workspaceId)}`,
      { method: 'POST', body: JSON.stringify(payload) },
    ),
  unwireRepository: (workspaceId: string, integrationId: string, repoId: string) =>
    api.del<{ status: string }>(`/git/integrations/${integrationId}/repositories/${repoId}${qs(workspaceId)}`),
  listRepositories: (workspaceId: string, options?: { all?: boolean }) =>
    api.get<GitRepository[]>(`/git/repositories${qs(workspaceId)}${options?.all ? '&all=true' : ''}`),
  listRepositoryBranches: (workspaceId: string, repoId: string) =>
    api.get<GitBranch[]>(`/git/repositories/${repoId}/branches${qs(workspaceId)}`),
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
