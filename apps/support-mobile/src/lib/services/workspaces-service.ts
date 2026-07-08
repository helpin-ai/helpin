import { api } from '@mobile/lib/api'
import type { Workspace } from '@mobile/lib/types'

export const workspacesService = {
  list: () => api.get<Workspace[]>('/workspaces'),
  getBySlug: (slug: string) => api.get<Workspace>(`/workspaces/by-slug/${slug}`),
}
