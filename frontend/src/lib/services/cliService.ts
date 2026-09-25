import { api } from '@/lib/api';
import type { CLIAuthorizationQuery, CLIConsentRequest } from '@/lib/cliTypes';

export const cliService = {
  consentRequest: (query: CLIAuthorizationQuery) =>
    api.get<CLIConsentRequest>(`/cli/oauth/request?${new URLSearchParams({ ...query })}`),
  authorize: (query: CLIAuthorizationQuery, workspaceId: string) =>
    api.post<{ redirect_url: string }>('/cli/oauth/authorize', { query, workspace_id: workspaceId }),
};
