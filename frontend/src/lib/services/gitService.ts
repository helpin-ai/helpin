import { api } from '../api';
import type {
  GitIntegration,
  GitHubInstallURLResponse,
  GitRepository,
  StoryDeliveryTarget,
  StoryGitLink,
  CreateGitIntegrationRequest,
  CreateBranchRequest,
  UpdateGitRepositoryRequest,
  UpdateStoryDeliveryTargetRequest,
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
  getStoryGitLinks: (workspaceId: string, storyId: string) =>
    api.get<StoryGitLink[]>(`/pm/stories/${storyId}/git-links${qs(workspaceId)}`),
  getStoryDeliveryTarget: (workspaceId: string, storyId: string) =>
    api.get<StoryDeliveryTarget>(`/pm/stories/${storyId}/delivery-target${qs(workspaceId)}`),
  updateStoryDeliveryTarget: (workspaceId: string, storyId: string, payload: UpdateStoryDeliveryTargetRequest) =>
    api.put<StoryDeliveryTarget>(`/pm/stories/${storyId}/delivery-target${qs(workspaceId)}`, payload),
  createBranch: (workspaceId: string, storyId: string, payload: CreateBranchRequest) =>
    api.post<StoryGitLink>(`/pm/stories/${storyId}/create-branch${qs(workspaceId)}`, payload),
};
