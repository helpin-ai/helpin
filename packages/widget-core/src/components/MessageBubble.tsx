import { FunctionComponent } from 'preact';
import { useEffect, useRef, useState } from 'preact/hooks';
import type { Message, Attachment, WidgetConfig } from '../types';
import { renderMarkdown } from '../utils/markdownRenderer';
import { FileTextIcon } from './icons';

interface MessageBubbleProps {
  message: Message;
  config?: WidgetConfig;
  isFirstInGroup?: boolean;
  onImageClick?: (src: string, alt: string) => void;
}

function isImageType(type: string): boolean {
  return type.startsWith('image/') && type !== 'image/svg+xml';
}

function formatFileSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

function ImageAttachments({ attachments, onImageClick }: { attachments: Attachment[]; onImageClick?: (src: string, alt: string) => void }) {
  const images = attachments.filter(a => a.url && isImageType(a.fileType));
  if (images.length === 0) return null;
  return (
    <div className="helpin-message-attachments">
      {images.map((att, i) => (
        <button
          key={att.id || i}
          type="button"
          className="helpin-attachment-image"
          onClick={() => onImageClick?.(att.url!, att.fileName)}
        >
          <img src={att.url} alt={att.fileName} loading="lazy" />
        </button>
      ))}
    </div>
  );
}

function FileAttachments({ attachments }: { attachments: Attachment[] }) {
  const files = attachments.filter(a => a.url && !isImageType(a.fileType));
  if (files.length === 0) return null;
  return (
    <div className="helpin-message-attachments">
      {files.map((att, i) => (
        <a
          key={att.id || i}
          className="helpin-attachment-file"
          href={att.url}
          target="_blank"
          rel="noopener noreferrer"
        >
          <FileTextIcon size={16} />
          <span className="helpin-attachment-filename">{att.fileName}</span>
          <span className="helpin-attachment-size">{formatFileSize(att.fileSize)}</span>
        </a>
      ))}
    </div>
  );
}

function formatRelativeTime(dateStr: string): string {
  const now = Date.now();
  const then = new Date(dateStr).getTime();
  const diffMs = now - then;
  const diffMin = Math.floor(diffMs / 60000);

  if (diffMin < 1) return 'Just now';
  if (diffMin < 60) return `${diffMin}m ago`;
  const diffHours = Math.floor(diffMin / 60);
  if (diffHours < 24) return `${diffHours}h ago`;
  const d = new Date(dateStr);
  return d.toLocaleString(undefined, { month: 'short', day: 'numeric', hour: 'numeric', minute: '2-digit' });
}

function formatSourceCount(count: number): string {
  return `${count} source${count === 1 ? '' : 's'}`;
}

function SourcePopover({ sources }: { sources: NonNullable<Message['sources']> }) {
  const [open, setOpen] = useState(false);
  const popoverRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    if (!open) return;

    const handlePointerDown = (event: MouseEvent | TouchEvent) => {
      if (!popoverRef.current?.contains(event.target as Node)) {
        setOpen(false);
      }
    };
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        setOpen(false);
      }
    };

    document.addEventListener('mousedown', handlePointerDown);
    document.addEventListener('touchstart', handlePointerDown);
    document.addEventListener('keydown', handleKeyDown);

    return () => {
      document.removeEventListener('mousedown', handlePointerDown);
      document.removeEventListener('touchstart', handlePointerDown);
      document.removeEventListener('keydown', handleKeyDown);
    };
  }, [open]);

  return (
    <div
      ref={popoverRef}
      className={`helpin-sources-popover ${open ? 'is-open' : ''}`}
      onMouseEnter={() => setOpen(true)}
      onMouseLeave={() => setOpen(false)}
    >
      <button
        type="button"
        className="helpin-message-sources-trigger"
        aria-haspopup="dialog"
        aria-expanded={open}
        onClick={() => setOpen(prev => !prev)}
      >
        {formatSourceCount(sources.length)}
      </button>

      {open && (
        <div className="helpin-sources-popover-card" role="dialog" aria-label="Sources">
          <div className="helpin-sources-popover-title">Sources</div>
          <div className="helpin-sources-popover-list">
            {sources.map((source, idx) => (
              <div key={idx} className="helpin-source-item">
                {source.title}
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}

export const MessageBubble: FunctionComponent<MessageBubbleProps> = ({
  message,
  config,
  isFirstInGroup = true,
  onImageClick,
}) => {
  const isCustomer = message.role === 'customer';
  const isAI = message.role === 'ai';
  const isAgent = message.role === 'agent';
  const isSystem = message.role === 'system';

  const bubbleClass = [
    'helpin-message-bubble',
    isCustomer && 'helpin-message--customer',
    isAgent && 'helpin-message--agent',
    isAI && 'helpin-message--ai',
    isSystem && 'helpin-message--system',
    message.isInternal && 'helpin-message--internal',
  ]
    .filter(Boolean)
    .join(' ');

  const hasTextContent = message.content.trim().length > 0;
  const hasFiles = message.attachments?.some(a => a.url && !isImageType(a.fileType)) ?? false;
  const hasImages = message.attachments?.some(a => a.url && isImageType(a.fileType)) ?? false;
  const hasSources = Boolean(message.sources && message.sources.length > 0);
  const showConfidence = hasSources && message.aiConfidence !== undefined;
  const showBubble = hasTextContent || hasFiles || message.viaChannel === 'email' || hasSources;
  const hasMeta = hasSources || showConfidence;

  const agentName = message.senderName;
  const agentAvatar = message.senderAvatar;
  const displayName = isCustomer ? '' : (isAI ? 'Helpin AI' : (agentName || config?.workspaceName || 'Support Agent'));
  const tooltipText = formatRelativeTime(message.createdAt);

  return (
    <div
      className={`helpin-message-row ${isCustomer ? 'helpin-message-row--customer' : 'helpin-message-row--agent'} ${isFirstInGroup ? '' : 'helpin-message-row--consecutive'}`}
      role="listitem"
      aria-label={`${displayName || 'You'} message`}
    >
      {!isCustomer ? (
        <>
          {/* Agent: avatar + name row, shown only on first message */}
          {isFirstInGroup && (
            <div className="helpin-message-agent-header">
              {agentAvatar ? (
                <img src={agentAvatar} alt={displayName} className="helpin-message-avatar" />
              ) : displayName ? (
                <span className="helpin-message-avatar-placeholder">
                  {displayName.charAt(0).toUpperCase()}
                </span>
              ) : null}
              {displayName && (
                <span className="helpin-message-agent-name">{displayName}</span>
              )}
            </div>
          )}

          {/* Bubble — indented to align with name, timestamp on hover */}
          <div className="helpin-message-agent-bubble-wrap">
            {showBubble && (
              <div className={bubbleClass} data-tooltip={tooltipText}>
                <div
                  className="helpin-message-content"
                  dangerouslySetInnerHTML={{ __html: renderMarkdown(message.content) }}
                />

                {message.viaChannel === 'email' && (
                  <div className="helpin-message-channel">Via email</div>
                )}

                {hasMeta && (
                  <div className="helpin-message-meta">
                    {hasSources && (
                      <SourcePopover sources={message.sources!} />
                    )}
                    {showConfidence && (
                      <div className="helpin-message-confidence">
                        Confidence: {Math.round(message.aiConfidence! * 100)}%
                      </div>
                    )}
                  </div>
                )}

                {message.attachments && message.attachments.length > 0 && (
                  <FileAttachments attachments={message.attachments} />
                )}
              </div>
            )}
            {hasImages && message.attachments && (
              <ImageAttachments attachments={message.attachments} onImageClick={onImageClick} />
            )}
          </div>
        </>
      ) : (
        /* Customer bubble — right-aligned, no avatar */
        <>
          {showBubble && (
            <div className={bubbleClass} data-tooltip={tooltipText}>
              <div
                className="helpin-message-content"
                dangerouslySetInnerHTML={{ __html: renderMarkdown(message.content) }}
              />
              {message.viaChannel === 'email' && (
                <div className="helpin-message-channel">Via email</div>
              )}
              {message.attachments && message.attachments.length > 0 && (
                <FileAttachments attachments={message.attachments} />
              )}
            </div>
          )}
          {hasImages && message.attachments && (
            <ImageAttachments attachments={message.attachments} onImageClick={onImageClick} />
          )}
        </>
      )}
    </div>
  );
};
