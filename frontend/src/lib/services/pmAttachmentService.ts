import { api } from '../api';
import type { AttachmentResponse, CreateAttachmentRequest } from '../pmTypes';

const qs = (workspaceId: string) => `workspace_id=${encodeURIComponent(workspaceId)}`;

export const pmAttachmentService = {
  /** Initiate an upload — returns the attachment record + presigned PUT URL. */
  initiateUpload: (workspaceId: string, payload: CreateAttachmentRequest) =>
    api.post<AttachmentResponse>(`/pm/attachments?${qs(workspaceId)}`, payload),

  /** Confirm that the S3 upload completed successfully. */
  confirmUpload: (workspaceId: string, id: string) =>
    api.patch(`/pm/attachments/${id}/confirm?${qs(workspaceId)}`),

  /** List uploaded attachments for an entity (returns presigned GET URLs). */
  list: (workspaceId: string, entityType: string, entityId: string) =>
    api.get<AttachmentResponse[]>(
      `/pm/attachments?${qs(workspaceId)}&entity_type=${encodeURIComponent(entityType)}&entity_id=${encodeURIComponent(entityId)}`,
    ),

  /** Delete an attachment. */
  remove: (workspaceId: string, id: string, options?: { pendingOnly?: boolean }) => {
    const pendingOnly = options?.pendingOnly ? '&pending_only=true' : '';
    return api.del(`/pm/attachments/${id}?${qs(workspaceId)}${pendingOnly}`);
  },
};
