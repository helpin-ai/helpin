import { SUPPORT_FILE_ACCEPT } from '../hooks/useAttachmentUploads';
import { FunctionComponent, type ComponentChildren } from 'preact';
import { useState, useRef, useEffect } from 'preact/hooks';
import { PaperclipIcon, SendIcon, XIcon } from './icons';
import { EmojiPicker } from './EmojiPicker';
import { BrandAttribution } from './BrandAttribution';
import type { PendingAttachment } from '../types';

interface ComposeBarProps {
  notice?: ComponentChildren;
  onSend: (content: string, attachmentIds?: string[]) => void;
  onTyping?: (content: string) => void;
  onFilesSelected?: (files: File[]) => void;
  disabled?: boolean;
  placeholder?: string;
  showBranding?: boolean;
  pendingAttachments?: PendingAttachment[];
  onRemoveAttachment?: (id: string) => void;
  onRetryAttachment?: (id: string) => void;
  attachmentError?: string;
  fileUploadsEnabled?: boolean;
  workspaceId?: string;
  workspaceName?: string;
}

function isImageType(type: string): boolean {
  return type.startsWith('image/') && type !== 'image/svg+xml';
}

function formatFileSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

export const ComposeBar: FunctionComponent<ComposeBarProps> = ({
  notice,
  onSend,
  onTyping,
  onFilesSelected,
  disabled = false,
  placeholder = 'Ask a question...',
  showBranding = true,
  pendingAttachments = [],
  onRemoveAttachment,
  onRetryAttachment,
  attachmentError,
  fileUploadsEnabled = true,
  workspaceId,
  workspaceName,
}) => {
  const [message, setMessage] = useState('');
  const [isDragOver, setIsDragOver] = useState(false);
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (textareaRef.current) {
      textareaRef.current.style.height = 'auto';
      textareaRef.current.style.height = `${Math.min(textareaRef.current.scrollHeight, 120)}px`;
    }
  }, [message]);

  const uploadedAttachmentIds = pendingAttachments
    .filter(a => a.status === 'uploaded' && a.attachmentId)
    .map(a => a.attachmentId!);

  const canSend = (message.trim().length > 0 || uploadedAttachmentIds.length > 0) && !disabled && !pendingAttachments.some(a => a.status !== 'uploaded');

  const handleSubmit = (e?: Event) => {
    e?.preventDefault();
    if (!canSend) return;
    onSend(message.trim(), uploadedAttachmentIds.length > 0 ? uploadedAttachmentIds : undefined);
    setMessage('');
  };

  const handleKeyDown = (e: KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSubmit();
    }
  };

  const handleEmojiSelect = (emoji: string) => {
    const textarea = textareaRef.current;
    if (textarea) {
      const start = textarea.selectionStart;
      const end = textarea.selectionEnd;
      const newValue = message.slice(0, start) + emoji + message.slice(end);
      setMessage(newValue);
      setTimeout(() => {
        textarea.selectionStart = textarea.selectionEnd = start + emoji.length;
        textarea.focus();
      }, 0);
    } else {
      setMessage(message + emoji);
    }
  };

  const handleFileSelect = () => {
    if (disabled) return;
    fileInputRef.current?.click();
  };

  const handleFileInputChange = (e: Event) => {
    const input = e.target as HTMLInputElement;
    if (input.files && input.files.length > 0) {
      onFilesSelected?.(Array.from(input.files));
      input.value = '';
    }
  };

  const handlePaste = (e: ClipboardEvent) => {
    const items = e.clipboardData?.items;
    if (disabled || !items || !onFilesSelected || !fileUploadsEnabled) return;
    const files: File[] = [];
    for (let i = 0; i < items.length; i++) {
      if (items[i].kind === 'file') {
        const file = items[i].getAsFile();
        if (file) files.push(file);
      }
    }
    if (files.length > 0) {
      e.preventDefault();
      onFilesSelected(files);
    }
  };

  const handleDragOver = (e: DragEvent) => {
    if (disabled || !fileUploadsEnabled) return;
    e.preventDefault();
    setIsDragOver(true);
  };

  const handleDragLeave = (e: DragEvent) => {
    e.preventDefault();
    setIsDragOver(false);
  };

  const handleDrop = (e: DragEvent) => {
    e.preventDefault();
    setIsDragOver(false);
    if (disabled || !onFilesSelected || !fileUploadsEnabled) return;
    const files = e.dataTransfer?.files;
    if (files && files.length > 0) {
      onFilesSelected(Array.from(files));
    }
  };

  return (
    <div className="helpin-compose-wrapper">
      {notice}
      <form
        className={`helpin-compose-bar ${isDragOver ? 'helpin-compose-bar--dragover' : ''}`}
        onSubmit={handleSubmit}
        onDragOver={handleDragOver}
        onDragLeave={handleDragLeave}
        onDrop={handleDrop}
      >
        {attachmentError && <div className="helpin-compose-upload-error" role="alert">{attachmentError}</div>}
        {pendingAttachments.length > 0 && (
          <div className="helpin-compose-upload-list">
            {pendingAttachments.map(att => (
              <div key={att.id} className="helpin-compose-upload-row">
                {att.previewUrl && isImageType(att.fileType) && <img src={att.previewUrl} alt="" />}
                <div className="helpin-compose-upload-details">
                  <span className="helpin-compose-upload-name" title={att.fileName}>{att.fileName}</span>
                  <span className="helpin-compose-upload-status">
                    {formatFileSize(att.fileSize)} · {att.status === 'uploaded' ? 'Ready to send' : att.status === 'error' ? 'Upload failed' : att.progress >= 99 ? 'Finishing upload…' : `Uploading ${att.progress}%`}
                  </span>
                  {att.status === 'uploading' && <progress max={100} value={att.progress} aria-label={`Uploading ${att.fileName}`} />}
                  {att.status === 'error' && <span className="helpin-compose-upload-error" role="alert">{att.error || 'Upload failed. Please try again.'}</span>}
                </div>
                {att.status === 'error' && onRetryAttachment && <button type="button" className="helpin-compose-upload-retry" onClick={() => onRetryAttachment(att.id)}>Retry</button>}
                {onRemoveAttachment && <button type="button" className="helpin-compose-upload-remove" onClick={() => onRemoveAttachment(att.id)} aria-label={`${att.status === 'uploading' ? 'Cancel upload of' : 'Remove'} ${att.fileName}`}><XIcon size={14} /></button>}
              </div>
            ))}
          </div>
        )}
        <textarea dir="auto"
          ref={textareaRef}
          className="helpin-compose-input"
          value={message}
          onInput={(e) => { const val = (e.target as HTMLTextAreaElement).value; setMessage(val); onTyping?.(val); }}
          onKeyDown={handleKeyDown}
          onPaste={handlePaste}
          placeholder={placeholder}
          disabled={disabled}
          rows={1}
          aria-label={placeholder}
        />
        <div className="helpin-compose-actions">
          <div className="helpin-compose-tools">
            {fileUploadsEnabled && (
              <button
                type="button"
                className="helpin-compose-tool-btn"
                aria-label="Attach file"
                tabIndex={0}
                onClick={handleFileSelect}
                disabled={disabled}
              >
                <PaperclipIcon size={18} />
              </button>
            )}
            <EmojiPicker onEmojiSelect={handleEmojiSelect} disabled={disabled} />
          </div>
          <button
            type="submit"
            className={`helpin-compose-send ${canSend ? 'helpin-compose-send--active' : ''}`}
            disabled={!canSend}
            aria-label="Send message"
          >
            <SendIcon size={16} />
          </button>
        </div>
        {fileUploadsEnabled && (
          <input
            ref={fileInputRef}
            type="file"
            multiple
            accept={SUPPORT_FILE_ACCEPT}
            style={{ display: 'none' }}
            onChange={handleFileInputChange}
          />
        )}
      </form>
      {showBranding && (
        <BrandAttribution
          label="We run on"
          className="helpin-compose-footer"
          workspaceId={workspaceId}
          workspaceName={workspaceName}
          content="chat_widget_composer"
        />
      )}
    </div>
  );
};
