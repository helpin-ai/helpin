import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { queryKeys } from '@/lib/queryKeys';
import { crmPlaybookService as service, readPlaybookResponse as read, PlaybookRequestError } from '@/lib/services/crmPlaybookService';
import type { CRMPlaybookAutomationCommand, CRMPlaybookAutomationAdoption, CRMPlaybookActionInspection, PublishCRMPlaybookConnectionRequest } from '@/lib/crmPlaybookTypes';

export function useCRMPlaybookAgentUsage(ws: string, id: string, enabled: boolean) {
  return useQuery({ queryKey: [...queryKeys.crm.playbooks(ws), 'agent-usage', id], queryFn: async () => read(await service.agentUsage(ws, id)), enabled: enabled && !!ws && !!id });
}
export function useCRMPlaybookAutomationActivity(ws: string, id: string, page: number) {
  return useQuery({ queryKey: [...queryKeys.crm.playbook(ws, id), 'automation-activity', page], queryFn: async () => read(await service.automationActivity(ws, id, page)), enabled: !!ws && !!id, refetchInterval: 30_000 });
}

export function useCRMPlaybookAutomation(ws: string, id: string) {
  return useQuery({ queryKey: [...queryKeys.crm.playbook(ws, id), 'automation'], queryFn: async () => read(await service.automation(ws, id)), enabled: !!ws && !!id, refetchInterval: 30_000 });
}
export function useCRMSignalAutomation(ws: string, id: string) {
  return useQuery({ queryKey: [...queryKeys.crm.situation(ws, id), 'automation'], queryFn: async () => {
    const response = await service.signalAutomation(ws, id);
    if (response.error) throw new PlaybookRequestError(response.error, response.status);
    return response.data;
  }, enabled: !!ws && !!id, refetchInterval: 15_000 });
}
export function useCRMPlaybookActionIntent(ws: string, id: string) {
  return useQuery({ queryKey: [...queryKeys.crm.situations(ws), 'action-intent', id], queryFn: async () => read(await service.actionIntent(ws, id)), enabled: !!ws && !!id });
}
type Write =
  | { kind: 'prepare'; id: string; body: { expected_revision: number; playbook_version_id: string } }
  | { kind: 'publish'; id: string; body: PublishCRMPlaybookConnectionRequest }
  | { kind: 'configure'; id: string; body: CRMPlaybookAutomationCommand }
  | { kind: 'adopt'; id: string; body: CRMPlaybookAutomationAdoption }
  | { kind: 'reconcile'; id: string }
  | { kind: 'inspect'; id: string; body: CRMPlaybookActionInspection };
export function useCRMPlaybookAutomationWrite(ws: string) {
  const client = useQueryClient();
  return useMutation({ mutationFn: async (req: Write) => {
    switch (req.kind) {
      case 'prepare': return read(await service.prepareSetup(ws, req.id, req.body));
      case 'publish': return read(await service.publishConnection(ws, req.id, req.body));
      case 'configure': return read(await service.configureAutomation(ws, req.id, req.body));
      case 'adopt': return read(await service.adoptAutomation(ws, req.id, req.body));
      case 'reconcile': return read(await service.reconcileAction(ws, req.id));
      case 'inspect': return read(await service.inspectAction(ws, req.id, req.body));
    }
  }, retry: false, onSettled: async () => { await Promise.all([
    client.invalidateQueries({ queryKey: queryKeys.crm.playbooks(ws) }),
    client.invalidateQueries({ queryKey: queryKeys.crm.situations(ws) }),
  ]); } });
}
