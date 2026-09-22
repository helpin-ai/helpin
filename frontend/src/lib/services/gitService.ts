import { API_BASE, api } from '../api';
import type {
  GitIntegration,
  GitIntegrationDetail,
  GitHubInstallURLResponse,
  GitHubAppManifestResponse,
  GitHubAppStatus,
  GitLabConnectTokenRequest,
  GitLabConnectResponse,
  GitBranch,
  GitAvailableRepo,
  GitRepository,
  EpicDeliveryTarget,
  TaskDeliveryTarget,
  TaskGitLink,
  CreateGitIntegrationRequest,
  UpdateGitIntegrationRequest,
  CreateBranchRequest,
  UpdateGitRepositoryRequest,
  UpdateEpicDeliveryTargetRequest,
  UpdateTaskDeliveryTargetRequest,
  WireGitRepositoriesConflictResponse,
  WireGitRepositoriesRequest,
  WireGitRepositoriesResponse,
} from '../pmTypes';

const qs = (workspaceId: string) => `?workspace_id=${encodeURIComponent(workspaceId)}`;
const orgQs = (workspaceId?: string) => workspaceId ? `?workspace_id=${encodeURIComponent(workspaceId)}` : '';

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
  getOrgGitHubInstallURL: (organizationId: string, workspaceId?: string, options?: { forceInstall?: boolean }) =>
    api.get<GitHubInstallURLResponse>(
      `/organizations/${organizationId}/git/github/install-url${orgQs(workspaceId)}${options?.forceInstall ? `${workspaceId ? '&' : '?'}force_install=true` : ''}`,
    ),
  connectOrgGitLab: (organizationId: string, workspaceId: string | undefined, payload: GitLabConnectTokenRequest) =>
    api.post<GitLabConnectResponse>(`/organizations/${organizationId}/git/gitlab/connect${orgQs(workspaceId)}`, payload),
  listOrgIntegrations: (organizationId: string) =>
    api.get<GitIntegration[]>(`/organizations/${organizationId}/git/integrations`),
  getOrgIntegration: (organizationId: string, integrationId: string) =>
    api.get<GitIntegrationDetail>(`/organizations/${organizationId}/git/integrations/${integrationId}`),
  updateOrgIntegration: (organizationId: string, integrationId: string, payload: UpdateGitIntegrationRequest) =>
    api.put<GitIntegration>(`/organizations/${organizationId}/git/integrations/${integrationId}`, payload),
  createOrgIntegration: (organizationId: string, workspaceId: string | undefined, payload: CreateGitIntegrationRequest) =>
    api.post<GitIntegration>(`/organizations/${organizationId}/git/integrations${orgQs(workspaceId)}`, payload),
  deleteOrgIntegration: (organizationId: string, integrationId: string, workspaceId?: string) =>
    api.del<{ status: string }>(`/organizations/${organizationId}/git/integrations/${integrationId}${orgQs(workspaceId)}`),
  syncOrgRepositories: (organizationId: string, integrationId: string) =>
    api.post<GitRepository[]>(`/organizations/${organizationId}/git/integrations/${integrationId}/sync`, {}),
  getGitHubAppStatus: (workspaceId: string) =>
    api.get<GitHubAppStatus>(`/workspaces/${workspaceId}/github/app-status`),
  createGitHubAppManifest: (workspaceId: string, organization?: string) =>
    api.post<GitHubAppManifestResponse>(`/workspaces/${workspaceId}/github/app-manifest`, organization ? { organization } : {}),
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
  listAvailableRepos: (
    workspaceId: string,
    integrationId: string,
    options?: { search?: string; noCache?: boolean; signal?: AbortSignal },
  ) => {
    const params = new URLSearchParams({ workspace_id: workspaceId });
    if (options?.search) params.set('search', options.search);
    if (options?.noCache) params.set('nocache', '1');
    return api.get<GitAvailableRepo[]>(
      `/git/integrations/${integrationId}/available-repos?${params.toString()}`,
      { signal: options?.signal },
    );
  },
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
  useTaskEpicDeliveryTarget: (workspaceId: string, taskId: string) =>
    api.post<TaskDeliveryTarget>(`/pm/tasks/${taskId}/delivery-target/use-epic${qs(workspaceId)}`, {}),
  getEpicDeliveryTarget: (workspaceId: string, epicId: string) =>
    api.get<EpicDeliveryTarget>(`/pm/epics/${epicId}/delivery-target${qs(workspaceId)}`),
  updateEpicDeliveryTarget: (workspaceId: string, epicId: string, payload: UpdateEpicDeliveryTargetRequest) =>
    api.put<EpicDeliveryTarget>(`/pm/epics/${epicId}/delivery-target${qs(workspaceId)}`, payload),
  createBranch: (workspaceId: string, taskId: string, payload: CreateBranchRequest) =>
    api.post<TaskGitLink>(`/pm/tasks/${taskId}/create-branch${qs(workspaceId)}`, payload),
};
