import { useCallback, useEffect, useRef, useState } from 'react';
import { ChevronLeft, ChevronRight, Download, Loader2, Paperclip, Trash2, Upload, X } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { pmAttachmentService } from '@/lib/services/pmAttachmentService';
import { uploadToS3 } from '@/lib/api';
import type { AttachmentResponse } from '@/lib/pmTypes';
import { QuickTooltip } from '@/components/ui/quick-tooltip';

// File type icons from Plane.so
import pdfIcon from '@/assets/attachment/pdf-icon.png';
import csvIcon from '@/assets/attachment/csv-icon.png';
import excelIcon from '@/assets/attachment/excel-icon.png';
import docIcon from '@/assets/attachment/doc-icon.png';
import pngIcon from '@/assets/attachment/png-icon.png';
import jpgIcon from '@/assets/attachment/jpg-icon.png';
import svgIcon from '@/assets/attachment/svg-icon.png';
import txtIcon from '@/assets/attachment/txt-icon.png';
import zipIcon from '@/assets/attachment/zip-icon.png';
import rarIcon from '@/assets/attachment/rar-icon.png';
import htmlIcon from '@/assets/attachment/html-icon.png';
import cssIcon from '@/assets/attachment/css-icon.png';
import jsIcon from '@/assets/attachment/js-icon.png';
import audioIcon from '@/assets/attachment/audio-icon.png';
import videoIcon from '@/assets/attachment/video-icon.png';
import defaultIcon from '@/assets/attachment/default-icon.png';

interface AttachmentsProps {
  workspaceId: string;
  entityType: 'story' | 'epic' | 'comment';
  entityId: string;
  memberNameMap?: Map<string, string>;
}

const MAX_SIZE = 10 * 1024 * 1024; // 10 MB

function formatFileSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

function getFileExtension(filename: string): string {
  const parts = filename.split('.');
  return parts.length > 1 ? parts.pop()!.toLowerCase() : '';
}

function getFileName(filename: string): string {
  const parts = filename.split('.');
  if (parts.length > 1) parts.pop();
  return parts.join('.');
}

function getFileTypeIcon(extension: string): string {
  const iconMap: Record<string, string> = {
    pdf: pdfIcon,
    csv: csvIcon,
    xlsx: excelIcon,
    xls: excelIcon,
    doc: docIcon,
    docx: docIcon,
    png: pngIcon,
    jpg: jpgIcon,
    jpeg: jpgIcon,
    svg: svgIcon,
    txt: txtIcon,
    md: txtIcon,
    zip: zipIcon,
    gz: zipIcon,
    tar: zipIcon,
    rar: rarIcon,
    html: htmlIcon,
    css: cssIcon,
    js: jsIcon,
    ts: jsIcon,
    mp3: audioIcon,
    wav: audioIcon,
    mp4: videoIcon,
    mkv: videoIcon,
    wmv: videoIcon,
    webm: videoIcon,
  };
  return iconMap[extension] || defaultIcon;
}

function isImageType(contentType: string): boolean {
  return contentType.startsWith('image/') && !contentType.includes('svg');
}

export function Attachments({ workspaceId, entityType, entityId, memberNameMap }: AttachmentsProps) {
  const [attachments, setAttachments] = useState<AttachmentResponse[]>([]);
  const [uploading, setUploading] = useState(false);
  const [uploadProgress, setUploadProgress] = useState(0);
  const [error, setError] = useState<string | null>(null);
  const [dragging, setDragging] = useState(false);
  const [previewEntry, setPreviewEntry] = useState<AttachmentResponse | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

  // Load attachments
  const reload = useCallback(async () => {
    const { data } = await pmAttachmentService.list(workspaceId, entityType, entityId);
    setAttachments(data ?? []);
  }, [workspaceId, entityType, entityId]);

  useEffect(() => { reload(); }, [reload]);

  // Re-fetch when another client changes attachments
  useEffect(() => {
    const handler = (e: Event) => {
      const d = (e as CustomEvent)?.detail;
      if (d?.parent_id === entityId && d?.entity === 'attachment') reload();
    };
    window.addEventListener('story-child-updated', handler);
    return () => window.removeEventListener('story-child-updated', handler);
  }, [entityId, reload]);

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

        const uploadResult = await uploadToS3(initData.url, file, setUploadProgress, { 'x-amz-acl': 'public-read' });
        if (!uploadResult.ok) {
          setError(uploadResult.error);
          setUploading(false);
          continue;
        }

        await pmAttachmentService.confirmUpload(workspaceId, initData.attachment.id);

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

  const resolveUrl = (a: AttachmentResponse) => a.public_url || a.url;
  const imageAttachments = attachments.filter(({ attachment }) => isImageType(attachment.content_type));
  const fileAttachments = attachments.filter(({ attachment }) => !isImageType(attachment.content_type));

  return (
    <div className="space-y-3">
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

      {/* Image thumbnails grid */}
      {imageAttachments.length > 0 && (
        <div className="grid grid-cols-3 gap-2">
          {imageAttachments.map((entry) => (
            <div key={entry.attachment.id} className="group relative">
              <button
                type="button"
                className="block w-full overflow-hidden rounded-md border border-border/60 cursor-pointer"
                onClick={() => setPreviewEntry(entry)}
              >
                <img
                  src={resolveUrl(entry)}
                  alt={entry.attachment.file_name}
                  className="aspect-[4/3] w-full object-cover transition-transform group-hover:scale-105"
                  loading="lazy"
                />
              </button>
              <div className="absolute top-1 right-1 flex gap-0.5 opacity-0 group-hover:opacity-100 transition-opacity">
                <QuickTooltip label="Download">
                  <Button
                    variant="secondary"
                    size="icon"
                    className="h-5 w-5 bg-background/80 backdrop-blur-sm"
                    onClick={() => window.open(resolveUrl(entry), '_blank')}
                  >
                    <Download className="h-2.5 w-2.5" />
                  </Button>
                </QuickTooltip>
                <QuickTooltip label="Delete">
                  <Button
                    variant="secondary"
                    size="icon"
                    className="h-5 w-5 bg-background/80 backdrop-blur-sm"
                    onClick={() => handleDelete(entry.attachment.id)}
                  >
                    <Trash2 className="h-2.5 w-2.5 text-destructive" />
                  </Button>
                </QuickTooltip>
              </div>
              <p className="mt-1 truncate text-[11px] text-muted-foreground" title={entry.attachment.file_name}>
                {entry.attachment.file_name}
              </p>
              {memberNameMap && (
                <p className="truncate text-[10px] text-muted-foreground/70">
                  {memberNameMap.get(entry.attachment.uploaded_by_id) ?? 'Unknown'}
                </p>
              )}
            </div>
          ))}
        </div>
      )}

      {/* Non-image file list */}
      {fileAttachments.length > 0 && (
        <div className="rounded-md border border-border/60">
          {fileAttachments.map((entry, idx) => {
            const ext = getFileExtension(entry.attachment.file_name);
            const name = getFileName(entry.attachment.file_name);
            return (
              <div key={entry.attachment.id}>
                {idx > 0 && <div className="border-t border-border/40" />}
                <div className="group flex h-11 items-center gap-3 px-3 hover:bg-accent/50 transition-colors">
                  <img
                    src={getFileTypeIcon(ext)}
                    alt={ext}
                    className="h-5 w-5 shrink-0"
                  />
                  <a
                    href={resolveUrl(entry)}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="min-w-0 flex-1 truncate text-xs font-medium text-foreground/90 hover:underline"
                    title={entry.attachment.file_name}
                  >
                    {name}
                    {ext && <span className="text-muted-foreground">.{ext}</span>}
                  </a>
                  {memberNameMap && (
                    <span className="shrink-0 truncate max-w-[80px] text-[11px] text-muted-foreground/70">
                      {memberNameMap.get(entry.attachment.uploaded_by_id) ?? 'Unknown'}
                    </span>
                  )}
                  <span className="shrink-0 text-[11px] text-muted-foreground">
                    {formatFileSize(entry.attachment.file_size)}
                  </span>
                  <QuickTooltip label="Download">
                    <Button
                      variant="ghost"
                      size="icon"
                      className="h-6 w-6 shrink-0 opacity-0 group-hover:opacity-100 transition-opacity"
                      onClick={(e) => {
                        e.preventDefault();
                        window.open(resolveUrl(entry), '_blank');
                      }}
                    >
                      <Download className="h-3 w-3 text-muted-foreground" />
                    </Button>
                  </QuickTooltip>
                  <QuickTooltip label="Delete">
                    <Button
                      variant="ghost"
                      size="icon"
                      className="h-6 w-6 shrink-0 opacity-0 group-hover:opacity-100 transition-opacity"
                      onClick={(e) => {
                        e.preventDefault();
                        handleDelete(entry.attachment.id);
                      }}
                    >
                      <Trash2 className="h-3 w-3 text-muted-foreground hover:text-destructive" />
                    </Button>
                  </QuickTooltip>
                </div>
              </div>
            );
          })}
        </div>
      )}

      {/* Image preview lightbox */}
      {previewEntry && (() => {
        const curIdx = imageAttachments.findIndex((e) => e.attachment.id === previewEntry.attachment.id);
        const hasPrev = curIdx > 0;
        const hasNext = curIdx < imageAttachments.length - 1;
        const goPrev = () => { if (hasPrev) setPreviewEntry(imageAttachments[curIdx - 1]); };
        const goNext = () => { if (hasNext) setPreviewEntry(imageAttachments[curIdx + 1]); };

        return (
          <div
            className="fixed inset-0 z-50 flex flex-col items-center justify-center bg-black/70 backdrop-blur-sm"
            onClick={() => setPreviewEntry(null)}
            onKeyDown={(e) => {
              if (e.key === 'ArrowLeft') goPrev();
              else if (e.key === 'ArrowRight') goNext();
              else if (e.key === 'Escape') setPreviewEntry(null);
            }}
            tabIndex={0}
          >
            <Button
              variant="ghost"
              size="icon"
              className="absolute top-4 right-4 h-8 w-8 text-white hover:bg-white/20"
              onClick={() => setPreviewEntry(null)}
            >
              <X className="h-5 w-5" />
            </Button>

            {hasPrev && (
              <Button
                variant="ghost"
                size="icon"
                className="absolute left-4 top-1/2 -translate-y-1/2 h-10 w-10 text-white hover:bg-white/20"
                onClick={(e) => { e.stopPropagation(); goPrev(); }}
              >
                <ChevronLeft className="h-6 w-6" />
              </Button>
            )}

            {hasNext && (
              <Button
                variant="ghost"
                size="icon"
                className="absolute right-4 top-1/2 -translate-y-1/2 h-10 w-10 text-white hover:bg-white/20"
                onClick={(e) => { e.stopPropagation(); goNext(); }}
              >
                <ChevronRight className="h-6 w-6" />
              </Button>
            )}

            <img
              src={resolveUrl(previewEntry)}
              alt={previewEntry.attachment.file_name}
              className="max-h-[60vh] max-w-[70vw] rounded-lg object-contain"
              onClick={(e) => e.stopPropagation()}
            />
            <div className="mt-3 flex items-center gap-2 text-white/80" onClick={(e) => e.stopPropagation()}>
              <span className="text-sm font-medium">{previewEntry.attachment.file_name}</span>
              <span className="text-xs text-white/50">{formatFileSize(previewEntry.attachment.file_size)}</span>
              {imageAttachments.length > 1 && (
                <span className="text-xs text-white/40">{curIdx + 1} / {imageAttachments.length}</span>
              )}
            </div>
          </div>
        );
      })()}
    </div>
  );
}
