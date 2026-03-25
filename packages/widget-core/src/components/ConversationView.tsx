import { FunctionComponent } from 'preact';
import { useCallback, useEffect, useMemo, useRef, useState } from 'preact/hooks';
import type { ActiveTeammate, Conversation, Message, PendingAttachment, WidgetConfig } from '../types';
import { MessageList } from './MessageList';
import { ComposeBar } from './ComposeBar';
import { TypingIndicator } from './TypingIndicator';
import { PreChatForm } from './PreChatForm';
import { ImageLightbox } from './ImageLightbox';
import { ChevronLeftIcon, MoreVerticalIcon, XIcon } from './icons';

const MAX_FILE_SIZE = 10 * 1024 * 1024; // 10 MB

interface ConversationViewProps {
  config: WidgetConfig;
  conversation?: Conversation;
  messages: Message[];
  activeTeammate?: ActiveTeammate;
  onSendMessage: (content: string, attachmentIds?: string[]) => void;
  onTyping?: (content: string) => void;
  onUploadAttachment?: (file: File, localId: string) => Promise<{ attachmentId: string; url: string } | null>;
  isTyping?: boolean;
  isAIThinking?: boolean;
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
  onPreChatSubmit?: (data: { phone: string; email: string }) => void;
  onImageClick?: (src: string, alt: string) => void;
}

export const ConversationView: FunctionComponent<ConversationViewProps> = ({
  config,
  conversation,
  messages,
  activeTeammate,
  onSendMessage,
  onTyping,
  onUploadAttachment,
  isTyping = false,
  isAIThinking = false,
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
  onPreChatSubmit,
  onImageClick: externalImageClick,
}) => {
  const [introCreatedAt] = useState(() => new Date().toISOString());
  const [preChatDone, setPreChatDone] = useState(false);
  const [pendingAttachments, setPendingAttachments] = useState<PendingAttachment[]>([]);
  const [lightboxImage, setLightboxImage] = useState<{ src: string; alt: string } | null>(null);
  const [menuOpen, setMenuOpen] = useState(false);
  const [showTranscriptForm, setShowTranscriptForm] = useState(false);
  const [transcriptEmailInput, setTranscriptEmailInput] = useState(transcriptEmail || '');
  const [transcriptStatus, setTranscriptStatus] = useState<{ type: 'success' | 'error'; message: string } | null>(null);
  const [isSendingTranscript, setIsSendingTranscript] = useState(false);
  const menuRef = useRef<HTMLDivElement | null>(null);

  const workspaceName = config.workspaceName || 'Support';
  const logoUrl = config.branding?.logoUrl;
  const welcomeMessage = config.branding?.welcomeMessage || 'Hi there. How can we help?';
  const availability = config.availability;
  const aiFirst = Boolean(config.features?.aiFirst);
  const hasTeamReply = messages.some((message) => message.role !== 'customer');
  const hasHumanReply = messages.some((message) => message.role === 'agent');
  const hasCustomerMessage = messages.some((message) => message.role === 'customer');
  const fileUploadsEnabled = Boolean(config.features?.fileUploads && onUploadAttachment);
  const hasHumanHandoffAlready = Boolean(
    conversation?.aiState === 'escalated' ||
      conversation?.status === 'resolved' ||
      conversation?.status === 'closed' ||
      conversation?.flowState === 'waiting_for_human' ||
      conversation?.flowState === 'queued_for_human' ||
      conversation?.flowState === 'after_hours_queue' ||
      conversation?.flowState === 'assigned_to_human' ||
      conversation?.flowState === 'resolved_by_human',
  );
  const showTalkToHumanButton = Boolean(
    config.features?.showTalkToHuman &&
      onEscalateToHuman &&
      messages.length > 0 &&
      !hasHumanHandoffAlready &&
      !isAIThinking &&
      !isTyping,
  );

  // Derive the most recent responding agent from messages.
  const activeAgent = useMemo(() => {
    if (activeTeammate?.name && !messages.some((message) => message.role === 'agent')) {
      return { name: activeTeammate.name, avatar: activeTeammate.avatarUrl, isAI: false };
    }
    for (let i = messages.length - 1; i >= 0; i--) {
      const m = messages[i];
      if (m.role === 'ai') {
        return { name: 'Helpin AI', avatar: m.senderAvatar, isAI: true };
      }
      if (m.role === 'agent' && m.senderName) {
        return { name: m.senderName, avatar: m.senderAvatar, isAI: false };
      }
    }
    if (activeTeammate?.name) {
      return { name: activeTeammate.name, avatar: activeTeammate.avatarUrl, isAI: false };
    }
    return null;
  }, [activeTeammate, messages]);

  const introRole = aiFirst ? 'ai' as const : 'agent' as const;
  const introName = aiFirst ? 'Helpin AI' : workspaceName;
  const introAvatar = aiFirst ? undefined : (logoUrl || undefined);
  const showHumanHandoffState = showHumanAvailability && !hasHumanReply;
  const showActiveAgentHeader = Boolean(activeAgent?.name) && !showHumanHandoffState;
  const teammateStatus = activeTeammate?.status || 'online';

  useEffect(() => {
    setTranscriptEmailInput(transcriptEmail || '');
  }, [transcriptEmail]);

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

  const handleFilesSelected = useCallback(async (files: File[]) => {
    if (!onUploadAttachment) return;

    for (const file of files) {
      if (file.size > MAX_FILE_SIZE) continue;

      const localId = `pending-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
      const previewUrl = file.type.startsWith('image/') ? URL.createObjectURL(file) : undefined;

      const pending: PendingAttachment = {
        id: localId,
        fileName: file.name,
        fileType: file.type || 'application/octet-stream',
        fileSize: file.size,
        progress: 0,
        status: 'uploading',
        previewUrl,
      };

      setPendingAttachments(prev => [...prev, pending]);

      try {
        const result = await onUploadAttachment(file, localId);
        if (result) {
          setPendingAttachments(prev =>
            prev.map(a => a.id === localId
              ? { ...a, status: 'uploaded' as const, progress: 100, attachmentId: result.attachmentId }
              : a
            )
          );
        } else {
          setPendingAttachments(prev =>
            prev.map(a => a.id === localId ? { ...a, status: 'error' as const } : a)
          );
        }
      } catch {
        setPendingAttachments(prev =>
          prev.map(a => a.id === localId ? { ...a, status: 'error' as const } : a)
        );
      }
    }
  }, [onUploadAttachment]);

  const handleRemoveAttachment = useCallback((id: string) => {
    setPendingAttachments(prev => {
      const att = prev.find(a => a.id === id);
      if (att?.previewUrl) URL.revokeObjectURL(att.previewUrl);
      return prev.filter(a => a.id !== id);
    });
  }, []);

  const handleSendMessage = useCallback((content: string, attachmentIds?: string[]) => {
    onSendMessage(content, attachmentIds);
    // Clean up preview URLs and clear pending attachments.
    setPendingAttachments(prev => {
      for (const a of prev) {
        if (a.previewUrl) URL.revokeObjectURL(a.previewUrl);
      }
      return [];
    });
  }, [onSendMessage]);

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
            <img src={logoUrl} alt={workspaceName} className="helpin-conversation-logo" />
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
                    ? availability?.statusText || 'Our team will follow up as soon as someone is available.'
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
              {onToggleExpanded && (
                <button
                  type="button"
                  className="helpin-conversation-menu-item"
                  onClick={() => {
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
              <XIcon size={16} />
            </button>
          )}
        </div>
      </div>

      <div className="helpin-conversation-thread">
        {showHumanHandoffState && (
          <div className="helpin-human-handoff-banner">
            <p className="helpin-human-handoff-title">
              {config.features?.escalationMessage || "We're handing this over to a team member."}
            </p>
            <p className="helpin-human-handoff-meta">
              {availability?.replyTimeText || availability?.outsideHoursMessage || 'Our team will follow up as soon as someone is available.'}
            </p>
          </div>
        )}
        <MessageList
          messages={displayMessages}
          showDateSeparators={true}
          config={config}
          onImageClick={handleImageClick}
        />
        {showPreChatForm && !preChatDone && hasCustomerMessage && hasTeamReply && !isAIThinking && onPreChatSubmit && (
          <PreChatForm
            config={config}
            onSubmit={(data) => {
              onPreChatSubmit(data);
              setPreChatDone(true);
            }}
          />
        )}
      </div>

      {isTyping && !isAIThinking && (
        <TypingIndicator
          agentName={typingAgentName}
          agentAvatar={typingAgentAvatar}
        />
      )}
      {isAIThinking && (
        <div className="helpin-ai-thinking">
          <div className="helpin-ai-thinking-icon">
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <path d="M12 2a4 4 0 0 1 4 4c0 1.95-1.4 3.58-3.25 3.93L12 22" />
              <path d="M12 2a4 4 0 0 0-4 4c0 1.95 1.4 3.58 3.25 3.93" />
            </svg>
          </div>
          <span className="helpin-ai-thinking-text">Thinking</span>
          <span className="helpin-ai-thinking-dots"><span>.</span><span>.</span><span>.</span></span>
        </div>
      )}
      {showTalkToHumanButton && (
        <div className="helpin-talk-to-human">
          <button type="button" className="helpin-talk-to-human-btn" onClick={onEscalateToHuman}>
            Talk to a human
          </button>
        </div>
      )}
      <ComposeBar
        onSend={handleSendMessage}
        onTyping={onTyping}
        onFilesSelected={handleFilesSelected}
        showBranding={config.branding?.showBranding ?? true}
        pendingAttachments={pendingAttachments}
        onRemoveAttachment={handleRemoveAttachment}
        fileUploadsEnabled={fileUploadsEnabled}
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
