import { FunctionComponent } from 'preact';
import { useCallback, useMemo, useState } from 'preact/hooks';
import type { Message, PendingAttachment, WidgetConfig } from '../types';
import { MessageList } from './MessageList';
import { ComposeBar } from './ComposeBar';
import { TypingIndicator } from './TypingIndicator';
import { PreChatForm } from './PreChatForm';
import { ImageLightbox } from './ImageLightbox';
import { ChevronLeftIcon, MoreVerticalIcon, XIcon } from './icons';

const MAX_FILE_SIZE = 10 * 1024 * 1024; // 10 MB

interface ConversationViewProps {
  config: WidgetConfig;
  messages: Message[];
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
  showPreChatForm?: boolean;
  onPreChatSubmit?: (data: { phone: string; email: string }) => void;
  onImageClick?: (src: string, alt: string) => void;
}

export const ConversationView: FunctionComponent<ConversationViewProps> = ({
  config,
  messages,
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
  showPreChatForm = false,
  onPreChatSubmit,
  onImageClick: externalImageClick,
}) => {
  const [introCreatedAt] = useState(() => new Date().toISOString());
  const [preChatDone, setPreChatDone] = useState(false);
  const [pendingAttachments, setPendingAttachments] = useState<PendingAttachment[]>([]);
  const [lightboxImage, setLightboxImage] = useState<{ src: string; alt: string } | null>(null);

  const workspaceName = config.workspaceName || 'Support';
  const logoUrl = config.branding?.logoUrl;
  const welcomeMessage = config.branding?.welcomeMessage || 'Hi there. How can we help?';
  const availability = config.availability;
  const hasTeamReply = messages.some((message) => message.role !== 'customer');
  const hasCustomerMessage = messages.some((message) => message.role === 'customer');
  const fileUploadsEnabled = Boolean(config.features?.fileUploads && onUploadAttachment);
  const showTalkToHumanButton = Boolean(
    config.features?.showTalkToHuman &&
      onEscalateToHuman &&
      messages.length > 0 &&
      !isAIThinking &&
      !isTyping,
  );

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

  const aiEnabled = config.features?.aiEnabled;
  const introRole = aiEnabled ? 'ai' as const : 'agent' as const;
  const introName = aiEnabled ? 'Helpin AI' : workspaceName;
  const introAvatar = aiEnabled ? undefined : (logoUrl || undefined);

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
          {activeAgent?.avatar ? (
            <img src={activeAgent.avatar} alt={activeAgent.name} className="helpin-conversation-logo helpin-agent-avatar" />
          ) : activeAgent?.name ? (
            <div className="helpin-conversation-logo-placeholder">
              <span>{activeAgent.name.charAt(0).toUpperCase()}</span>
            </div>
          ) : logoUrl ? (
            <img src={logoUrl} alt={workspaceName} className="helpin-conversation-logo" />
          ) : (
            <div className="helpin-conversation-logo-placeholder">
              <span>{workspaceName.charAt(0).toUpperCase()}</span>
            </div>
          )}

          <div className="helpin-conversation-brand-copy">
            {activeAgent?.name ? (
              <>
                <span className="helpin-conversation-title">{activeAgent.name}</span>
                <span className="helpin-conversation-subtitle">
                  {activeAgent.isAI ? 'Our bot will reply to your questions' : `from ${workspaceName}`}
                </span>
              </>
            ) : (
              <>
                <span className="helpin-conversation-title">{config.features?.aiEnabled ? 'Helpin AI' : workspaceName}</span>
                <span className="helpin-conversation-subtitle">
                  {config.features?.aiEnabled
                    ? 'Our bot will reply to your questions'
                    : availability?.statusText || 'Online now'}
                </span>
              </>
            )}
          </div>
        </div>

        <div className="helpin-conversation-header-actions">
          <button
            type="button"
            className="helpin-conversation-header-btn"
            aria-label="More options"
          >
            <MoreVerticalIcon size={20} />
          </button>
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
