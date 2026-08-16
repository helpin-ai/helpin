import { pmAttachmentService } from '@/lib/services/pmAttachmentService';
import { uploadToS3 } from '@/lib/api';

export interface EditorUploadConfig {
  workspaceId: string;
  entityType: 'task' | 'epic' | 'objective' | 'sprint' | 'editor_upload';
  entityId: string;
  private?: boolean;
}

export interface EditorImageUploadResult {
  attachmentId: string;
  publicUrl: string;
}

const MAX_SIZE = 50 * 1024 * 1024; // 50 MB

export async function uploadEditorFile(
  file: File,
  config: EditorUploadConfig,
): Promise<EditorImageUploadResult> {
  if (file.size > MAX_SIZE) {
    throw new Error('File exceeds maximum size of 50 MB');
  }

  // 1. Initiate upload → get presigned PUT URL + public URL
  const { data: initData, error: initError } = await pmAttachmentService.initiateUpload(
    config.workspaceId,
    {
      entity_type: config.entityType,
      entity_id: config.entityId,
      file_name: file.name || 'attachment',
      file_size: file.size,
      content_type: file.type || 'application/octet-stream',
      private: config.private,
    },
  );

  if (initError || !initData) {
    throw new Error(initError ?? 'Failed to initiate upload');
  }

  // 2. Upload directly to S3. Private uploads deliberately omit the ACL.
  const { ok, error: s3Error } = await uploadToS3(
    initData.url,
    file,
    undefined,
    initData.public_url ? { 'x-amz-acl': 'public-read' } : undefined,
  );
  if (!ok) {
    throw new Error(s3Error ?? 'Failed to upload to S3');
  }

  // 3. Confirm upload
  await pmAttachmentService.confirmUpload(config.workspaceId, initData.attachment.id);

  // 4. Return the stable app-controlled content URL. The backend resolves it
  // to a fresh object-store download URL when the image is requested.
  return {
    attachmentId: initData.attachment.id,
    publicUrl: pmAttachmentService.contentUrl(initData.attachment.id),
  };
}

/**
 * Upload an image file via the attachment infrastructure and return the public URL
 * together with the persistent attachment ID.
 */
export async function uploadEditorImage(
  file: File,
  config: EditorUploadConfig,
): Promise<EditorImageUploadResult> {
  if (!file.type.startsWith('image/')) {
    throw new Error('Only image files are supported');
  }
  return uploadEditorFile(file, config);
}
