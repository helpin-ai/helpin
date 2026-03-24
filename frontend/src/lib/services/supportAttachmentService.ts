import { api } from '../api';

const qs = (workspaceId: string) => `?workspace_id=${encodeURIComponent(workspaceId)}`;

export interface SupportAttachmentInitResponse {
  attachment: {
    id: string;
    workspace_id: string;
    conversation_id: string;
    file_name: string;
    file_size: number;
    content_type: string;
    storage_key: string;
    public_url: string;
    is_uploaded: boolean;
    uploaded_by_type: string;
    created_at: string;
  };
  upload_url: string;
  public_url: string;
}

export const supportAttachmentService = {
  initiateUpload: (workspaceId: string, conversationId: string, payload: {
    file_name: string;
    file_size: number;
    content_type: string;
  }) =>
    api.post<SupportAttachmentInitResponse>(
      `/support/inbox/conversations/${conversationId}/attachments${qs(workspaceId)}`,
      payload,
    ),

  confirmUpload: (workspaceId: string, attachmentId: string) =>
    api.patch<{ message: string }>(
      `/support/inbox/attachments/${attachmentId}/confirm${qs(workspaceId)}`,
    ),

  deleteAttachment: (workspaceId: string, attachmentId: string) =>
    api.del<{ message: string }>(
      `/support/inbox/attachments/${attachmentId}${qs(workspaceId)}`,
    ),
};
