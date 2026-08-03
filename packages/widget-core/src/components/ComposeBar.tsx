import { FunctionComponent } from 'preact';
import { useState, useRef, useEffect } from 'preact/hooks';
import { PaperclipIcon, SendIcon, XIcon } from './icons';
import { EmojiPicker } from './EmojiPicker';
import { HelpinMark } from './HelpinMark';
import type { PendingAttachment } from '../types';

const HELPIN_BRANDING_URL = 'https://helpin.ai/?utm_source=helpin_widget&utm_medium=widget&utm_campaign=powered_by';

interface ComposeBarProps {
  onSend: (content: string, attachmentIds?: string[]) => void;
  onTyping?: (content: string) => void;
  onFilesSelected?: (files: File[]) => void;
  disabled?: boolean;
  placeholder?: string;
  showBranding?: boolean;
  pendingAttachments?: PendingAttachment[];
  onRemoveAttachment?: (id: string) => void;
  fileUploadsEnabled?: boolean;
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
  onSend,
  onTyping,
  onFilesSelected,
  disabled = false,
  placeholder = 'Ask a question...',
  showBranding = true,
  pendingAttachments = [],
  onRemoveAttachment,
  fileUploadsEnabled = true,
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

  const canSend = (message.trim().length > 0 || uploadedAttachmentIds.length > 0) && !disabled;

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
      <form
        className={`helpin-compose-bar ${isDragOver ? 'helpin-compose-bar--dragover' : ''}`}
        onSubmit={handleSubmit}
        onDragOver={handleDragOver}
        onDragLeave={handleDragLeave}
        onDrop={handleDrop}
      >
        {pendingAttachments.length > 0 && (
          <div className="helpin-compose-attachments">
            {pendingAttachments.map(att => (
              <div key={att.id} className={`helpin-compose-attachment-item ${att.status === 'error' ? 'helpin-compose-attachment-item--error' : ''}`}>
                {att.previewUrl && isImageType(att.fileType) ? (
                  <img src={att.previewUrl} alt={att.fileName} />
                ) : (
                  <div className="helpin-compose-attachment-file">
                    <span className="helpin-compose-attachment-filename">{att.fileName}</span>
                    <span className="helpin-compose-attachment-filesize">{formatFileSize(att.fileSize)}</span>
                  </div>
                )}
                {att.status === 'uploading' && (
                  <div className="helpin-compose-attachment-progress">
                    {att.progress}%
                  </div>
                )}
                {onRemoveAttachment && (
                  <button
                    type="button"
                    className="helpin-compose-attachment-remove"
                    onClick={() => onRemoveAttachment(att.id)}
                    aria-label={`Remove ${att.fileName}`}
                  >
                    <XIcon size={10} />
                  </button>
                )}
              </div>
            ))}
          </div>
        )}
        <textarea
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
            accept="image/*,application/pdf,.doc,.docx,.txt,.csv,.xls,.xlsx,.zip,.gz,.tar,.md"
            style={{ display: 'none' }}
            onChange={handleFileInputChange}
          />
        )}
      </form>
      {showBranding && (
        <div className="helpin-compose-footer">
          We run on{' '}
          <a href={HELPIN_BRANDING_URL} target="_blank" rel="noopener noreferrer" className="helpin-compose-footer-link">
            <HelpinMark className="helpin-compose-footer-icon" />
            <span>Helpin</span>
          </a>
        </div>
      )}
    </div>
  );
};
