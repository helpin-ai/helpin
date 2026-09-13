import { useQuery } from '@tanstack/react-query';
import { workspacesService } from '@/lib/services/workspacesService';
import { queryKeys } from '@/lib/queryKeys';
import { unwrapRequired } from '@/lib/queryUtils';

export function useCRMOwnerMembers(workspaceId: string) {
  const query = useQuery({
    queryKey: [...queryKeys.workspaces.moduleAccess(workspaceId), 'owners', 'crm'],
    enabled: !!workspaceId,
    queryFn: async () => unwrapRequired(await workspacesService.listAssignableMembers(workspaceId, 'crm'), 'CRM owners'),
  });
  return { members: query.data ?? [], loading: query.isPending, error: query.isError };
}
