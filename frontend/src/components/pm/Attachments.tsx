import { useCallback, useEffect, useRef, useState } from 'react';
import { ArrowLeft01Icon, ArrowRight01Icon, Download04Icon, Loading01Icon, AttachmentIcon, Delete01Icon, Upload01Icon, Cancel01Icon, PlayCircleIcon } from '@/lib/icons';
import { Button } from '@/components/ui/button';
import { LoadingImage } from '@/components/ui/loading-image';
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
  entityType: 'task' | 'epic' | 'objective' | 'sprint' | 'comment';
  entityId: string;
  memberNameMap?: Map<string, string>;
  onDeleteAttachment?: (attachment: AttachmentResponse) => Promise<'handled' | 'prevent' | 'fallback'>;
  /** Expose a way for the parent to open the file picker */
  onFilePickerReady?: (openPicker: () => void) => void;
  /** Allow parent to programmatically upload files (e.g. from drag overlay) */
  onUploadReady?: (upload: (files: FileList | File[]) => Promise<void>) => void;
  editable?: boolean;
  showAddAction?: boolean;
  showEmptyState?: boolean;
}

const MAX_SIZE = 50 * 1024 * 1024; // 50 MB

export function getAttachmentGridDensityClasses(count: number): string {
  if (count >= 9) {
    return 'grid grid-cols-[repeat(auto-fill,minmax(8rem,1fr))] gap-1.5';
  }
  if (count >= 5) {
    return 'grid grid-cols-[repeat(auto-fill,minmax(9rem,1fr))] gap-2';
  }
  return 'grid grid-cols-[repeat(auto-fill,minmax(10rem,1fr))] gap-2';
}

function formatFileSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

function getFileExtension(filename: string): string {
  const parts = filename.split('.');
  return parts.length > 1 ? parts.pop()!.toLowerCase() : '';
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

function isVideoType(contentType: string, fileName: string): boolean {
  if (contentType.startsWith('video/')) return true;
  return ['mp4', 'mov', 'webm', 'mkv', 'wmv', 'avi', 'mpeg', 'mpg'].includes(getFileExtension(fileName));
}

export function Attachments({
  workspaceId,
  entityType,
  entityId,
  memberNameMap,
  onDeleteAttachment,
  onFilePickerReady,
  onUploadReady,
  editable = true,
  showAddAction = false,
  showEmptyState = false,
}: AttachmentsProps) {
  const [attachments, setAttachments] = useState<AttachmentResponse[]>([]);
  const [uploading, setUploading] = useState(false);
  const [uploadProgress, setUploadProgress] = useState(0);
  const [error, setError] = useState<string | null>(null);
  const [dragging, setDragging] = useState(false);
  const [previewEntry, setPreviewEntry] = useState<AttachmentResponse | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const dragCounterRef = useRef(0);

  // Load attachments
  const reload = useCallback(async () => {
    const { data } = await pmAttachmentService.list(workspaceId, entityType, entityId);
    setAttachments(data ?? []);
  }, [workspaceId, entityType, entityId]);

  useEffect(() => {
    let cancelled = false;
    void pmAttachmentService.list(workspaceId, entityType, entityId).then(({ data }) => {
      if (!cancelled) setAttachments(data ?? []);
    });
    return () => {
      cancelled = true;
    };
  }, [workspaceId, entityType, entityId]);

  // Re-fetch when another client changes attachments
  useEffect(() => {
    const handler = (e: Event) => {
      const d = (e as CustomEvent)?.detail;
      if (d?.parent_id === entityId && d?.entity === 'attachment') reload();
    };
    window.addEventListener('task-child-updated', handler);
    window.addEventListener('epic-child-updated', handler);
    return () => {
      window.removeEventListener('task-child-updated', handler);
      window.removeEventListener('epic-child-updated', handler);
    };
  }, [entityId, reload]);

  const handleUpload = useCallback(
    async (files: FileList | File[]) => {
      if (!editable) return;
      setError(null);
      const fileArray = Array.from(files);

      for (const file of fileArray) {
        if (file.size > MAX_SIZE) {
          setError(`${file.name} exceeds 50MB limit`);
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
    [workspaceId, entityType, entityId, editable],
  );

  // Expose file picker and upload to parent
  useEffect(() => {
    onFilePickerReady?.(() => {
      if (editable) fileInputRef.current?.click();
    });
  }, [onFilePickerReady, editable]);

  useEffect(() => {
    onUploadReady?.(handleUpload);
  }, [onUploadReady, handleUpload]);

  const handleDelete = async (entry: AttachmentResponse) => {
    if (!editable) return;
    if (onDeleteAttachment) {
      const action = await onDeleteAttachment(entry);
      if (action === 'handled') {
        setAttachments((prev) => prev.filter((item) => item.attachment.id !== entry.attachment.id));
        return;
      }
      if (action === 'prevent') {
        return;
      }
    }

    const { error: delError } = await pmAttachmentService.remove(workspaceId, entry.attachment.id);
    if (delError) {
      setError(delError);
      return;
    }
    setAttachments((prev) => prev.filter((a) => a.attachment.id !== entry.attachment.id));
  };

  const isFileDrag = (e: React.DragEvent) => e.dataTransfer.types.includes('Files');
  const onDragEnter = (e: React.DragEvent) => {
    if (!editable) return;
    if (!isFileDrag(e)) return;
    e.preventDefault();
    e.stopPropagation();
    dragCounterRef.current++;
    setDragging(true);
  };
  const onDragOver = (e: React.DragEvent) => {
    if (!editable) return;
    if (!isFileDrag(e)) return;
    e.preventDefault();
    e.stopPropagation();
  };
  const onDragLeave = (e: React.DragEvent) => {
    if (!editable) return;
    if (!isFileDrag(e)) return;
    e.preventDefault();
    e.stopPropagation();
    dragCounterRef.current = Math.max(0, dragCounterRef.current - 1);
    if (dragCounterRef.current === 0) setDragging(false);
  };
  const onDrop = (e: React.DragEvent) => {
    if (!editable) return;
    if (!isFileDrag(e)) return;
    e.preventDefault();
    e.stopPropagation();
    dragCounterRef.current = 0;
    setDragging(false);
    if (e.dataTransfer.files.length > 0) handleUpload(e.dataTransfer.files);
  };

  const resolveUrl = (a: AttachmentResponse) => a.public_url || a.url;
  const previewAttachments = attachments.filter(({ attachment }) => isImageType(attachment.content_type) || isVideoType(attachment.content_type, attachment.file_name));

  const hasAttachments = attachments.length > 0;

  return (
    <div
      className="space-y-3"
      onDragEnter={onDragEnter}
      onDragOver={onDragOver}
      onDragLeave={onDragLeave}
      onDrop={onDrop}
    >
      {(hasAttachments || showAddAction || showEmptyState) && (
        <div className="flex items-center justify-between gap-3">
          <div className="flex items-center gap-1.5">
            <AttachmentIcon className="h-3.5 w-3.5 text-muted-foreground" />
            <h3 className="text-xs font-semibold text-foreground/70 uppercase tracking-wide">
              Attachments
            </h3>
          </div>
          {showAddAction && editable ? (
            <Button
              type="button"
              variant="outline"
              size="xs"
              disabled={uploading}
              onClick={() => fileInputRef.current?.click()}
            >
              <AttachmentIcon className="h-3 w-3" />
              Attach files
            </Button>
          ) : null}
        </div>
      )}

      <input
        ref={fileInputRef}
        type="file"
        multiple
        disabled={!editable}
        className="hidden"
        onChange={(e) => {
          if (e.target.files?.length) handleUpload(e.target.files);
          e.target.value = '';
        }}
      />

      {/* Drop zone — only visible when dragging or uploading */}
      {editable && (dragging || uploading) && (
        <div
          onDragOver={onDragOver}
          onDragLeave={onDragLeave}
          onDrop={onDrop}
          className={`rounded-lg border-2 border-dashed px-4 py-3 text-center transition-colors
            ${dragging ? 'border-primary bg-primary/5' : 'border-border/60'}
          `}
        >
          {uploading ? (
            <div className="flex items-center justify-center gap-2">
              <Loading01Icon className="h-4 w-4 animate-spin text-muted-foreground" />
              <span className="text-xs text-muted-foreground">Uploading... {uploadProgress}%</span>
            </div>
          ) : (
            <div className="flex items-center justify-center gap-2">
              <Upload01Icon className="h-4 w-4 text-muted-foreground" />
              <span className="text-xs text-muted-foreground">Drop files here</span>
            </div>
          )}
        </div>
      )}

      {error && <p className="text-xs text-destructive">{error}</p>}

      {showEmptyState && !hasAttachments && !dragging && !uploading ? (
        <p className="text-sm text-muted-foreground">No attachments</p>
      ) : null}

      {/* Unified attachment grid — images + files as consistent cards */}
      {attachments.length > 0 && (
        <div className={getAttachmentGridDensityClasses(attachments.length)}>
          {attachments.map((entry) => {
            const isImage = isImageType(entry.attachment.content_type);
            const isVideo = isVideoType(entry.attachment.content_type, entry.attachment.file_name);
            const ext = getFileExtension(entry.attachment.file_name);
            const url = resolveUrl(entry);
            return (
              <div key={entry.attachment.id} className="group relative">
                <button
                  type="button"
                  className="block w-full overflow-hidden rounded-lg border border-border/60 cursor-pointer transition-colors hover:border-border"
                  onClick={() => (isImage || isVideo) ? setPreviewEntry(entry) : window.open(url, '_blank')}
                >
                  {isImage ? (
                    <LoadingImage
                      src={url}
                      alt={entry.attachment.file_name}
                      containerClassName="block h-20 w-full overflow-hidden"
                      className="h-20 w-full object-cover transition-transform group-hover:scale-105"
                      loading="lazy"
                    />
                  ) : isVideo ? (
                    <div className="relative h-20 w-full overflow-hidden bg-black">
                      <video
                        src={url}
                        preload="metadata"
                        muted
                        className="h-20 w-full object-cover opacity-80"
                      />
                      <div className="absolute inset-0 flex items-center justify-center bg-black/20">
                        <PlayCircleIcon className="h-8 w-8 text-white drop-shadow" />
                      </div>
                    </div>
                  ) : (
                    <div className="flex h-20 flex-col items-center justify-center gap-1.5 bg-muted/30">
                      <img
                        src={getFileTypeIcon(ext)}
                        alt={ext}
                        className="h-8 w-8"
                      />
                      <span className="text-[9px] font-medium uppercase text-muted-foreground tracking-wide">
                        {ext || 'FILE'}
                      </span>
                    </div>
                  )}
                </button>
                <div className="absolute top-1.5 right-1.5 flex gap-0.5 opacity-0 group-hover:opacity-100 transition-opacity">
                  <QuickTooltip label="Download">
                    <Button
                      variant="secondary"
                      size="icon"
                      className="h-6 w-6 bg-background/80 backdrop-blur-sm"
                      onClick={(e) => { e.stopPropagation(); window.open(url, '_blank'); }}
                    >
                      <Download04Icon className="h-3 w-3" />
                    </Button>
                  </QuickTooltip>
                  {editable && (
                    <QuickTooltip label="Delete">
                      <Button
                        variant="secondary"
                        size="icon"
                        className="h-6 w-6 bg-background/80 backdrop-blur-sm"
                        onClick={(e) => { e.stopPropagation(); void handleDelete(entry); }}
                      >
                        <Delete01Icon className="h-3 w-3 text-destructive" />
                      </Button>
                    </QuickTooltip>
                  )}
                </div>
                <p className="mt-1 truncate text-[10px] text-muted-foreground" title={entry.attachment.file_name}>
                  {entry.attachment.file_name}
                </p>
                {memberNameMap && (
                  <p className="truncate text-[9px] text-muted-foreground/70">
                    {memberNameMap.get(entry.attachment.uploaded_by_id) ?? 'Unknown'}
                  </p>
                )}
              </div>
            );
          })}
        </div>
      )}

      {/* Image/video preview lightbox */}
      {previewEntry && (() => {
        const curIdx = previewAttachments.findIndex((e) => e.attachment.id === previewEntry.attachment.id);
        const hasPrev = curIdx > 0;
        const hasNext = curIdx < previewAttachments.length - 1;
        const goPrev = () => { if (hasPrev) setPreviewEntry(previewAttachments[curIdx - 1]); };
        const goNext = () => { if (hasNext) setPreviewEntry(previewAttachments[curIdx + 1]); };
        const previewUrl = resolveUrl(previewEntry);
        const previewIsVideo = isVideoType(previewEntry.attachment.content_type, previewEntry.attachment.file_name);

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
              <Cancel01Icon className="h-5 w-5" />
            </Button>

            {hasPrev && (
              <Button
                variant="ghost"
                size="icon"
                className="absolute left-4 top-1/2 -translate-y-1/2 h-10 w-10 text-white hover:bg-white/20"
                onClick={(e) => { e.stopPropagation(); goPrev(); }}
              >
                <ArrowLeft01Icon className="h-6 w-6" />
              </Button>
            )}

            {hasNext && (
              <Button
                variant="ghost"
                size="icon"
                className="absolute right-4 top-1/2 -translate-y-1/2 h-10 w-10 text-white hover:bg-white/20"
                onClick={(e) => { e.stopPropagation(); goNext(); }}
              >
                <ArrowRight01Icon className="h-6 w-6" />
              </Button>
            )}

            {previewIsVideo ? (
              <video
                src={previewUrl}
                controls
                autoPlay
                className="max-h-[70vh] max-w-[75vw] rounded-lg bg-black"
                onClick={(e) => e.stopPropagation()}
              >
                <a href={previewUrl} target="_blank" rel="noopener noreferrer">{previewEntry.attachment.file_name}</a>
              </video>
            ) : (
              <LoadingImage
                src={previewUrl}
                alt={previewEntry.attachment.file_name}
                containerClassName="max-h-[50vh] max-w-[60vw] overflow-hidden rounded-lg"
                className="max-h-[50vh] max-w-[60vw] rounded-lg object-contain"
                onClick={(e) => e.stopPropagation()}
              />
            )}
            <div className="mt-3 flex items-center gap-2 text-white/80" onClick={(e) => e.stopPropagation()}>
              <span className="text-sm font-medium">{previewEntry.attachment.file_name}</span>
              <span className="text-xs text-white/50">{formatFileSize(previewEntry.attachment.file_size)}</span>
              {previewAttachments.length > 1 && (
                <span className="text-xs text-white/40">{curIdx + 1} / {previewAttachments.length}</span>
              )}
            </div>
          </div>
        );
      })()}
    </div>
  );
}
