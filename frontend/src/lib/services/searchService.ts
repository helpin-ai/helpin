import { api } from '../api';

export interface SearchResult {
  id: string;
  name: string;
  type: string;
  display_id?: number;
  task_key?: string;
  team_id?: string;
  team_name?: string;
}

export interface SearchResponse {
  tasks: SearchResult[];
  epics: SearchResult[];
  sprints: SearchResult[];
  objectives: SearchResult[];
  members: SearchResult[];
  documents: SearchResult[];
}

const qs = (workspaceId: string) => `workspace_id=${encodeURIComponent(workspaceId)}`;

export const searchService = {
  search: (workspaceId: string, query: string, options?: RequestInit) =>
    api.get<SearchResponse>(`/search/?${qs(workspaceId)}&q=${encodeURIComponent(query)}`, options),
};
