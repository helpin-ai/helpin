import { FunctionComponent } from 'preact';
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
  const showBubble = hasTextContent || hasFiles || message.viaChannel === 'email' || (message.sources && message.sources.length > 0) || message.aiConfidence !== undefined;

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

                {message.sources && message.sources.length > 0 && (
                  <div className="helpin-message-sources">
                    {message.sources.map((source, idx) => (
                      <div key={idx} className="helpin-source-item">
                        {source.title}
                      </div>
                    ))}
                  </div>
                )}

                {message.aiConfidence !== undefined && (
                  <div className="helpin-message-confidence">
                    Confidence: {Math.round(message.aiConfidence * 100)}%
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
