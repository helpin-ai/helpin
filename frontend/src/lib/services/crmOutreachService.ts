import { api } from "@/lib/api";
import type {
  EmailTemplate,
  EmailSequence,
  SequenceEnrollment,
  SequenceDelivery,
  EnrollmentRequest,
  EnrollmentPreview,
} from "@/lib/crmOutreachTypes";
const path = (ws: string, resource: string) =>
  `/crm/outreach/${resource}?workspace_id=${encodeURIComponent(ws)}`;
export const crmOutreachService = {
  renderTemplate: (
    ws: string,
    id: string,
    data: { email: string; account_id: string; deal_id?: string },
  ) => api.post<EmailTemplate>(path(ws, `templates/${id}/render`), data),
  templates: (ws: string) => api.get<EmailTemplate[]>(path(ws, "templates")),
  saveTemplate: (ws: string, data: Partial<EmailTemplate>) =>
    data.id
      ? api.put<EmailTemplate>(path(ws, `templates/${data.id}`), data)
      : api.post<EmailTemplate>(path(ws, "templates"), data),
  deleteTemplate: (ws: string, id: string) =>
    api.del(path(ws, `templates/${id}`)),
  sequences: (ws: string) => api.get<EmailSequence[]>(path(ws, "sequences")),
  saveSequence: (ws: string, data: Partial<EmailSequence>) =>
    data.id
      ? api.put<EmailSequence>(path(ws, `sequences/${data.id}`), data)
      : api.post<EmailSequence>(path(ws, "sequences"), data),
  preview: (ws: string, id: string, data: EnrollmentRequest) =>
    api.post<EnrollmentPreview[]>(
      `${path(ws, `sequences/${id}/enroll`)}&preview=true`,
      data,
    ),
  enroll: (ws: string, id: string, data: EnrollmentRequest) =>
    api.post<SequenceEnrollment[]>(path(ws, `sequences/${id}/enroll`), data),
  enrollments: (
    ws: string,
    filters: {
      sequence_id?: string;
      contact_id?: string;
      deal_id?: string;
      search?: string;
      status?: string;
      page?: string;
    } = {},
    signal?: AbortSignal,
  ) =>
    api.get<SequenceEnrollment[]>(
      `${path(ws, "enrollments")}&${new URLSearchParams(Object.entries(filters).filter(([, v]) => Boolean(v)) as [string, string][]).toString()}`,
      {
        signal: signal
          ? AbortSignal.any([signal, AbortSignal.timeout(15000)])
          : AbortSignal.timeout(15000),
      },
    ),
  detail: (ws: string, id: string) =>
    api.get<{ enrollment: SequenceEnrollment; deliveries: SequenceDelivery[] }>(
      path(ws, `enrollments/${id}`),
    ),
  control: (
    ws: string,
    id: string,
    action: string,
    review?: { subject: string; body_html: string },
  ) => api.post(path(ws, `enrollments/${id}`), { action, ...review }),
};
