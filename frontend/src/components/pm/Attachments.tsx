import { useCallback, useEffect, useRef, useState } from 'react';
import { File as FileIcon, Loader2, Paperclip, Trash2, Upload } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { pmAttachmentService } from '@/lib/services/pmAttachmentService';
import { uploadToS3 } from '@/lib/api';
import type { AttachmentResponse } from '@/lib/pmTypes';

interface AttachmentsProps {
  workspaceId: string;
  entityType: 'story' | 'epic' | 'comment';
  entityId: string;
}

const MAX_SIZE = 10 * 1024 * 1024; // 10 MB

function formatFileSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

function fileIcon(contentType: string) {
  if (contentType.startsWith('image/')) return '🖼️';
  if (contentType === 'application/pdf') return '📄';
  if (contentType.includes('spreadsheet') || contentType.includes('excel')) return '📊';
  if (contentType.includes('word') || contentType.includes('document')) return '📝';
  if (contentType.includes('zip') || contentType.includes('tar') || contentType.includes('gzip')) return '📦';
  return '📎';
}

export function Attachments({ workspaceId, entityType, entityId }: AttachmentsProps) {
  const [attachments, setAttachments] = useState<AttachmentResponse[]>([]);
  const [uploading, setUploading] = useState(false);
  const [uploadProgress, setUploadProgress] = useState(0);
  const [error, setError] = useState<string | null>(null);
  const [dragging, setDragging] = useState(false);
  const fileInputRef = useRef<HTMLInputElement>(null);

  // Load attachments
  useEffect(() => {
    (async () => {
      const { data } = await pmAttachmentService.list(workspaceId, entityType, entityId);
      setAttachments(data ?? []);
    })();
  }, [workspaceId, entityType, entityId]);

  const handleUpload = useCallback(
    async (files: FileList | File[]) => {
      setError(null);
      const fileArray = Array.from(files);

      for (const file of fileArray) {
        if (file.size > MAX_SIZE) {
          setError(`${file.name} exceeds 10MB limit`);
          continue;
        }

        setUploading(true);
        setUploadProgress(0);

        // 1. Initiate upload — get presigned URL
        const { data: initData, error: initError } = await pmAttachmentService.initiateUpload(workspaceId, {
          entity_type: entityType,
          entity_id: entityId,
          file_name: file.name,
          file_size: file.size,
          content_type: file.type || 'application/octet-stream',
        });

        if (initError || !initData) {
          setError(initError ?? 'Failed to initiate upload');
          setUploading(false);
          continue;
        }

        // 2. Upload directly to S3
        const uploadResult = await uploadToS3(initData.url, file, setUploadProgress);
        if (!uploadResult.ok) {
          setError(uploadResult.error);
          setUploading(false);
          continue;
        }

        // 3. Confirm upload
        await pmAttachmentService.confirmUpload(workspaceId, initData.attachment.id);

        // 4. Refresh list to get download URLs
        const { data: refreshed } = await pmAttachmentService.list(workspaceId, entityType, entityId);
        setAttachments(refreshed ?? []);
        setUploading(false);
      }
    },
    [workspaceId, entityType, entityId],
  );

  const handleDelete = async (id: string) => {
    const { error: delError } = await pmAttachmentService.remove(workspaceId, id);
    if (delError) {
      setError(delError);
      return;
    }
    setAttachments((prev) => prev.filter((a) => a.attachment.id !== id));
  };

  // Drag-and-drop handlers
  const onDragOver = (e: React.DragEvent) => {
    e.preventDefault();
    setDragging(true);
  };
  const onDragLeave = () => setDragging(false);
  const onDrop = (e: React.DragEvent) => {
    e.preventDefault();
    setDragging(false);
    if (e.dataTransfer.files.length > 0) handleUpload(e.dataTransfer.files);
  };

  return (
    <div className="space-y-2">
      <div className="flex items-center gap-1.5">
        <Paperclip className="h-3.5 w-3.5 text-muted-foreground" />
        <h3 className="text-xs font-semibold text-muted-foreground uppercase tracking-wide">Attachments</h3>
      </div>

      {/* Drop zone */}
      <div
        onDragOver={onDragOver}
        onDragLeave={onDragLeave}
        onDrop={onDrop}
        className={`rounded-lg border-2 border-dashed px-4 py-3 text-center transition-colors cursor-pointer
          ${dragging ? 'border-primary bg-primary/5' : 'border-border/60 hover:border-border'}
        `}
        onClick={() => fileInputRef.current?.click()}
      >
        <input
          ref={fileInputRef}
          type="file"
          multiple
          className="hidden"
          onChange={(e) => {
            if (e.target.files?.length) handleUpload(e.target.files);
            e.target.value = '';
          }}
        />
        {uploading ? (
          <div className="flex items-center justify-center gap-2">
            <Loader2 className="h-4 w-4 animate-spin text-muted-foreground" />
            <span className="text-xs text-muted-foreground">Uploading... {uploadProgress}%</span>
          </div>
        ) : (
          <div className="flex items-center justify-center gap-2">
            <Upload className="h-4 w-4 text-muted-foreground" />
            <span className="text-xs text-muted-foreground">Drop files or click to upload (max 10MB)</span>
          </div>
        )}
      </div>

      {error && <p className="text-xs text-destructive">{error}</p>}

      {/* File list */}
      {attachments.length > 0 && (
        <div className="space-y-1">
          {attachments.map(({ attachment, url }) => (
            <div
              key={attachment.id}
              className="group flex items-center gap-2 rounded-md px-2 py-1.5 text-sm hover:bg-accent/50 transition-colors"
            >
              <span className="text-sm shrink-0">{fileIcon(attachment.content_type)}</span>
              <a
                href={url}
                target="_blank"
                rel="noopener noreferrer"
                className="min-w-0 flex-1 truncate text-xs text-foreground hover:underline"
                title={attachment.file_name}
              >
                {attachment.file_name}
              </a>
              <span className="shrink-0 text-[11px] text-muted-foreground">{formatFileSize(attachment.file_size)}</span>
              <Button
                variant="ghost"
                size="icon"
                className="h-6 w-6 shrink-0 opacity-0 group-hover:opacity-100 transition-opacity"
                onClick={(e) => {
                  e.stopPropagation();
                  handleDelete(attachment.id);
                }}
              >
                <Trash2 className="h-3 w-3 text-muted-foreground hover:text-destructive" />
              </Button>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
