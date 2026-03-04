import { api } from '../api';
import type { Invitation, InviteInfo } from '../types';

export const inviteService = {
  send: (data: { workspace_id: string; email: string; role: string }) =>
    api.post<Invitation>('/invitations', data),
  list: (workspaceId: string) =>
    api.get<Invitation[]>(`/invitations?workspace_id=${workspaceId}`),
  getInfo: (token: string) =>
    api.get<InviteInfo>(`/invitations/info?token=${token}`),
  accept: (token: string) =>
    api.post<{ message: string }>('/invitations/accept', { token }),
  resend: (id: string) =>
    api.post<{ message: string }>(`/invitations/${id}/resend`),
  revoke: (id: string) =>
    api.del<{ message: string }>(`/invitations/${id}`),
};
