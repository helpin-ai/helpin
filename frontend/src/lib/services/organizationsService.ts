import { api } from '../api';
import type { Organization, OrganizationWithRole, MemberWithUser } from '../types';

export const organizationsService = {
  list: () => api.get<OrganizationWithRole[]>('/organizations'),
  create: (data: { name: string; slug: string; logo_url?: string }) =>
    api.post<OrganizationWithRole>('/organizations', data),
  get: (id: string) => api.get<Organization>(`/organizations/${id}`),
  update: (id: string, data: { name?: string; logo_url?: string }) =>
    api.put<Organization>(`/organizations/${id}`, data),
  delete: (id: string) => api.del(`/organizations/${id}`),
  listMembers: (id: string) => api.get<MemberWithUser[]>(`/organizations/${id}/members`),
  addMember: (id: string, data: { user_id: string; role: string }) =>
    api.post(`/organizations/${id}/members`, data),
  updateMember: (id: string, userId: string, data: { role: string }) =>
    api.put(`/organizations/${id}/members/${userId}`, data),
  removeMember: (id: string, userId: string) =>
    api.del(`/organizations/${id}/members/${userId}`),
};
