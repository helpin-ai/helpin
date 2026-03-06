import { api } from '../api';
import type { SupportTicket, SupportMessage, CreateTicketRequest, CreateMessageRequest, LinkStoryRequest, AssignTicketAgentRequest, PaginatedResponse, AgentRun, TicketStatus } from '../pmTypes';

const qs = (workspaceId: string) => `?workspace_id=${encodeURIComponent(workspaceId)}`;

export const supportService = {
  listTickets: (workspaceId: string, filters?: { status?: string; priority?: string }) => {
    let path = `/support/tickets${qs(workspaceId)}`;
    if (filters?.status) path += `&status=${filters.status}`;
    if (filters?.priority) path += `&priority=${filters.priority}`;
    return api.get<PaginatedResponse<SupportTicket[]>>(path);
  },
  getTicket: (workspaceId: string, id: string) =>
    api.get<SupportTicket>(`/support/tickets/${id}${qs(workspaceId)}`),
  createTicket: (workspaceId: string, payload: CreateTicketRequest) =>
    api.post<SupportTicket>(`/support/tickets${qs(workspaceId)}`, payload),
  listMessages: (workspaceId: string, ticketId: string) =>
    api.get<SupportMessage[]>(`/support/tickets/${ticketId}/messages${qs(workspaceId)}`),
  createMessage: (workspaceId: string, ticketId: string, payload: CreateMessageRequest) =>
    api.post<SupportMessage>(`/support/tickets/${ticketId}/messages${qs(workspaceId)}`, payload),
  updateStatus: (workspaceId: string, ticketId: string, status: TicketStatus) =>
    api.put<SupportTicket>(`/support/tickets/${ticketId}/status${qs(workspaceId)}`, { status }),
  linkStory: (workspaceId: string, ticketId: string, payload: LinkStoryRequest) =>
    api.post(`/support/tickets/${ticketId}/link-story${qs(workspaceId)}`, payload),
  assignAgent: (workspaceId: string, ticketId: string, payload: AssignTicketAgentRequest) =>
    api.post(`/support/tickets/${ticketId}/assign-agent${qs(workspaceId)}`, payload),
  runAgent: (workspaceId: string, ticketId: string) =>
    api.post<AgentRun>(`/support/tickets/${ticketId}/run-agent${qs(workspaceId)}`, {}),
};
