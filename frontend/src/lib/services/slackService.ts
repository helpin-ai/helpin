import { api } from '../api'
import { unwrap } from '../queryUtils'

export type SlackPurpose = 'support' | 'mixed' | 'pm'
export interface SlackRoute {
  channel_id: string
  channel_name: string
  crm_company_id?: string | null
  purpose: SlackPurpose
  mailbox_id: string | null
  team_id: string | null
  epic_id: string | null
}
export interface SlackSettings {
  configured: boolean
  connected?: boolean
  ai_enabled?: boolean
  workspace_name?: string
  routes: SlackRoute[] | null
}
export interface SlackChannel {
  id: string
  name: string
  is_private?: boolean
  is_pending_ext_shared?: boolean
  is_ext_shared: boolean
}

const path = (ws: string, suffix = '') =>
  `/slack/${suffix}?workspace_id=${encodeURIComponent(ws)}`
export const slackService = {
  settings: (ws: string) => api.get<SlackSettings>(path(ws)).then(unwrap),
  channels: (ws: string) =>
    api.get<SlackChannel[]>(path(ws, 'channels')).then(unwrap),
  start: (ws: string) =>
    api.post<{ url: string }>(path(ws, 'install'), {}).then(unwrap),
  complete: (ws: string, code: string, state: string) =>
    api.post(path(ws, 'complete'), { code, state }).then(unwrap),
  link: (ws: string, token: string) =>
    api.post(path(ws, 'link'), { token }).then(unwrap),
  save: (ws: string, route: SlackRoute) =>
    api.put(path(ws, 'routes'), route).then(unwrap),
  remove: (ws: string, id: string) =>
    api.del(path(ws, 'routes/' + encodeURIComponent(id))).then(unwrap),
  setAI: (ws: string, enabled: boolean) =>
    api.patch(path(ws), { enabled }).then(unwrap),
  disconnect: (ws: string) => api.del(path(ws)).then(unwrap),
}
