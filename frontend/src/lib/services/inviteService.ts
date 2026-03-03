import { api } from '../api';

export const inviteService = {
  send: (data: { workspace_id: string; email: string; role: string }) =>
    api.post('/invites', data),
};
