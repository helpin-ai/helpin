import { api } from '../api';
import type { RoadmapData } from '../pmTypes';

const qs = (workspaceId: string) => `?workspace_id=${encodeURIComponent(workspaceId)}`;

const filterQuery = (filters: Record<string, string | boolean | undefined>) => {
  const search = new URLSearchParams();
  Object.entries(filters).forEach(([key, value]) => {
    if (value === undefined || value === '') return;
    search.set(key, String(value));
  });
  const str = search.toString();
  return str ? `&${str}` : '';
};

export const pmRoadmapService = {
  getData: (
    workspaceId: string,
    filters?: {
      team_id?: string;
      objective_id?: string;
      health?: string;
      show_completed?: boolean;
    },
  ) =>
    api.get<RoadmapData>(
      `/pm/roadmap${qs(workspaceId)}${filterQuery(filters ?? {})}`,
    ),
};
