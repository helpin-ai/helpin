import { api } from '@/lib/api'
import type { Workspace } from '@/lib/types'

export const workspacesService = {
  list: () => api.get<Workspace[]>('/workspaces'),
  getBySlug: (slug: string) => api.get<Workspace>(`/workspaces/by-slug/${slug}`),
}
