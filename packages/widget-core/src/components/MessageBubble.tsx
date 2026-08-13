import { FunctionComponent } from 'preact';
import { useEffect, useRef, useState } from 'preact/hooks';
import type { Message, Attachment, WidgetConfig } from '../types';
import { renderMarkdown } from '../utils/markdownRenderer';
import { FileTextIcon, ThumbsDownIcon, ThumbsUpIcon } from './icons';

interface MessageBubbleProps {
  message: Message;
  config?: WidgetConfig;
  isFirstInGroup?: boolean;
  onImageClick?: (src: string, alt: string) => void;
  onAnswerFeedback?: (messageId: string, helpful: boolean) => void;
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

function linkPreviewHost(preview: NonNullable<Message['linkPreviews']>[number]): string {
  try {
    return new URL(preview.url).hostname.replace(/^www\./, '') || preview.host;
  } catch {
    return preview.host.replace(/^www\./, '');
  }
}

function LinkPreviews({ previews, outgoing }: { previews: NonNullable<Message['linkPreviews']>; outgoing: boolean }) {
  if (previews.length === 0) return null;
  return (
    <div className="helpin-link-previews">
      {previews.map((preview, index) => (
        <a
          key={`${preview.url}-${index}`}
          className={`helpin-link-preview ${outgoing ? 'helpin-link-preview--outgoing' : ''}`}
          href={preview.url}
          target="_blank"
          rel="noopener noreferrer"
        >
          {preview.image_url ? (
            <img
              src={preview.image_url}
              alt={preview.title}
              className="helpin-link-preview-image"
              loading="lazy"
            />
          ) : null}
          <div className="helpin-link-preview-body">
            <div className="helpin-link-preview-host">{preview.site_name || linkPreviewHost(preview)}</div>
            <div className="helpin-link-preview-title">{preview.title}</div>
            {preview.description ? (
              <div className="helpin-link-preview-description">{preview.description}</div>
            ) : null}
          </div>
        </a>
      ))}
    </div>
  );
}

function MessageContent({ message }: { message: Message }) {
  const [quotedExpanded, setQuotedExpanded] = useState(false);
  const isEmail = message.viaChannel === 'email';
  const visibleContent = isEmail && message.emailVisibleText !== undefined
    ? message.emailVisibleText
    : message.content;
  const quotedContent = message.emailQuotedText?.trim() ?? '';
  const hasQuotedContent = isEmail
    && message.emailHasQuotedContent === true
    && quotedContent.length > 0;

  return (
    <div className={isEmail ? 'helpin-email-message-content' : undefined}>
      {visibleContent.trim().length > 0 && (
        <div
          className="helpin-message-content"
          dangerouslySetInnerHTML={{ __html: renderMarkdown(visibleContent) }}
        />
      )}

      {hasQuotedContent && (
        <div className="helpin-email-history">
          <button
            type="button"
            className="helpin-email-history-toggle"
            aria-expanded={quotedExpanded}
            onClick={() => setQuotedExpanded(previous => !previous)}
          >
            {quotedExpanded ? 'Hide previous messages' : 'Show previous messages'}
          </button>
          {quotedExpanded && (
            <div
              className="helpin-message-content helpin-email-quoted-content"
              dangerouslySetInnerHTML={{ __html: renderMarkdown(quotedContent) }}
            />
          )}
        </div>
      )}
    </div>
  );
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
        Based on {formatSourceCount(sources.length)}
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
  onAnswerFeedback,
}) => {
  const [answerFeedback, setAnswerFeedback] = useState<'helpful' | 'not-helpful' | null>(null);
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
    message.isStreaming && 'helpin-message--streaming',
    isCustomer && message.deliveryStatus === 'sending' && 'helpin-message--sending',
  ]
    .filter(Boolean)
    .join(' ');

  const displayContent = message.viaChannel === 'email' && message.emailVisibleText !== undefined
    ? message.emailVisibleText
    : message.content;
  const hasTextContent = displayContent.trim().length > 0;
  const hasFiles = message.attachments?.some(a => a.url && !isImageType(a.fileType)) ?? false;
  const hasImages = message.attachments?.some(a => a.url && isImageType(a.fileType)) ?? false;
  const hasSources = Boolean(message.sources && message.sources.length > 0);
  const hasLinkPreviews = Boolean(message.linkPreviews && message.linkPreviews.length > 0);
  const showBubble = hasTextContent || hasFiles || message.viaChannel === 'email' || hasSources || hasLinkPreviews || message.isStreaming;
  const showAnswerFeedback = isAI
    && message.id !== '__intro__'
    && hasTextContent
    && !message.isStreaming
    && Boolean(onAnswerFeedback);
  const hasMeta = hasSources || showAnswerFeedback;

  const submitAnswerFeedback = (helpful: boolean) => {
    setAnswerFeedback(helpful ? 'helpful' : 'not-helpful');
    onAnswerFeedback?.(message.id, helpful);
  };

  const agentName = message.senderName;
  const agentAvatar = message.senderAvatar;
  const isWorkspaceBrandAvatar = message.id === '__intro__' && Boolean(agentAvatar);
  // Flat Intercom-style pill is reserved for teammate_joined — the one and
  // only widget-visible routing event. Every other system_event_type either
  // falls through to a normal bubble (when sender context is present — e.g.
  // ai_escalated renders as a "Helpin AI" bubble) or is filtered upstream.
  const showSystemPill = isSystem && message.systemEventType === 'teammate_joined';
  const displayName = isCustomer ? '' : (isAI ? 'Helpin AI' : (agentName || config?.workspaceName || 'Support Agent'));
  const tooltipText = formatRelativeTime(message.createdAt);

  if (showSystemPill) {
    return (
      <div
        className="helpin-message-row helpin-message-row--system"
        role="listitem"
        aria-label={message.content}
      >
        <div className="helpin-system-message" data-tooltip={tooltipText}>
          {agentAvatar ? (
            <img src={agentAvatar} alt={agentName || ''} className="helpin-system-message-avatar" />
          ) : agentName ? (
            <span className="helpin-system-message-avatar helpin-system-message-avatar--placeholder">
              {agentName.charAt(0).toUpperCase()}
            </span>
          ) : null}
          <span className="helpin-system-message-text">{message.content}</span>
        </div>
      </div>
    );
  }

  return (
    <div
      className={`helpin-message-row ${isCustomer ? 'helpin-message-row--customer' : 'helpin-message-row--agent'} ${isFirstInGroup ? '' : 'helpin-message-row--consecutive'} ${message.clientId ? 'helpin-message-row--optimistic' : ''}`}
      role="listitem"
      aria-label={`${displayName || 'You'} message`}
    >
      {!isCustomer ? (
        <>
          {/* Agent: avatar + name row, shown only on first message */}
          {isFirstInGroup && (
            <div className="helpin-message-agent-header">
              {isWorkspaceBrandAvatar ? (
                <span
                  className="helpin-message-avatar helpin-message-avatar--brand"
                  style={{ backgroundColor: config?.branding?.primaryColor || '#6366f1' }}
                >
                  <img src={agentAvatar} alt={displayName} className="helpin-message-brand-logo" />
                </span>
              ) : agentAvatar ? (
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
                <MessageContent message={message} />
                {message.isStreaming && <span className="helpin-streaming-cursor" aria-hidden="true" />}

                {message.viaChannel === 'email' && (
                  <div className="helpin-message-channel">Via email</div>
                )}

                {hasMeta && (
                  <div className="helpin-message-meta">
                    {hasSources && (
                      <SourcePopover sources={message.sources!} />
                    )}
                    {showAnswerFeedback && (
                      <div className="helpin-answer-feedback" aria-live="polite">
                        {answerFeedback ? (
                          <span className="helpin-answer-feedback-thanks">Thanks for the feedback</span>
                        ) : (
                          <>
                            <span className="helpin-answer-feedback-label">Helpful?</span>
                            <button
                              type="button"
                              className="helpin-answer-feedback-btn"
                              aria-label="This answer was helpful"
                              onClick={() => submitAnswerFeedback(true)}
                            >
                              <ThumbsUpIcon size={14} />
                            </button>
                            <button
                              type="button"
                              className="helpin-answer-feedback-btn"
                              aria-label="This answer was not helpful"
                              onClick={() => submitAnswerFeedback(false)}
                            >
                              <ThumbsDownIcon size={14} />
                            </button>
                          </>
                        )}
                      </div>
                    )}
                  </div>
                )}

                {hasLinkPreviews && message.linkPreviews && (
                  <LinkPreviews previews={message.linkPreviews} outgoing={false} />
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
              <MessageContent message={message} />
              {message.viaChannel === 'email' && (
                <div className="helpin-message-channel">Via email</div>
              )}
              {hasLinkPreviews && message.linkPreviews && (
                <LinkPreviews previews={message.linkPreviews} outgoing />
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
