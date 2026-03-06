import { api } from '../api';
import type { GitIntegration, StoryGitLink, CreateGitIntegrationRequest, CreateBranchRequest } from '../pmTypes';

const qs = (workspaceId: string) => `?workspace_id=${encodeURIComponent(workspaceId)}`;

export const gitService = {
  listIntegrations: (workspaceId: string) =>
    api.get<GitIntegration[]>(`/git/integrations${qs(workspaceId)}`),
  createIntegration: (workspaceId: string, payload: CreateGitIntegrationRequest) =>
    api.post<GitIntegration>(`/git/integrations${qs(workspaceId)}`, payload),
  getStoryGitLinks: (workspaceId: string, storyId: string) =>
    api.get<StoryGitLink[]>(`/pm/stories/${storyId}/git-links${qs(workspaceId)}`),
  createBranch: (workspaceId: string, storyId: string, payload: CreateBranchRequest) =>
    api.post<StoryGitLink>(`/pm/stories/${storyId}/create-branch${qs(workspaceId)}`, payload),
};
