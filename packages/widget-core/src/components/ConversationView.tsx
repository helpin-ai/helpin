import { useAttachmentUploads } from '../hooks/useAttachmentUploads';
import type { UploadAttachment } from '../hooks/useAttachmentUploads';
import { FunctionComponent } from 'preact';
import { useCallback, useEffect, useMemo, useRef, useState } from 'preact/hooks';
import type { ActiveTeammate, Conversation, Message, WidgetConfig } from '../types';
import { MessageList } from './MessageList';
import { getPrivacyPolicyURL } from '@helpin-ai/shared';
import { PrivacyNotice } from './PrivacyNotice';
import { ComposeBar } from './ComposeBar';
import { TypingIndicator } from './TypingIndicator';
import { PreChatForm } from './PreChatForm';
import { ImageLightbox } from './ImageLightbox';
import { SpecialNoticeBanner } from './SpecialNoticeBanner';
import { AIThinkingMark } from './AIThinkingMark';
import { CsatRating } from './CsatRating';
import { ChevronLeftIcon, MoreVerticalIcon, XIcon } from './icons';
import { resolveWelcomeMessage } from '../welcomeMessage';


interface ConversationViewProps {
  draftPrivacyDismissed?: boolean;
  onDismissDraftPrivacy?: () => void;
  config: WidgetConfig;
  conversation?: Conversation;
  messages: Message[];
  activeTeammate?: ActiveTeammate;
  onSendMessage: (content: string, attachmentIds?: string[]) => void;
  onTyping?: (content: string) => void;
  onUploadAttachment?: UploadAttachment;
  isTyping?: boolean;
  isAIThinking?: boolean;
  aiProgressLabel?: string;
  typingAgentName?: string;
  typingAgentAvatar?: string;
  onBack: () => void;
  onClose?: () => void;
  onEscalateToHuman?: () => void;
  isExpanded?: boolean;
  onToggleExpanded?: () => void;
  transcriptEmail?: string;
  onRequestTranscript?: (email?: string) => Promise<{ success: boolean; message: string }>;
  showHumanAvailability?: boolean;
  showPreChatForm?: boolean;
  contactCaptureCompleted?: boolean;
  onCaptureEmail?: (email: string) => Promise<void>;
  onPreChatSubmit?: (data: { phone: string; email: string }) => void;
  onImageClick?: (src: string, alt: string) => void;
  onAnswerFeedback?: (messageId: string, helpful: boolean) => Promise<boolean>;
  connectionStatus?: 'idle' | 'connecting' | 'connected' | 'disconnected' | 'failed';
  queuedMessageCount?: number;
  csatSubmitted?: boolean;
  onCsatSubmit?: (rating: number, feedback?: string) => void;
}

export const ConversationView: FunctionComponent<ConversationViewProps> = ({
  draftPrivacyDismissed = false,
  onDismissDraftPrivacy,
  config,
  conversation,
  messages,
  activeTeammate,
  onSendMessage,
  onTyping,
  onUploadAttachment,
  isTyping = false,
  isAIThinking = false,
  aiProgressLabel = 'Looking into this…',
  typingAgentName,
  typingAgentAvatar,
  onBack,
  onClose,
  onEscalateToHuman,
  isExpanded = false,
  onToggleExpanded,
  transcriptEmail,
  onRequestTranscript,
  showHumanAvailability = false,
  showPreChatForm = false,
  contactCaptureCompleted = false,
  onPreChatSubmit,
  onCaptureEmail,
  onImageClick: externalImageClick,
  onAnswerFeedback,
  connectionStatus = 'connected',
  queuedMessageCount = 0,
  csatSubmitted = false,
  onCsatSubmit,
}) => {
  const conversationKey = conversation?.id || '__new__';
  const policyUrl = config.privacyNotice?.enabled ? getPrivacyPolicyURL(config.privacyNotice.policyUrl) : undefined;
  const hasSentCustomerMessage = messages.some(message => message.role === 'customer' && !message.isInternal && !message.deliveryStatus);
  const showPrivacyNotice = Boolean(
    policyUrl && !hasSentCustomerMessage && !draftPrivacyDismissed
    && (!conversation?.id || messages.length > 0),
  );
  const [introCreatedAt] = useState(() => new Date().toISOString());
  const [preChatDone, setPreChatDone] = useState(false);
  const [showHumanContactForm, setShowHumanContactForm] = useState(false);
  const uploads = useAttachmentUploads(conversationKey, onUploadAttachment);
  const { pendingAttachments } = uploads;
  const [lightboxImage, setLightboxImage] = useState<{ src: string; alt: string } | null>(null);
  const [menuOpen, setMenuOpen] = useState(false);
  const [showTranscriptForm, setShowTranscriptForm] = useState(false);
  const [transcriptEmailInput, setTranscriptEmailInput] = useState(transcriptEmail || '');
  const [transcriptStatus, setTranscriptStatus] = useState<{ type: 'success' | 'error'; message: string } | null>(null);
  const [emailCaptureStatus, setEmailCaptureStatus] = useState<'idle' | 'saving' | 'saved'>('idle');
  const [emailCaptureError, setEmailCaptureError] = useState('');
  const [isSendingTranscript, setIsSendingTranscript] = useState(false);
  const [autoExpandDismissed, setAutoExpandDismissed] = useState(false);
  const menuRef = useRef<HTMLDivElement | null>(null);
  const threadRef = useRef<HTMLDivElement | null>(null);
  const autoExpandedConversationRef = useRef<string | null>(null);

  const workspaceName = config.workspaceName || 'Support';
  const logoUrl = config.branding?.logoUrl;
  const brandColor = config.branding?.primaryColor || '#6366f1';
  const availability = config.availability;
  const aiFirst = Boolean(config.features?.aiFirst);
  const welcomeMessage = resolveWelcomeMessage(config.branding?.welcomeMessage, aiFirst);
  const hasTeamReply = messages.some((message) => message.role !== 'customer');
  const hasAssistantReply = messages.some((message) => message.role === 'ai' || message.role === 'agent');
  const hasHumanReply = messages.some((message) => message.role === 'agent');
  const hasCustomerMessage = messages.some((message) => message.role === 'customer' && message.content.trim().length > 0);
  const escalationMessageCopy = (config.features?.escalationMessage || 'Let me connect you with a team member who can help further.').trim();
  const hasEscalationNotice = messages.some((message) =>
    message.role === 'system' &&
    (
      message.systemEventType === 'ai_escalated' ||
      message.content.trim() === escalationMessageCopy ||
      message.content.trim() === 'Let me connect you with a team member who can help further.'
    ),
  );
  const fileUploadsEnabled = Boolean(config.features?.fileUploads && onUploadAttachment);
  const composeDisabled = connectionStatus === 'connecting';
  const composePlaceholder = connectionStatus === 'failed'
    ? 'Write a message — we’ll send it when reconnected'
    : connectionStatus === 'disconnected'
      ? 'Write a message — we’ll send it when reconnected'
      : connectionStatus === 'connecting'
        ? 'Connecting to support...'
        : 'Ask a question...';
  const hasHumanHandoffAlready = Boolean(
    hasHumanReply ||
      hasEscalationNotice ||
      conversation?.aiState === 'escalated' ||
      conversation?.status === 'resolved' ||
      conversation?.status === 'closed' ||
      conversation?.flowState === 'waiting_for_human' ||
      conversation?.flowState === 'queued_for_human' ||
      conversation?.flowState === 'after_hours_queue' ||
      conversation?.flowState === 'assigned_to_human' ||
      conversation?.flowState === 'resolved_by_human',
  );
  const showHumanHandoffState = showHumanAvailability && !hasHumanReply;
  const handoffState = conversation?.handoffState;
  const nobodyAvailable = handoffState === 'busy' || handoffState === 'after_hours';
  const delayedReplyIndex = messages.reduce((latest, message, index) => message.delayedTeamReply ? index : latest, -1);
  const delayedEmailCapture = delayedReplyIndex >= 0
    && messages[delayedReplyIndex].captureEmail === true
    && !messages.slice(delayedReplyIndex + 1).some((message) => message.role === 'agent');
  const showEscalationEmailCapture = (nobodyAvailable || delayedEmailCapture)
    && Boolean(onCaptureEmail)
    && conversation?.status !== 'resolved'
    && conversation?.status !== 'closed'
    && !transcriptEmail
    && (delayedEmailCapture || !hasHumanReply)
    && (delayedEmailCapture || (!contactCaptureCompleted && !preChatDone))
    && !showHumanContactForm
    && emailCaptureStatus !== 'saved';
  const showTalkToHumanButton = Boolean(
    config.features?.showTalkToHuman &&
      onEscalateToHuman &&
      hasAssistantReply &&
      !hasHumanHandoffAlready &&
      !showHumanHandoffState &&
      !isAIThinking &&
      !isTyping,
  );
  const isFinished = ['resolved', 'closed', 'spam'].includes(conversation?.status || '')
    || ['resolved_by_human', 'resolved_by_ai'].includes(conversation?.flowState || '');
  // Explicit current flow takes priority over historical AI escalation state.
  const hasCurrentHumanHandoff = conversation?.flowState
    ? ['waiting_for_human', 'queued_for_human', 'after_hours_queue', 'assigned_to_human'].includes(conversation.flowState)
    : conversation?.aiState === 'escalated';
  const latestCustomerIndex = messages.reduce((latest, message, index) =>
    !message.isInternal && message.role === 'customer' ? index : latest, -1);
  const latestHumanIndex = messages.reduce((latest, message, index) =>
    !message.isInternal && message.role === 'agent' ? index : latest, -1);
  const handoffStartedAt = Date.parse(conversation?.handoffStartedAt || '');
  const humanRepliedSinceHandoff = latestHumanIndex >= 0
    && latestHumanIndex > latestCustomerIndex
    && (!Number.isFinite(handoffStartedAt) || Date.parse(messages[latestHumanIndex].createdAt) >= handoffStartedAt);
  const isWaitingForTeammate = Boolean(
    !isFinished && hasCurrentHumanHandoff && !humanRepliedSinceHandoff && !isTyping && !isAIThinking,
  );
  const waitingTeammates = (config.availableTeammates || []).slice(0, 3);
  const showCsat = Boolean(
    config.features?.csatRating
      && onCsatSubmit
      && conversation?.id
      && (conversation.status === 'resolved' || conversation.status === 'closed' || conversation.flowState === 'resolved_by_human')
      && hasCustomerMessage
      && hasAssistantReply
      && !csatSubmitted
      && !isTyping
      && !isAIThinking,
  );
  const handoffProgress = useMemo(() => {
    if (handoffState === 'after_hours' || conversation?.flowState === 'after_hours_queue') {
      return { title: 'Our team is currently offline', detail: availability?.outsideHoursMessage || '' };
    }
    return { title: 'Waiting for a teammate', detail: '' };
  }, [availability?.outsideHoursMessage, conversation?.flowState, handoffState]);

  // Derive the most recent responding agent from messages.
  const activeAgent = useMemo(() => {
    for (let i = messages.length - 1; i >= 0; i--) {
      const m = messages[i];
      if (m.role === 'ai') {
        return { name: 'Helpin AI', avatar: m.senderAvatar, isAI: true };
      }
      if (m.role === 'agent' && m.senderName) {
        return { name: m.senderName, avatar: m.senderAvatar, isAI: false };
      }
    }
    return null;
  }, [messages]);

  const introRole = aiFirst ? 'ai' as const : 'agent' as const;
  const introName = aiFirst ? 'Helpin AI' : workspaceName;
  const introAvatar = aiFirst ? undefined : (logoUrl || undefined);
  const showActiveAgentHeader = Boolean(activeAgent?.name) && !showHumanHandoffState;
  const teammateStatus = activeTeammate?.status || 'online';

  useEffect(() => {
    setTranscriptEmailInput(transcriptEmail || '');
  }, [transcriptEmail]);

  useEffect(() => {
    setEmailCaptureStatus('idle');
    setEmailCaptureError('');
    setAutoExpandDismissed(false);
    setShowHumanContactForm(false);
    autoExpandedConversationRef.current = null;
  }, [conversationKey]);

  const requestHumanSupport = () => {
    if (showPreChatForm && !preChatDone && onPreChatSubmit) {
      setShowHumanContactForm(true);
      return;
    }
    onEscalateToHuman?.();
  };

  const completeHumanSupportRequest = (data: { phone: string; email: string }) => {
    onPreChatSubmit?.(data);
    setPreChatDone(true);
    setShowHumanContactForm(false);
    onEscalateToHuman?.();
  };

  useEffect(() => {
    if (!menuOpen) {
      return;
    }
    const handlePointerDown = (event: MouseEvent) => {
      const path = typeof event.composedPath === 'function' ? event.composedPath() : [];
      const clickedInsideMenu = menuRef.current
        ? path.includes(menuRef.current) || menuRef.current.contains(event.target as Node)
        : false;
      if (!clickedInsideMenu) {
        setMenuOpen(false);
        setShowTranscriptForm(false);
      }
    };
    document.addEventListener('mousedown', handlePointerDown);
    return () => {
      document.removeEventListener('mousedown', handlePointerDown);
    };
  }, [menuOpen]);

  const introMessage: Message = {
    id: '__intro__',
    conversationId: '__intro__',
    role: introRole,
    content: welcomeMessage,
    senderName: introName,
    senderAvatar: introAvatar,
    isInternal: false,
    createdAt: introCreatedAt,
  };

  const displayMessages = hasTeamReply || messages.length === 0
    ? messages.length === 0
      ? [introMessage]
      : messages
    : [introMessage, ...messages];

  useEffect(() => {
    if (!onToggleExpanded || isExpanded || autoExpandDismissed || typeof window === 'undefined') {
      return;
    }
    if (window.matchMedia('(max-width: 767px)').matches) {
      return;
    }

    let frameId = 0;
    frameId = window.requestAnimationFrame(() => {
      const tableWraps = Array.from(
        threadRef.current?.querySelectorAll<HTMLElement>('.helpin-message-content .helpin-table-wrap') ?? [],
      );
      const hasWideTable = tableWraps.some((wrap) => {
        const table = wrap.querySelector<HTMLTableElement>('table');
        if (!table) {
          return false;
        }
        const rows = Array.from(table.rows);
        const columnCount = rows.reduce((max, row) => Math.max(max, row.cells.length), 0);
        return columnCount >= 3 && table.scrollWidth > wrap.clientWidth + 32;
      });

      if (hasWideTable) {
        autoExpandedConversationRef.current = conversationKey;
        onToggleExpanded();
      }
    });

    return () => {
      window.cancelAnimationFrame(frameId);
    };
  }, [autoExpandDismissed, conversationKey, displayMessages, isExpanded, onToggleExpanded]);

  const handleSendMessage = useCallback((content: string, attachmentIds?: string[]) => {
    if (pendingAttachments.some(a => a.status !== 'uploaded')) return;
    onSendMessage(content, attachmentIds);
    uploads.clear();
  }, [onSendMessage, pendingAttachments, uploads.clear]);

  const handleImageClick = useCallback((src: string, alt: string) => {
    if (externalImageClick) {
      externalImageClick(src, alt);
    } else {
      setLightboxImage({ src, alt });
    }
  }, [externalImageClick]);

  const handleTranscriptRequest = useCallback(async () => {
    if (!onRequestTranscript || isSendingTranscript) {
      return;
    }
    setIsSendingTranscript(true);
    setTranscriptStatus(null);
    try {
      const resp = await onRequestTranscript(transcriptEmailInput.trim() || undefined);
      setTranscriptStatus({ type: resp.success ? 'success' : 'error', message: resp.message });
      if (resp.success) {
        setShowTranscriptForm(false);
      }
    } catch (error) {
      setTranscriptStatus({
        type: 'error',
        message: error instanceof Error ? error.message : 'Unable to send transcript right now.',
      });
    } finally {
      setIsSendingTranscript(false);
    }
  }, [isSendingTranscript, onRequestTranscript, transcriptEmailInput]);

  return (
    <div className="helpin-conversation-view">
      <div className="helpin-conversation-header">
        <button
          type="button"
          className="helpin-conversation-back"
          onClick={onBack}
          aria-label="Back"
        >
          <ChevronLeftIcon size={20} />
        </button>

          <div className="helpin-conversation-brand">
            {showActiveAgentHeader && activeAgent?.name ? (
            <div
              className={`helpin-agent-avatar-wrap${activeAgent.isAI ? '' : ' helpin-avatar-tooltip'}`}
              data-tooltip={activeAgent.isAI ? undefined : activeAgent.name}
            >
              {activeAgent?.avatar ? (
                <img src={activeAgent.avatar} alt={activeAgent.name} className="helpin-conversation-logo helpin-agent-avatar" />
              ) : (
                <div className="helpin-conversation-logo-placeholder">
                  <span>{activeAgent.name.charAt(0).toUpperCase()}</span>
                </div>
              )}
              {!activeAgent.isAI && (
                <span
                  className={`helpin-presence-dot helpin-presence-dot--${teammateStatus}`}
                  aria-label={`${activeAgent.name} is ${teammateStatus}`}
                />
              )}
            </div>
          ) : logoUrl ? (
            <div
              className="helpin-conversation-logo helpin-conversation-logo--brand"
              style={{ backgroundColor: brandColor }}
            >
              <img
                src={logoUrl}
                alt={workspaceName}
                className="helpin-conversation-brand-logo"
              />
            </div>
          ) : (
            <div className="helpin-conversation-logo-placeholder">
              <span>{workspaceName.charAt(0).toUpperCase()}</span>
            </div>
          )}

          <div className="helpin-conversation-brand-copy">
            {showActiveAgentHeader && activeAgent?.name ? (
              <>
                <span className="helpin-conversation-title">{activeAgent.name}</span>
                <span className="helpin-conversation-subtitle">
                  {activeAgent.isAI ? 'Our AI assistant will reply first' : `from ${workspaceName}`}
                </span>
              </>
            ) : (
              <>
                <span className="helpin-conversation-title">
                  {showHumanHandoffState ? workspaceName : (aiFirst ? 'Helpin AI' : workspaceName)}
                </span>
                <span className="helpin-conversation-subtitle">
                  {showHumanHandoffState
                    ? availability?.replyTimeText || availability?.outsideHoursMessage || 'Our team will follow up as soon as someone is available.'
                    : aiFirst
                      ? 'Our AI assistant will reply first'
                      : availability?.statusText || 'Online now'}
                </span>
              </>
            )}
          </div>
        </div>

        <div className="helpin-conversation-header-actions" ref={menuRef}>
          <button
            type="button"
            className="helpin-conversation-header-btn"
            aria-label="More options"
            aria-expanded={menuOpen}
            onClick={() => {
              setMenuOpen((open) => !open);
              setTranscriptStatus(null);
            }}
          >
            <MoreVerticalIcon size={20} />
          </button>
          {menuOpen && (
            <div className="helpin-conversation-menu">
              {policyUrl && (
                <a className="helpin-conversation-menu-item" href={policyUrl} target="_blank" rel="noopener noreferrer" onClick={() => setMenuOpen(false)}>Privacy Policy</a>
              )}
              {onToggleExpanded && (
                <button
                  type="button"
                  className="helpin-conversation-menu-item"
                  onClick={() => {
                    if (isExpanded && autoExpandedConversationRef.current === conversationKey) {
                      setAutoExpandDismissed(true);
                      autoExpandedConversationRef.current = null;
                    }
                    onToggleExpanded();
                    setMenuOpen(false);
                  }}
                >
                  {isExpanded ? 'Collapse' : 'Expand'}
                </button>
              )}
              {onRequestTranscript && (
                <>
                  <button
                    type="button"
                    className="helpin-conversation-menu-item"
                    onClick={() => {
                      if (transcriptEmail) {
                        void handleTranscriptRequest();
                      } else {
                        setShowTranscriptForm(true);
                      }
                    }}
                  >
                    Transcript
                  </button>
                  {showTranscriptForm && (
                    <div className="helpin-conversation-transcript">
                      <label className="helpin-conversation-transcript-label" htmlFor="helpin-transcript-email">
                        Send transcript to
                      </label>
                      <input
                        id="helpin-transcript-email"
                        type="email"
                        className="helpin-conversation-transcript-input"
                        value={transcriptEmailInput}
                        onInput={(event) => setTranscriptEmailInput((event.currentTarget as HTMLInputElement).value)}
                        placeholder="you@example.com"
                      />
                      <div className="helpin-conversation-transcript-actions">
                        <button
                          type="button"
                          className="helpin-conversation-transcript-btn"
                          onClick={() => void handleTranscriptRequest()}
                          disabled={isSendingTranscript}
                        >
                          {isSendingTranscript ? 'Sending...' : 'Send transcript'}
                        </button>
                      </div>
                    </div>
                  )}
                  {transcriptStatus && (
                    <div className={`helpin-conversation-transcript-status helpin-conversation-transcript-status--${transcriptStatus.type}`}>
                      {transcriptStatus.message}
                    </div>
                  )}
                </>
              )}
            </div>
          )}
          {onClose && (
            <button
              type="button"
              className="helpin-window-close-inline"
              onClick={onClose}
              aria-label="Close"
            >
              <XIcon size={18} />
            </button>
          )}
        </div>
      </div>

      {(showEscalationEmailCapture || emailCaptureStatus === 'saved') && (
        <div className="helpin-escalation-email-capture">
          {emailCaptureStatus === 'saved' ? (
            <p className="helpin-escalation-email-capture__ok" role="status">
              You’re all set. We’ll email you when our team replies.
            </p>
          ) : (
            <>
              <p className="helpin-escalation-email-capture__label">
                Leave your email and we'll reply there too.
              </p>
              <form
                onSubmit={async (e) => {
                  e.preventDefault();
                  if (!onCaptureEmail || emailCaptureStatus === 'saving') return;
                  setEmailCaptureStatus('saving');
                  setEmailCaptureError('');
                  try {
                    await onCaptureEmail(transcriptEmailInput.trim());
                    setEmailCaptureStatus('saved');
                  } catch {
                    setEmailCaptureStatus('idle');
                    setEmailCaptureError('We couldn’t save your email. Please try again.');
                  }
                }}
                className="helpin-escalation-email-capture__form"
              >
                <input
                  type="email"
                  required
                  aria-label="Email for reply notifications"
                  autoComplete="email"
                  placeholder="you@example.com"
                  value={transcriptEmailInput}
                  disabled={emailCaptureStatus === 'saving'}
                  onInput={(e) => setTranscriptEmailInput((e.target as HTMLInputElement).value)}
                  className="helpin-transcript-email-input"
                />
                <button type="submit" disabled={emailCaptureStatus === 'saving'}>
                  {emailCaptureStatus === 'saving' ? 'Saving…' : 'Notify me by email'}
                </button>
              </form>
              {emailCaptureError && <p role="alert">{emailCaptureError}</p>}
            </>
          )}
        </div>
      )}

      <div className="helpin-conversation-thread" ref={threadRef}>
        <SpecialNoticeBanner text={availability?.specialNoticeText} workspaceId={config.workspaceId} />
        <MessageList
          messages={displayMessages}
          showDateSeparators={true}
          config={config}
          onImageClick={handleImageClick}
          onAnswerFeedback={onAnswerFeedback}
        />
        {showCsat && (
          <div className="helpin-csat-shell">
            <CsatRating onSubmit={onCsatSubmit!} />
          </div>
        )}
      </div>

      {isTyping && !isAIThinking && (
        <TypingIndicator
          agentName={typingAgentName}
          agentAvatar={typingAgentAvatar}
        />
      )}
      {isAIThinking && (
        <div className="helpin-ai-thinking" role="status" aria-live="polite">
          <span className="helpin-ai-thinking-icon" aria-hidden="true">
            <AIThinkingMark className="helpin-ai-thinking-mark" />
            <span className="helpin-ai-thinking-status" />
          </span>
          <span className="helpin-ai-thinking-copy">
            <span className="helpin-ai-thinking-label" key={aiProgressLabel}>{aiProgressLabel}</span>
            <span className="helpin-ai-thinking-shimmer" aria-hidden="true">
              <span className="helpin-ai-thinking-line helpin-ai-thinking-line--primary" />
              <span className="helpin-ai-thinking-line helpin-ai-thinking-line--secondary" />
            </span>
          </span>
        </div>
      )}
      {showHumanContactForm && (
        <PreChatForm
          config={config}
          onSubmit={completeHumanSupportRequest}
        />
      )}
      {showTalkToHumanButton && !showHumanContactForm && (
        <div className="helpin-talk-to-human">
          <button type="button" className="helpin-talk-to-human-btn" onClick={requestHumanSupport}>
            Talk to a person
          </button>
        </div>
      )}
      {isWaitingForTeammate && (
        <div className="helpin-waiting-teammate" role="status" aria-live="polite">
          {waitingTeammates.length > 0 && (
            <div className="helpin-waiting-teammate-avatars">
              {waitingTeammates.map((teammate) => (
                <div key={teammate.userId} className="helpin-waiting-teammate-avatar-wrap">
                  {teammate.avatarUrl ? (
                    <img
                      src={teammate.avatarUrl}
                      alt={teammate.name}
                      className="helpin-waiting-teammate-avatar"
                    />
                  ) : (
                    <div className="helpin-waiting-teammate-avatar helpin-waiting-teammate-avatar--placeholder">
                      {teammate.name ? teammate.name.charAt(0).toUpperCase() : '?'}
                    </div>
                  )}
                </div>
              ))}
            </div>
          )}
          <span className="helpin-waiting-teammate-copy">
            <span className="helpin-waiting-teammate-label">{handoffProgress.title}</span>
            {handoffProgress.detail && <span className="helpin-waiting-teammate-detail">{handoffProgress.detail}</span>}
          </span>
        </div>
      )}
      {queuedMessageCount > 0 && (
        <div className="helpin-message-queue-status" role="status" aria-live="polite">
          {connectionStatus === 'connected'
            ? queuedMessageCount === 1 ? 'Sending message…' : `Sending ${queuedMessageCount} messages…`
            : `${queuedMessageCount === 1 ? '1 message saved' : `${queuedMessageCount} messages saved`} — sending automatically when reconnected`}
        </div>
      )}
      <ComposeBar
        draftScope={conversationKey}
        notice={showPrivacyNotice && policyUrl ? (
          <PrivacyNotice
            key={`${config.workspaceId}:${conversationKey}`}
            policyUrl={policyUrl}
            text={config.privacyNotice?.text || ''}
            workspaceId={config.workspaceId}
            conversationId={conversation?.id}
            onDismiss={!conversation?.id ? onDismissDraftPrivacy : undefined}
          />
        ) : null}
        onSend={handleSendMessage}
        onTyping={onTyping}
        onFilesSelected={uploads.select}
        showBranding={config.branding?.showBranding ?? true}
        pendingAttachments={pendingAttachments}
        onRemoveAttachment={uploads.remove}
        onRetryAttachment={uploads.retry}
        attachmentError={uploads.validationError}
        fileUploadsEnabled={fileUploadsEnabled}
        disabled={composeDisabled}
        placeholder={composePlaceholder}
        workspaceId={config.workspaceId}
        workspaceName={config.workspaceName}
      />

      {lightboxImage && (
        <ImageLightbox
          src={lightboxImage.src}
          alt={lightboxImage.alt}
          onClose={() => setLightboxImage(null)}
        />
      )}
    </div>
  );
};
