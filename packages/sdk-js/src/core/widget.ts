import { mountWidget, unmountWidget, SYSTEM_EVENT_TYPES } from '@helpin-ai/widget-core';
import type { WidgetConfig, Message, Conversation, WidgetView } from '@helpin-ai/widget-core';
// @ts-ignore — Vite ?inline import returns CSS as a string
import widgetStyles from '@helpin-ai/widget-core/styles?inline';
import { isBot } from '../utils/bot-detect';
import {
  getOrCreateAnonymousId,
  getStoredSession,
  persistSession,
  clearSession,
  getStoredIdentity,
  persistIdentity,
  clearIdentity,
  clearConfigCache,
  getCachedConfig,
  cacheConfig,
} from './identity';

export { type WidgetConfig };

export interface WidgetUser {
  email?: string;
  name?: string;
  firstName?: string;
  lastName?: string;
  first_name?: string;
  last_name?: string;
  userId?: string;
  createdAt?: string;
  metadata?: Record<string, unknown>;
}

export interface WidgetSettings {
  widgetKey?: string;
  key?: string;
  host?: string;
  user?: WidgetUser;
}

export interface ShowArticleOptions {
  collectionId?: string;
  spaceId?: string;
}

type WidgetCallback = (...args: any[]) => void;
type ConnectionStatus = 'idle' | 'connecting' | 'connected' | 'disconnected' | 'failed';

type WidgetActiveTeammate = {
  userId: string;
  name: string;
  avatarUrl?: string;
  status?: 'online' | 'away' | 'offline';
};

type WidgetConversation = Conversation & {
  activeTeammate?: WidgetActiveTeammate;
  flowState?: string;
  aiState?: string;
};

const MAX_WS_RETRIES = 10;
const MAX_WS_INITIAL_RETRIES = 3; // retries before first successful connection (invalid key, server down)
const WS_BASE_DELAY_MS = 1000;
const WS_MAX_DELAY_MS = 30000;
const MAX_AUTO_RECONNECT_WINDOW_MS = 25_000;
const MAX_BACKGROUND_RETRY_DELAY_MS = 120_000;
const RECEIVED_MESSAGE_SOUND_URL = 'https://cdn.helpin.ai/sounds/ping.mp3';
const SENT_MESSAGE_SOUND_URL = 'https://cdn.helpin.ai/sounds/submit.mp3';

function normalizeWidgetConfig(raw: any): WidgetConfig {
  const teammates = Array.isArray(raw?.availableTeammates)
    ? raw.availableTeammates
        .map((teammate: any) => ({
          userId: teammate.userId || teammate.user_id || '',
          name: teammate.name || '',
          avatarUrl: teammate.avatarUrl || teammate.avatar_url || undefined,
          status: teammate.status || undefined,
        }))
        .filter((teammate: WidgetActiveTeammate) => Boolean(teammate.userId && teammate.name))
    : [];

  return {
    ...raw,
    availableTeammates: teammates,
  } as WidgetConfig;
}

function buildWidgetUserDisplayName(firstName?: string, lastName?: string, fallbackName?: string): string {
  const joined = [firstName?.trim(), lastName?.trim()].filter(Boolean).join(' ');
  return joined || fallbackName?.trim() || '';
}

export class WidgetManager {
  private config: WidgetSettings | null = null;
  private widgetConfig: WidgetConfig | null = null;
  private isVisible = false;
  private isOpen = false;
  private unreadCount = 0;
  private titleUnreadByConversation = new Map<string, number>();
  private sessionToken: string | null = null;
  private wsConnection: WebSocket | null = null;
  private wsRetryCount = 0;
  private wsHasConnected = false;
  private wsRetryTimer: ReturnType<typeof setTimeout> | null = null;
  private connectionIssueStartedAt: number | null = null;
  private isShutdown = false;
  private hasBeenOpened = false;
  private host = 'client.prod.helpin.ai';
  private widgetKey: string | null = null;
  private anonymousId: string | null = null;
  private connectionStatus: ConnectionStatus = 'idle';

  // Preact mount state
  private mountContainer: HTMLElement | null = null;
  private shadowRoot: ShadowRoot | null = null;
  private messages: Message[] = [];
  private conversations: WidgetConversation[] = [];
  private activeConversationId: string | null = null;
  private currentView: WidgetView = 'home';
  private openArticleRequest: { key: number; articleSlug: string } | null = null;
  private articleRequestKey = 0;
  private isTyping = false;
  private isAIThinking = false;
  private typingAgentName: string | undefined;
  private typingAgentAvatar: string | undefined;
  private activeTeammate: WidgetActiveTeammate | undefined;
  private currentEmail: string | null = null;
  private isConversationExpanded = false;
  private originalDocumentTitle: string | null = null;
  private pageIsFocused = true;
  private preChatDone = false;
  private receivedMessageAudio: HTMLAudioElement | null = null;
  private receivedMessageAudioUnlocked = false;
  private sentMessageAudio: HTMLAudioElement | null = null;
  private sentMessageAudioUnlocked = false;
  private audioUnlockListener: (() => void) | null = null;
  private visibilityChangeListener: (() => void) | null = null;
  private focusListener: (() => void) | null = null;
  private blurListener: (() => void) | null = null;

  private callbacks: Record<string, WidgetCallback[]> = {
    onOpen: [],
    onClose: [],
    onUnreadCountChange: [],
    onUserEmailSupplied: [],
    onConversationStarted: [],
    onMessageReceived: [],
  };

  boot(settings: WidgetSettings): void {
    // Bot/crawler filtering
    if (isBot()) return;

    if (typeof window === 'undefined' || typeof document === 'undefined') {
      console.error('[Helpin] Widget boot skipped: browser APIs are unavailable.');
      return;
    }

    const widgetKey = settings.widgetKey || settings.key || '';
    if (!widgetKey) {
      console.error('[Helpin] Widget boot skipped: widgetKey is required.');
      return;
    }

    // Clean up previous boot if any
    if (this.config) {
      this.cleanup();
    }

    this.isShutdown = false;
    this.isVisible = true;
    this.originalDocumentTitle = document.title;
    this.pageIsFocused = this.computePageIsFocused();
    this.registerTitleNotificationListeners();
    this.config = {
      ...settings,
      widgetKey,
    };
    this.widgetKey = widgetKey;

    // Restore pre-chat done state from localStorage.
    if (this.widgetKey) {
      try { this.preChatDone = localStorage.getItem(`helpin_prechat_${this.widgetKey}`) === '1'; } catch { /* ignore */ }
    }

    if (settings.host) {
      this.host = settings.host.replace(/^https?:\/\//, '');
    }

    // Get or create anonymous ID from cookie
    this.anonymousId = this.widgetKey ? getOrCreateAnonymousId(this.widgetKey) : null;

    // Unlock notification audio on first user interaction with the page
    if (!this.audioUnlockListener) {
      this.audioUnlockListener = () => {
        this.unlockReceivedMessageSound();
        this.unlockSentMessageSound();
        document.removeEventListener('click', this.audioUnlockListener!);
        document.removeEventListener('touchstart', this.audioUnlockListener!);
        this.audioUnlockListener = null;
      };
      document.addEventListener('click', this.audioUnlockListener, { once: true });
      document.addEventListener('touchstart', this.audioUnlockListener, { once: true });
    }

    // Fetch widget config (with localStorage caching), then connect WS
    this.fetchWidgetConfig().then(() => {
      this.connectWebSocket();
    }).catch((error) => {
      console.error('Failed to fetch widget config during boot:', error);
    });
  }

  shutdown(): void {
    this.isShutdown = true;

    // 1. Revoke session — prefer WS, fall back to HTTP
    if (this.wsConnection?.readyState === WebSocket.OPEN) {
      this.wsSend('session:revoke', {});
      this.disconnectWebSocket();
    } else {
      if (this.sessionToken && this.host) {
        const url = this.host.startsWith('http') ? this.host : `https://${this.host}`;
        fetch(`${url}/widget/session/revoke`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ session_token: this.sessionToken }),
        }).catch(() => {}); // best-effort
      }
    }

    // 2. Clear persisted session + identity (but NOT anonymous_id cookie)
    if (this.widgetKey) {
      clearSession(this.widgetKey);
      clearIdentity(this.widgetKey);
      clearConfigCache(this.widgetKey);
      try { localStorage.removeItem(`helpin_prechat_${this.widgetKey}`); } catch { /* ignore */ }
    }
    this.preChatDone = false;

    // 3. Reset in-memory state + unmount widget
    this.cleanup();
  }

  isActive(): boolean {
    return this.sessionToken !== null && !this.isShutdown;
  }

  getCurrentEmail(): string | null {
    return this.currentEmail;
  }

  private cleanup(): void {
    this.stopTyping();
    this.restoreDocumentTitle();
    this.unregisterTitleNotificationListeners();
    this.config = null;
    this.widgetConfig = null;
    this.isVisible = false;
    this.sessionToken = null;
    this.isOpen = false;
    this.unreadCount = 0;
    this.titleUnreadByConversation.clear();
    this.hasBeenOpened = false;
    this.wsRetryCount = 0;
    this.connectionIssueStartedAt = null;
    this.messages = [];
    this.conversations = [];
    this.activeConversationId = null;
    this.currentView = 'home';
    this.openArticleRequest = null;
    this.articleRequestKey = 0;
    this.isTyping = false;
    this.connectionStatus = 'idle';
    this.activeTeammate = undefined;
    this.currentEmail = null;
    this.isConversationExpanded = false;
    this.originalDocumentTitle = null;
    this.pageIsFocused = true;

    if (this.keepaliveTimer) {
      clearInterval(this.keepaliveTimer);
      this.keepaliveTimer = null;
    }

    if (this.wsRetryTimer) {
      clearTimeout(this.wsRetryTimer);
      this.wsRetryTimer = null;
    }

    this.disconnectWebSocket();
    this.removeWidget();
  }

  show(): void {
    this.isVisible = true;
    this.ensureWidget();
    this.render();
  }

  hide(): void {
    this.isVisible = false;
    this.isOpen = false;
    this.updateDocumentTitle();
    this.render();
  }

  open(): void {
    this.isVisible = true;
    this.isOpen = true;
    this.unlockReceivedMessageSound();
    this.unlockSentMessageSound();
    if (!this.hasBeenOpened && this.currentView === 'home') {
      this.hasBeenOpened = true;
      // If there's an active conversation (restored session), resume it;
      // otherwise start fresh in conversation view.
      this.currentView = 'conversation';
    } else {
      this.hasBeenOpened = true;
    }
    this.clearTitleUnread(this.currentView === 'conversation' ? this.activeConversationId : null);
    this.ensureWidget();
    this.updateDocumentTitle();
    this.render();
  }

  close(): void {
    this.isOpen = false;
    this.updateDocumentTitle();
    this.render();
  }

  private openFromUser(): void {
    this.open();
    this.triggerCallback('onOpen');
  }

  private closeFromUser(): void {
    this.isOpen = false;
    this.render();
    this.triggerCallback('onClose');
  }

  toggle(): void {
    this.isVisible = true;
    if (this.isOpen) {
      this.close();
    } else {
      this.open();
    }
  }

  private toggleFromUser(): void {
    this.isVisible = true;
    if (this.isOpen) {
      this.closeFromUser();
    } else {
      this.openFromUser();
    }
  }

  openMessages(): void {
    this.currentView = 'messages';
    this.open();
  }

  openNewMessage(content?: string): void {
    this.resetActiveConversation();
    this.currentView = 'conversation';

    // Tell server to clear active conversation so next message creates a new one
    if (this.wsConnection?.readyState === WebSocket.OPEN) {
      this.wsSend('conversation:new', {});
    }

    this.open();

    // If content provided, send it as the first message
    if (content?.trim()) {
      this.handleSendMessage(content);
    }
  }

  openConversation(conversationId: string): void {
    this.currentView = 'home';
    this.open();
  }

  toggleConversationExpanded(): void {
    this.isConversationExpanded = !this.isConversationExpanded;
    this.render();
  }

  async requestConversationTranscript(email?: string): Promise<{ success: boolean; message: string }> {
    if (!this.sessionToken) {
      throw new Error('Session not ready');
    }
    if (!this.activeConversationId) {
      throw new Error('No active conversation');
    }

    const response = await fetch(
      `https://${this.host}/widget/conversations/${encodeURIComponent(this.activeConversationId)}/transcript`,
      {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          session_token: this.sessionToken,
          email,
        }),
      },
    );

    const payload = await response.json().catch(() => ({}));
    if (!response.ok) {
      throw new Error(payload?.error || 'Unable to send transcript right now.');
    }

    if (email && !this.currentEmail) {
      this.currentEmail = email;
    }

    return {
      success: Boolean(payload?.success),
      message: payload?.message || 'Transcript sent.',
    };
  }

  private getConversationIdFromHash(): string | null {
    if (typeof window === 'undefined') return null;
    const hash = window.location.hash || '';
    if (!hash.startsWith('#helpin-conv=')) return null;
    const conversationId = hash.slice('#helpin-conv='.length).trim();
    return conversationId || null;
  }

  openArticle(articleId: string, _options?: ShowArticleOptions): void {
    this.articleRequestKey += 1;
    this.openArticleRequest = {
      key: this.articleRequestKey,
      articleSlug: articleId,
    };
    this.currentView = 'help-article';
    this.open();
  }

  onOpen(callback: WidgetCallback): void {
    this.callbacks.onOpen.push(callback);
  }

  onClose(callback: WidgetCallback): void {
    this.callbacks.onClose.push(callback);
  }

  onUnreadCountChange(callback: WidgetCallback): void {
    this.callbacks.onUnreadCountChange.push(callback);
    callback(this.unreadCount);
  }

  onUserEmailSupplied(callback: WidgetCallback): void {
    this.callbacks.onUserEmailSupplied.push(callback);
  }

  onConversationStarted(callback: WidgetCallback): void {
    this.callbacks.onConversationStarted.push(callback);
  }

  onMessageReceived(callback: WidgetCallback): void {
    this.callbacks.onMessageReceived.push(callback);
  }

  getVisitorId(): string {
    return this.anonymousId || '';
  }

  isWidgetReady(): boolean {
    return this.widgetConfig !== null;
  }

  // ─── Preact Rendering ─────────────────────────────────────

  private ensureWidget(): void {
    if (this.mountContainer) return;

    // Create host element
    const host = document.createElement('div');
    host.id = 'helpin-widget-container';
    host.style.cssText = 'position:fixed;z-index:2147483647;all:initial;';
    document.body.appendChild(host);

    // Shadow DOM for CSS isolation
    this.shadowRoot = host.attachShadow({ mode: 'open' });

    // Inject styles into shadow root
    const style = document.createElement('style');
    style.textContent = `:host { all: initial; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; }\n*, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }\n${widgetStyles}`;
    this.shadowRoot.appendChild(style);

    // Create mount container inside shadow root
    const container = document.createElement('div');
    container.id = 'helpin-widget-mount';
    this.shadowRoot.appendChild(container);
    this.mountContainer = container;
  }

  private render(): void {
    if (!this.mountContainer || !this.widgetConfig) return;

    // Inject brand color overrides as a <style> targeting .helpin-widget directly.
    // This beats the default --helpin-primary in widget.css because it has equal specificity
    // but appears later in the shadow DOM stylesheet order.
    const primaryColor = this.widgetConfig.branding?.primaryColor;
    if (primaryColor && this.shadowRoot) {
      const overrideId = 'helpin-brand-override';
      let overrideStyle = this.shadowRoot.getElementById(overrideId) as HTMLStyleElement | null;
      if (!overrideStyle) {
        overrideStyle = document.createElement('style');
        overrideStyle.id = overrideId;
        this.shadowRoot.appendChild(overrideStyle);
      }
      overrideStyle.textContent = `.helpin-widget, .helpin-launcher {
  --helpin-primary: ${primaryColor};
  --helpin-primary-hover: ${this.darkenColor(primaryColor, 15)};
  --helpin-primary-foreground: ${this.getContrastColor(primaryColor)};
}`;
    }

    // Show pre-chat form only when: feature is enabled AND session is anonymous (no email yet) AND not already completed/skipped.
    // When forceIdentify is on, ignore the skip flag — email is mandatory.
    const alreadyIdentified = !!this.currentEmail || !!this.config?.user?.email;
    const forceIdentify = !!this.widgetConfig.features?.forceIdentify;
    const showPreChat = !!this.widgetConfig.features?.preChatForm && !alreadyIdentified && (forceIdentify || !this.preChatDone);

    const mountOptions: Parameters<typeof mountWidget>[1] & {
      activeTeammate?: WidgetActiveTeammate;
      activeConversation?: WidgetConversation;
      openArticleRequest?: {
        key: number;
        articleSlug: string;
      };
      isConversationExpanded?: boolean;
      onToggleConversationExpanded?: () => void;
      transcriptEmail?: string;
      onRequestTranscript?: (email?: string) => Promise<{ success: boolean; message: string }>;
    } = {
      config: this.widgetConfig,
      messages: this.messages,
      isOpen: this.isOpen,
      onClose: () => this.closeFromUser(),
      onSendMessage: (content: string, attachmentIds?: string[]) => this.handleSendMessage(content, { attachmentIds }),
      onSendMessageFromHome: (content: string) => this.handleSendMessage(content, { startNewConversation: true }),
      onQuickReply: (content: string) => this.handleSendMessage(content),
      onUploadAttachment: (file: File, localId: string) => this.handleUploadAttachment(file, localId),
      onTyping: (content: string) => this.handleTyping(content),
      showPreChatForm: showPreChat,
      onPreChatSubmit: (data: { phone: string; email: string }) => this.handlePreChatSubmit(data),
      isTyping: this.isTyping,
      isAIThinking: this.isAIThinking,
      onEscalateToHuman: () => this.handleEscalateToHuman(),
      typingAgentName: this.typingAgentName,
      typingAgentAvatar: this.typingAgentAvatar,
      activeTeammate: this.activeTeammate,
      initialView: this.currentView,
      showLauncher: this.isVisible,
      onLauncherClick: () => this.toggleFromUser(),
      unreadCount: this.unreadCount,
      connectionStatus: this.connectionStatus,
      onRetryConnection: () => this.reconnectWebSocket(),
      conversations: this.conversations,
      activeConversation: this.activeConversationId
        ? this.conversations.find((conversation) => conversation.id === this.activeConversationId)
        : undefined,
      onSelectConversation: (id: string) => this.handleSelectConversation(id),
      onStartNewConversation: () => this.handleStartNewConversation(),
      isConversationExpanded: this.isConversationExpanded,
      onToggleConversationExpanded: () => this.toggleConversationExpanded(),
      transcriptEmail: this.currentEmail || undefined,
      onRequestTranscript: (email?: string) => this.requestConversationTranscript(email),
      widgetKey: this.widgetKey || undefined,
      host: this.host,
      openArticleRequest: this.openArticleRequest || undefined,
      onImageClick: (src: string, alt: string) => this.showImageLightbox(src, alt),
      onViewChange: (view: WidgetView) => {
        this.currentView = view;
        // Refresh conversations list from server when navigating to Messages tab
        if (view === 'messages' && this.wsConnection?.readyState === WebSocket.OPEN) {
          this.wsSend('conversations:list', {});
        }
      },
    };

    mountWidget(this.mountContainer, mountOptions);
  }

  private removeWidget(): void {
    if (this.mountContainer) {
      unmountWidget(this.mountContainer);
      // Remove the host element (parent of shadow root)
      const host = this.mountContainer.getRootNode();
      if (host instanceof ShadowRoot && host.host) {
        host.host.remove();
      } else if (this.mountContainer.parentElement) {
        this.mountContainer.parentElement.remove();
      }
      this.mountContainer = null;
      this.shadowRoot = null;
    }
  }

  // ─── Unread Count ──────────────────────────────────────────

  /** Preload and unlock audio playback (call from a user-gesture handler like show/toggle). */
  private unlockReceivedMessageSound(): void {
    if (this.receivedMessageAudioUnlocked) return;
    try {
      if (!this.receivedMessageAudio) {
        this.receivedMessageAudio = new Audio(RECEIVED_MESSAGE_SOUND_URL);
      }
      // Silent play to unlock autoplay policy, then pause
      this.receivedMessageAudio.volume = 0;
      this.receivedMessageAudio.play().then(() => {
        this.receivedMessageAudio!.pause();
        this.receivedMessageAudio!.currentTime = 0;
        this.receivedMessageAudioUnlocked = true;
      }).catch(() => {/* ignore */});
    } catch { /* audio not supported */ }
  }

  private playReceivedMessageSound(): void {
    try {
      if (!this.receivedMessageAudio) {
        this.receivedMessageAudio = new Audio(RECEIVED_MESSAGE_SOUND_URL);
      }
      this.receivedMessageAudio.volume = 0.5;
      this.receivedMessageAudio.currentTime = 0;
      this.receivedMessageAudio.play().catch(() => {/* autoplay blocked — ignore */});
    } catch { /* audio not supported — ignore */ }
  }

  /** Preload + unlock the customer "send" pop. Same gesture-unlock dance
   *  as the received-message sound so the first send doesn't get blocked
   *  by the browser's autoplay policy. */
  private unlockSentMessageSound(): void {
    if (this.sentMessageAudioUnlocked) return;
    try {
      if (!this.sentMessageAudio) {
        this.sentMessageAudio = new Audio(SENT_MESSAGE_SOUND_URL);
      }
      this.sentMessageAudio.volume = 0;
      this.sentMessageAudio.play().then(() => {
        this.sentMessageAudio!.pause();
        this.sentMessageAudio!.currentTime = 0;
        this.sentMessageAudioUnlocked = true;
      }).catch(() => {/* ignore */});
    } catch { /* audio not supported */ }
  }

  private playSentMessageSound(): void {
    try {
      if (!this.sentMessageAudio) {
        this.sentMessageAudio = new Audio(SENT_MESSAGE_SOUND_URL);
      }
      this.sentMessageAudio.volume = 0.4;
      this.sentMessageAudio.currentTime = 0;
      this.sentMessageAudio.play().catch(() => {/* autoplay blocked — ignore */});
    } catch { /* audio not supported — ignore */ }
  }

  private syncUnreadCount(): void {
    const total = this.conversations.reduce((sum, c) => {
      const unread = Number(c.unreadCount ?? 0);
      return sum + (Number.isFinite(unread) ? unread : 0);
    }, 0);
    if (total !== this.unreadCount) {
      this.unreadCount = total;
      this.triggerCallback('onUnreadCountChange', total);
    }
    this.syncTitleUnreadWithConversations();
    this.updateDocumentTitle();
  }

  private clearActiveConversationUnread(): void {
    if (!this.activeConversationId) return;
    const convIdx = this.conversations.findIndex(c => c.id === this.activeConversationId);
    if (convIdx >= 0 && this.conversations[convIdx].unreadCount) {
      const updated = { ...this.conversations[convIdx], unreadCount: 0 };
      this.conversations = this.conversations.map((c, i) => i === convIdx ? updated : c);
      this.syncUnreadCount();
    }
    this.clearTitleUnread(this.activeConversationId);
  }

  private isConversationVisibleToUser(conversationId?: string | null): boolean {
    return Boolean(
      conversationId &&
      this.pageIsFocused &&
      this.isOpen &&
      this.currentView === 'conversation' &&
      this.activeConversationId === conversationId,
    );
  }

  private markConversationRead(conversationId?: string | null): void {
    if (!conversationId) return;

    const convIdx = this.conversations.findIndex((conversation) => conversation.id === conversationId);
    if (convIdx >= 0 && this.conversations[convIdx].unreadCount) {
      const updated = { ...this.conversations[convIdx], unreadCount: 0 };
      this.conversations = this.conversations.map((conversation, index) => (
        index === convIdx ? updated : conversation
      ));
      this.syncUnreadCount();
    } else {
      this.clearTitleUnread(conversationId);
    }

    if (this.wsConnection?.readyState === WebSocket.OPEN) {
      this.wsSend('conversation:read', { conversation_id: conversationId });
    }
  }

  private computePageIsFocused(): boolean {
    if (typeof document === 'undefined') return true;
    return document.visibilityState === 'visible' && document.hasFocus();
  }

  private registerTitleNotificationListeners(): void {
    if (typeof window === 'undefined' || typeof document === 'undefined') return;

    this.visibilityChangeListener = () => {
      this.pageIsFocused = this.computePageIsFocused();
      if (this.pageIsFocused && this.isConversationVisibleToUser(this.activeConversationId)) {
        this.markConversationRead(this.activeConversationId);
      }
      this.updateDocumentTitle();
    };
    this.focusListener = () => {
      this.pageIsFocused = true;
      if (this.isConversationVisibleToUser(this.activeConversationId)) {
        this.markConversationRead(this.activeConversationId);
      }
      this.updateDocumentTitle();
    };
    this.blurListener = () => {
      this.pageIsFocused = this.computePageIsFocused();
      this.updateDocumentTitle();
    };

    document.addEventListener('visibilitychange', this.visibilityChangeListener);
    window.addEventListener('focus', this.focusListener);
    window.addEventListener('blur', this.blurListener);
  }

  private unregisterTitleNotificationListeners(): void {
    if (typeof window === 'undefined' || typeof document === 'undefined') return;
    if (this.visibilityChangeListener) {
      document.removeEventListener('visibilitychange', this.visibilityChangeListener);
      this.visibilityChangeListener = null;
    }
    if (this.focusListener) {
      window.removeEventListener('focus', this.focusListener);
      this.focusListener = null;
    }
    if (this.blurListener) {
      window.removeEventListener('blur', this.blurListener);
      this.blurListener = null;
    }
  }

  private restoreDocumentTitle(): void {
    if (typeof document === 'undefined' || this.originalDocumentTitle === null) return;
    document.title = this.originalDocumentTitle;
  }

  private getTitleUnreadCount(): number {
    return Array.from(this.titleUnreadByConversation.values()).reduce((sum, count) => sum + count, 0);
  }

  private syncTitleUnreadWithConversations(): void {
    if (!this.titleUnreadByConversation.size) return;

    const conversationIds = new Set(this.conversations.map((conversation) => conversation.id));
    for (const [conversationId] of this.titleUnreadByConversation.entries()) {
      const conversation = this.conversations.find((entry) => entry.id === conversationId);
      if (!conversationIds.has(conversationId) || Number(conversation?.unreadCount ?? 0) <= 0) {
        this.titleUnreadByConversation.delete(conversationId);
      }
    }
  }

  private clearTitleUnread(conversationId?: string | null): void {
    if (!conversationId) {
      this.updateDocumentTitle();
      return;
    }
    if (this.titleUnreadByConversation.delete(conversationId)) {
      this.updateDocumentTitle();
      return;
    }
    this.updateDocumentTitle();
  }

  private incrementTitleUnread(conversationId?: string): void {
    if (!conversationId) return;
    this.titleUnreadByConversation.set(
      conversationId,
      (this.titleUnreadByConversation.get(conversationId) ?? 0) + 1,
    );
  }

  private updateDocumentTitle(): void {
    if (typeof document === 'undefined') return;
    if (this.originalDocumentTitle === null) {
      this.originalDocumentTitle = document.title;
    }

    const unread = this.getTitleUnreadCount();
    if (unread <= 0 || this.pageIsFocused) {
      this.restoreDocumentTitle();
      return;
    }

    document.title = unread === 1 ? '(1) New reply' : `(${unread}) New replies`;
  }

  // ─── Message Handling ──────────────────────────────────────

  private resetActiveConversation(): void {
    this.activeConversationId = null;
    this.activeTeammate = undefined;
    this.messages = [];
    this.isTyping = false;
  }

  private mapActiveTeammate(raw: any): WidgetActiveTeammate | undefined {
    if (!raw || typeof raw !== 'object') return undefined;
    const userId = typeof raw.user_id === 'string' ? raw.user_id.trim() : '';
    const name = typeof raw.name === 'string' ? raw.name.trim() : '';
    if (!userId || !name) return undefined;
    return {
      userId,
      name,
      avatarUrl: typeof raw.avatar_url === 'string' && raw.avatar_url.trim() ? raw.avatar_url : undefined,
      status: raw.status === 'online' || raw.status === 'away' || raw.status === 'offline' ? raw.status : undefined,
    };
  }

  private updateAvailableTeammateStatus(userId: string, status: 'online' | 'away' | 'offline'): boolean {
    if (!this.widgetConfig?.availableTeammates?.length) {
      return false;
    }

    let changed = false;
    const current = this.widgetConfig.availableTeammates;
    const next = status === 'offline'
      ? current.filter((teammate) => {
          const keep = teammate.userId !== userId;
          if (!keep) {
            changed = true;
          }
          return keep;
        })
      : current.map((teammate) => {
          if (teammate.userId !== userId || teammate.status === status) {
            return teammate;
          }
          changed = true;
          return {
            ...teammate,
            status,
          };
        });

    if (!changed) {
      return false;
    }

    this.widgetConfig = {
      ...this.widgetConfig,
      availableTeammates: next,
    };

    return true;
  }

  private mapConversation(raw: any): WidgetConversation {
    return {
      id: raw.id,
      subject: raw.subject || 'Untitled',
      status: raw.status || 'open',
      flowState: raw.flow_state || undefined,
      aiState: raw.ai_state || undefined,
      lastMessage: raw.last_message,
      lastMessageAt: raw.updated_at || raw.created_at,
      unreadCount: raw.unread_count ?? 0,
      activeTeammate: this.mapActiveTeammate({
        user_id: raw.opened_by_user_id,
        name: raw.opened_by_display_name,
        avatar_url: raw.opened_by_avatar_url,
        status: raw.opened_by_status,
      }),
    };
  }

  private mapSupportMessage(raw: any): Message {
    let parsedMeta: any = null;
    if (raw?.metadata) {
      try { parsedMeta = typeof raw.metadata === 'string' ? JSON.parse(raw.metadata) : raw.metadata; } catch { /* ignore */ }
    }

    const isSystem = raw?.message_type === 'system';
    const isAI = raw?.sender_type === 'ai' || !!(parsedMeta?.ai_agent_id);
    const role: Message['role'] = isSystem
      ? 'system'
      : raw?.sender_type === 'customer'
        ? 'customer'
        : isAI
          ? 'ai'
          : 'agent';

    // Validate system_event_type against the shared union before surfacing it
    // so renderers can trust the value.
    let systemEventType: Message['systemEventType'];
    const rawEventType = typeof raw?.system_event_type === 'string' ? raw.system_event_type : undefined;
    if (rawEventType && Array.isArray(SYSTEM_EVENT_TYPES) && (SYSTEM_EVENT_TYPES as readonly string[]).includes(rawEventType)) {
      systemEventType = rawEventType as Message['systemEventType'];
    }

    const message: Message = {
      id: raw?.id || `ws-${Date.now()}`,
      conversationId: raw?.conversation_id || '',
      role,
      content: raw?.content || '',
      senderName: raw?.sender_display_name || raw?.sender_name || undefined,
      senderAvatar: raw?.sender_avatar_url || raw?.sender_avatar || undefined,
      systemEventType,
      viaChannel: raw?.via_channel || undefined,
      isInternal: raw?.is_internal || false,
      attachments: WidgetManager.mapAttachments(raw?.attachments),
      createdAt: raw?.created_at || new Date().toISOString(),
    };

    if (parsedMeta) {
      if (parsedMeta.ai_sources) message.sources = parsedMeta.ai_sources;
      if (parsedMeta.ai_confidence !== undefined) message.aiConfidence = parsedMeta.ai_confidence;
      if (Array.isArray(parsedMeta.link_previews)) message.linkPreviews = parsedMeta.link_previews;
    }

    return message;
  }

  private handleSendMessage(content: string, options: { startNewConversation?: boolean; attachmentIds?: string[] } = {}): void {
    if (!content.trim() && (!options.attachmentIds || options.attachmentIds.length === 0)) return;

    if (options.startNewConversation) {
      this.resetActiveConversation();
    }

    // If no active conversation, tell server to start a new one.
    // Server responds with conversation:created (real ID) before message:send is processed.
    if (!this.activeConversationId && this.wsConnection?.readyState === WebSocket.OPEN) {
      this.wsSend('conversation:new', {});
    }

    // Optimistic update
    const optimisticMsg: Message = {
      id: `temp-${Date.now()}`,
      conversationId: this.activeConversationId || '',
      role: 'customer',
      content,
      isInternal: false,
      createdAt: new Date().toISOString(),
    };
    this.messages = [...this.messages, optimisticMsg];
    this.render();
    this.playSentMessageSound();

    // Stop typing indicator before sending
    this.stopTyping();

    // Send via WS
    const payload: Record<string, unknown> = { content };
    if (options.attachmentIds && options.attachmentIds.length > 0) {
      payload.attachment_ids = options.attachmentIds;
    }

    if (this.wsConnection?.readyState === WebSocket.OPEN) {
      this.wsSend('message:send', payload);
    } else {
      // Fallback to HTTP if WS not available
      this.sendMessageHTTP(content);
    }
  }

  private async handleUploadAttachment(file: File, _localId: string): Promise<{ attachmentId: string; url: string } | null> {
    if (!this.sessionToken) return null;

    try {
      // Step 1: Initiate — get presigned URL from server
      const initResp = await fetch(`https://${this.host}/widget/support/attachments`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'X-Session-Token': this.sessionToken,
        },
        body: JSON.stringify({
          file_name: file.name,
          file_size: file.size,
          content_type: file.type || 'application/octet-stream',
        }),
      });

      if (!initResp.ok) {
        console.error('[helpin] Failed to initiate attachment upload:', initResp.status);
        return null;
      }

      const initData = await initResp.json();
      const attachmentId = initData.attachment?.id;
      const uploadUrl = initData.upload_url;
      const publicUrl = initData.public_url;

      if (!attachmentId || !uploadUrl) return null;

      // Step 2: Upload file directly to S3 via presigned PUT URL
      const uploadResp = await fetch(uploadUrl, {
        method: 'PUT',
        body: file,
        headers: {
          'Content-Type': file.type || 'application/octet-stream',
          'x-amz-acl': 'public-read',
        },
      });

      if (!uploadResp.ok) {
        console.error('[helpin] Failed to upload file to storage:', uploadResp.status);
        return null;
      }

      // Step 3: Confirm upload with server
      const confirmResp = await fetch(`https://${this.host}/widget/support/attachments/${attachmentId}/confirm`, {
        method: 'PATCH',
        headers: {
          'Content-Type': 'application/json',
          'X-Session-Token': this.sessionToken,
        },
      });

      if (!confirmResp.ok) {
        console.error('[helpin] Failed to confirm attachment upload:', confirmResp.status);
        return null;
      }

      return { attachmentId, url: publicUrl };
    } catch (error) {
      console.error('[helpin] Attachment upload error:', error);
      return null;
    }
  }

  private keepaliveTimer: ReturnType<typeof setInterval> | null = null;
  private typingTimer: ReturnType<typeof setTimeout> | null = null;
  private isSendingTyping = false;
  private lastTypingSentAt = 0;

  private handleTyping(content: string): void {
    if (!this.sessionToken) {
      if (import.meta.env.DEV) console.debug('[helpin] typing skipped — no session token');
      return;
    }

    const now = Date.now();
    const wasTyping = this.isSendingTyping;
    const shouldSendContent = now - this.lastTypingSentAt >= 300;

    if (!wasTyping || shouldSendContent) {
      this.isSendingTyping = true;
      this.lastTypingSentAt = now;
      if (this.wsConnection?.readyState === WebSocket.OPEN) {
        if (import.meta.env.DEV) console.debug('[helpin] sending typing:start via WS, conversationId:', this.activeConversationId);
        this.wsSend('typing:start', { content });
      } else if (!wasTyping) {
        // Only send HTTP fallback on the initial typing:start (no content preview over HTTP)
        if (import.meta.env.DEV) console.debug('[helpin] sending typing:start via HTTP fallback');
        void this.sendTypingHTTP(true);
      }
    }

    if (this.typingTimer) clearTimeout(this.typingTimer);
    this.typingTimer = setTimeout(() => {
      this.isSendingTyping = false;
      this.lastTypingSentAt = 0;
      if (this.wsConnection?.readyState === WebSocket.OPEN) {
        this.wsSend('typing:stop', {});
      } else {
        void this.sendTypingHTTP(false);
      }
    }, 5000);
  }

  private stopTyping(): void {
    if (this.isSendingTyping) {
      if (this.wsConnection?.readyState === WebSocket.OPEN) {
        this.wsSend('typing:stop', {});
      } else {
        void this.sendTypingHTTP(false);
      }
    }
    this.isSendingTyping = false;
    if (this.typingTimer) {
      clearTimeout(this.typingTimer);
      this.typingTimer = null;
    }
  }

  private async sendTypingHTTP(isTyping: boolean): Promise<void> {
    if (!this.sessionToken) return;
    try {
      await fetch(`https://${this.host}/widget/typing`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ session_token: this.sessionToken, is_typing: isTyping }),
      });
    } catch (error) {
      console.error('Failed to send typing indicator:', error);
    }
  }

  private async sendMessageHTTP(content: string): Promise<void> {
    if (!this.sessionToken) return;
    try {
      await fetch(`https://${this.host}/widget/messages`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ session_token: this.sessionToken, content }),
      });
    } catch (error) {
      console.error('Failed to send message:', error);
    }
  }

  private handlePreChatSubmit(data: { phone: string; email: string }): void {
    if (data.email) {
      this.triggerCallback('onUserEmailSupplied', data.email);
      this.currentEmail = data.email;
      // Persist identity so it survives page refresh
      if (this.widgetKey) {
        persistIdentity(this.widgetKey, data.email, '');
      }
    }

    // Mark pre-chat as done so it doesn't reappear on reload.
    this.preChatDone = true;
    if (this.widgetKey) {
      try { localStorage.setItem(`helpin_prechat_${this.widgetKey}`, '1'); } catch { /* ignore */ }
    }

    // Upgrade session via WS with source=prechat (even if email is empty — server handles skip)
    if (this.wsConnection?.readyState === WebSocket.OPEN) {
      this.wsSend('session:upgrade', { email: data.email || '', phone: data.phone || '', source: 'widget_prechat' });
    }

    // Inject a confirmation message so the user knows email was recorded + AI is ready
    if (data.email) {
      const confirmationMsg: Message = {
        id: `prechat-confirm-${Date.now()}`,
        conversationId: this.activeConversationId || '',
        role: 'ai',
        content: `Thanks! Our AI assistant is here to help you. If needed, a team member can also follow up with you at **${data.email}**.`,
        isInternal: false,
        createdAt: new Date().toISOString(),
      };
      this.messages = [...this.messages, confirmationMsg];
    }

    // Fire lead tracking event to events pipeline (ClickHouse)
    if (data.email && (globalThis as any).helpin?.track) {
      (globalThis as any).helpin.track('lead', { email: data.email });
    }

    this.render();
  }

  // ─── Image Lightbox (rendered in parent document, outside shadow DOM) ───

  private showImageLightbox(src: string, _alt: string): void {
    // Remove any existing lightbox
    this.dismissImageLightbox();

    const overlay = document.createElement('div');
    overlay.id = 'helpin-image-lightbox';
    overlay.style.cssText = `
      position: fixed; inset: 0; z-index: 2147483647;
      display: flex; align-items: center; justify-content: center;
      background: rgba(0, 0, 0, 0.85); backdrop-filter: blur(4px);
      cursor: zoom-out; animation: helpin-lb-fade-in 0.15s ease;
    `;

    const img = document.createElement('img');
    img.src = src;
    img.style.cssText = `
      max-width: 90vw; max-height: 90vh; object-fit: contain;
      border-radius: 8px; box-shadow: 0 8px 32px rgba(0,0,0,0.4);
      cursor: default;
    `;
    img.onclick = (e) => e.stopPropagation();

    const closeBtn = document.createElement('button');
    closeBtn.innerHTML = `<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="white" stroke-width="2" stroke-linecap="round"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>`;
    closeBtn.style.cssText = `
      position: absolute; top: 16px; right: 16px;
      width: 36px; height: 36px; border-radius: 50%;
      background: rgba(255,255,255,0.15); border: none;
      display: flex; align-items: center; justify-content: center;
      cursor: pointer; transition: background 0.15s;
    `;
    closeBtn.onmouseenter = () => { closeBtn.style.background = 'rgba(255,255,255,0.25)'; };
    closeBtn.onmouseleave = () => { closeBtn.style.background = 'rgba(255,255,255,0.15)'; };
    closeBtn.onclick = () => this.dismissImageLightbox();

    overlay.onclick = () => this.dismissImageLightbox();
    overlay.appendChild(img);
    overlay.appendChild(closeBtn);

    // Add fade-in animation
    const style = document.createElement('style');
    style.id = 'helpin-lb-style';
    style.textContent = `@keyframes helpin-lb-fade-in { from { opacity: 0; } to { opacity: 1; } }`;
    document.head.appendChild(style);

    document.body.appendChild(overlay);

    // Close on Escape key
    const handleEscape = (e: KeyboardEvent) => {
      if (e.key === 'Escape') this.dismissImageLightbox();
    };
    document.addEventListener('keydown', handleEscape);
    (overlay as any)._escHandler = handleEscape;
  }

  private dismissImageLightbox(): void {
    const existing = document.getElementById('helpin-image-lightbox');
    if (existing) {
      if ((existing as any)._escHandler) {
        document.removeEventListener('keydown', (existing as any)._escHandler);
      }
      existing.remove();
    }
    document.getElementById('helpin-lb-style')?.remove();
  }

  // ─── Escalation ────────────────────────────────────────────

  private handleEscalateToHuman(): void {
    if (!this.activeConversationId) return;

    if (this.wsConnection?.readyState === WebSocket.OPEN) {
      this.wsSend('conversation:escalate', {});
    } else {
      console.error('Failed to escalate to human: WebSocket not connected');
    }
  }

  // ─── Conversation Switching ─────────────────────────────────

  private handleSelectConversation(conversationId: string): void {
    this.activeConversationId = conversationId;
    this.activeTeammate = this.conversations.find((conversation) => conversation.id === conversationId)?.activeTeammate;

    // Request messages for this conversation via WS (also marks it read server-side)
    if (this.wsConnection?.readyState === WebSocket.OPEN) {
      this.wsSend('conversation:select', { conversation_id: conversationId });
    }

    // Clear local unread count for this conversation immediately (optimistic)
    const convIdx = this.conversations.findIndex(c => c.id === conversationId);
    if (convIdx >= 0 && this.conversations[convIdx].unreadCount) {
      const updated = { ...this.conversations[convIdx], unreadCount: 0 };
      this.conversations = this.conversations.map((c, i) => i === convIdx ? updated : c);
      this.syncUnreadCount();
    }
    this.clearTitleUnread(conversationId);

    // Clear current messages while loading
    this.messages = [];
    this.currentView = 'conversation';
    this.render();
  }

  private handleStartNewConversation(): void {
    this.resetActiveConversation();
    this.currentView = 'conversation';

    // Tell server to clear the session's conversation_id so next message creates a new one
    if (this.wsConnection?.readyState === WebSocket.OPEN) {
      this.wsSend('conversation:new', {});
    }

    this.render();
  }

  // ─── API / Session ─────────────────────────────────────────

  private async fetchWidgetConfig(): Promise<void> {
    if (!this.config?.widgetKey) return;

    // Try localStorage cache first
    const cached = getCachedConfig(this.config.widgetKey);
    if (cached) {
      this.widgetConfig = normalizeWidgetConfig(cached);
      this.ensureWidget();
      this.render();
      return;
    }

    try {
      const response = await fetch(
        `https://${this.host}/widget/config?widget_key=${encodeURIComponent(this.config.widgetKey)}`
      );

      if (!response.ok) {
        throw new Error(`Config fetch failed: ${response.status}`);
      }

      this.widgetConfig = normalizeWidgetConfig(await response.json());

      // Cache in localStorage
      if (this.widgetConfig && this.config.widgetKey) {
        cacheConfig(this.config.widgetKey, this.widgetConfig);
      }

      this.ensureWidget();
      this.render();
    } catch (error) {
      console.error('Failed to fetch widget config:', error);
    }
  }

  // ─── WebSocket (WS-first) ─────────────────────────────────

  private connectWebSocket(): void {
    if (this.isShutdown || !this.widgetKey) return;

    this.connectionStatus = 'connecting';
    this.render();

    try {
      // Connect with just widget_key (unauthenticated)
      this.wsConnection = new WebSocket(
        `wss://${this.host}/widget/ws?key=${encodeURIComponent(this.widgetKey)}`
      );

      this.wsConnection.onopen = () => {
        this.wsRetryCount = 0;
        this.wsHasConnected = true;
        this.connectionIssueStartedAt = null;
        this.connectionStatus = 'connected';
        this.render();

        // Send session:create or session:restore
        const storedSession = this.widgetKey ? getStoredSession(this.widgetKey) : null;
        if (storedSession) {
          this.wsSend('session:restore', { session_token: storedSession.session_token });
        } else {
          this.wsSend('session:create', {
            anonymous_id: this.anonymousId || '',
            page_url: typeof window !== 'undefined' ? window.location.href : '',
            page_title: typeof document !== 'undefined' ? document.title : '',
            user_agent: typeof navigator !== 'undefined' ? navigator.userAgent : '',
            timezone: Intl?.DateTimeFormat?.()?.resolvedOptions?.()?.timeZone || '',
            locale: typeof navigator !== 'undefined' ? navigator.language : '',
          });
        }
      };

      this.wsConnection.onmessage = (event) => {
        try {
          const data = JSON.parse(event.data);
          this.handleWSMessage(data);
        } catch {
          console.error('Failed to parse WebSocket message');
        }
      };

      this.wsConnection.onclose = (event) => {
        if (this.isShutdown) return;

        const now = Date.now();
        if (this.connectionIssueStartedAt === null) {
          this.connectionIssueStartedAt = now;
        }

        this.connectionStatus = 'disconnected';
        this.render();

        // Server rejected before WS upgrade (e.g. invalid widget key → HTTP 400).
        // Code 1006 = abnormal closure (no close frame received — typical for HTTP rejection).
        const maxRetries = this.wsHasConnected ? MAX_WS_RETRIES : MAX_WS_INITIAL_RETRIES;
        const reconnectWindowElapsed = this.wsHasConnected
          && this.connectionIssueStartedAt !== null
          && now - this.connectionIssueStartedAt >= MAX_AUTO_RECONNECT_WINDOW_MS;
        const shouldSurfaceFailure = this.wsRetryCount >= maxRetries || reconnectWindowElapsed;

        if (shouldSurfaceFailure) {
          if (!this.wsHasConnected) {
            console.error(
              `Helpin widget: failed to connect after ${MAX_WS_INITIAL_RETRIES} attempts. ` +
              'Continuing to retry in the background.'
            );
          } else if (reconnectWindowElapsed) {
            console.error('Helpin widget: reconnect window exceeded, continuing background retries');
          } else {
            console.error(`Helpin widget: lost connection after ${MAX_WS_RETRIES} retries, continuing background retries`);
          }
          this.connectionStatus = 'failed';
          this.render();
        }

        const delayCap = shouldSurfaceFailure ? MAX_BACKGROUND_RETRY_DELAY_MS : WS_MAX_DELAY_MS;
        const delay = Math.min(
          WS_BASE_DELAY_MS * Math.pow(2, this.wsRetryCount) + Math.random() * 1000,
          delayCap
        );
        this.wsRetryCount++;

        if (this.wsRetryTimer) {
          clearTimeout(this.wsRetryTimer);
        }
        this.wsRetryTimer = setTimeout(() => {
          this.wsRetryTimer = null;
          this.connectWebSocket();
        }, delay);
      };

      this.wsConnection.onerror = (event) => {
        console.error('WebSocket error:', event);
      };
    } catch (error) {
      console.error('WebSocket connection failed:', error);
      this.connectionStatus = 'failed';
      this.render();
    }
  }

  /** Map snake_case attachment payloads from the API to camelCase Attachment objects. */
  private static mapAttachments(raw: any[] | undefined): any[] | undefined {
    if (!raw || !Array.isArray(raw) || raw.length === 0) return undefined;
    return raw.map((a: any) => ({
      id: a.id,
      fileKey: a.file_key,
      fileName: a.file_name,
      fileType: a.file_type,
      fileSize: a.file_size,
      url: a.url,
    }));
  }

  private handleWSMessage(data: { type: string; data?: any }): void {
    switch (data.type) {
      case 'session:joined': {
        const payload = data.data;
        this.sessionToken = payload.session_token;

        // Persist session to localStorage
        if (this.widgetKey && payload.session_token && payload.expires_at) {
          persistSession(this.widgetKey, payload.session_token, payload.expires_at);
        }

        // Load conversations list from server
        if (payload.conversations && payload.conversations.length > 0) {
          this.conversations = payload.conversations.map((c: any) => this.mapConversation(c));
        }

        // Set active conversation from messages (if session has one)
        if (payload.messages && payload.messages.length > 0 && payload.messages[0].conversation_id) {
          this.activeConversationId = payload.messages[0].conversation_id;
          this.activeTeammate = this.mapActiveTeammate(payload.active_teammate)
            || this.conversations.find((c) => c.id === this.activeConversationId)?.activeTeammate;
          // Auto-navigate to the active conversation so the user resumes where they left off
          this.currentView = 'conversation';
        }

        // Load conversation history from server
        if (payload.messages && payload.messages.length > 0) {
          this.messages = payload.messages.map((m: any) => this.mapSupportMessage(m));
        }

        const hashConversationId = this.getConversationIdFromHash();
        if (hashConversationId && this.conversations.some((c) => c.id === hashConversationId)) {
          this.activeConversationId = hashConversationId;
          this.currentView = 'conversation';
          this.isVisible = true;
          this.isOpen = true;
          if (this.wsConnection?.readyState === WebSocket.OPEN) {
            this.wsSend('conversation:select', { conversation_id: hashConversationId });
          }
        }

        this.connectionStatus = 'connected';
        this.syncUnreadCount();

        // Start keepalive ping every 60s to refresh server-side visitor online keys.
        if (this.keepaliveTimer) {
          clearInterval(this.keepaliveTimer);
        }
        this.keepaliveTimer = setInterval(() => {
          if (this.wsConnection?.readyState === WebSocket.OPEN) {
            this.wsSend('ping', {});
          }
        }, 60_000);

        // If session was restored as identified, store email to skip pre-chat
        if (!payload.is_anonymous && payload.customer_email) {
          this.currentEmail = payload.customer_email;
        }

        // If session is anonymous, try to auto-upgrade from boot config or stored identity
        if (payload.is_anonymous) {
          const bootEmail = this.config?.user?.email;
          const storedIdentity = this.widgetKey ? getStoredIdentity(this.widgetKey) : null;
          const email = bootEmail || storedIdentity?.email;
          const bootUser = this.config?.user;
          const firstName = (bootEmail ? (bootUser?.firstName || bootUser?.first_name) : storedIdentity?.firstName) || '';
          const lastName = (bootEmail ? (bootUser?.lastName || bootUser?.last_name) : storedIdentity?.lastName) || '';
          const name = buildWidgetUserDisplayName(
            firstName,
            lastName,
            bootEmail ? bootUser?.name : storedIdentity?.name,
          );

          if (email) {
            this.wsSend('session:upgrade', {
              email,
              name,
              first_name: firstName,
              last_name: lastName,
              source: bootEmail ? 'sdk_identify' : 'stored_identity',
            });
            this.currentEmail = email;
          }
        }

        this.render();
        break;
      }

      case 'session:error': {
        // Token invalid — clear and retry with session:create
        if (this.widgetKey) {
          clearSession(this.widgetKey);
        }
        this.sessionToken = null;

        // Send session:create as retry on same connection
        this.wsSend('session:create', {
          anonymous_id: this.anonymousId || '',
          page_url: typeof window !== 'undefined' ? window.location.href : '',
          page_title: typeof document !== 'undefined' ? document.title : '',
          user_agent: typeof navigator !== 'undefined' ? navigator.userAgent : '',
          timezone: Intl?.DateTimeFormat?.()?.resolvedOptions?.()?.timeZone || '',
          locale: typeof navigator !== 'undefined' ? navigator.language : '',
        });
        break;
      }

      case 'session:upgraded':
        this.render();
        break;

      case 'session:revoked':
        // Server confirmed revoke — cleanup handled by shutdown()
        break;

      case 'message:received': {
        const msg = data.data;
        const newMsg = this.mapSupportMessage(msg);

        // Replace optimistic message if this is an echo
        if (msg.sender_type === 'customer') {
          const tempIdx = this.messages.findIndex(
            (m) => m.id.startsWith('temp-') && m.content === msg.content
          );
          if (tempIdx >= 0) {
            this.messages[tempIdx] = newMsg;
            this.messages = [...this.messages];
          } else {
            this.messages = [...this.messages, newMsg];
          }
        } else {
          this.messages = [...this.messages, newMsg];
        }

        if (msg.sender_type !== 'customer') {
          this.isTyping = false;
          this.playReceivedMessageSound();
        }

        // Update conversation in the list (lastMessage preview + unread count + move to top)
        if (newMsg.conversationId) {
          const convIdx = this.conversations.findIndex(c => c.id === newMsg.conversationId);
          const isActiveAndOpen = this.isConversationVisibleToUser(newMsg.conversationId);
          const nextUnreadCount = msg.sender_type !== 'customer' && !isActiveAndOpen ? 1 : 0;
          if (msg.sender_type !== 'customer' && newMsg.role !== 'system' && !isActiveAndOpen) {
            this.incrementTitleUnread(newMsg.conversationId);
          }
          if (convIdx >= 0) {
            const prev = this.conversations[convIdx];
            const updated = {
              ...prev,
              lastMessage: newMsg.content,
              lastMessageAt: newMsg.createdAt,
              unreadCount: (msg.sender_type !== 'customer' && !isActiveAndOpen)
                ? (prev.unreadCount ?? 0) + 1
                : (prev.unreadCount ?? 0),
            };
            this.conversations = [updated, ...this.conversations.filter((_, i) => i !== convIdx)];
          } else {
            this.conversations = [{
              id: newMsg.conversationId,
              subject: newMsg.content || 'Conversation',
              status: 'open',
              lastMessage: newMsg.content,
              lastMessageAt: newMsg.createdAt,
              unreadCount: nextUnreadCount,
            }, ...this.conversations];
          }

          if (this.activeConversationId === newMsg.conversationId && this.conversations.length > 0) {
            this.activeTeammate = this.conversations.find((conversation) => conversation.id === newMsg.conversationId)?.activeTeammate;
          }

          if (msg.sender_type !== 'customer' && isActiveAndOpen) {
            this.markConversationRead(newMsg.conversationId);
          }
        }

        this.syncUnreadCount();

        this.triggerCallback('onMessageReceived', msg);
        this.render();
        break;
      }

      case 'conversation:created': {
        const convId = data.data?.conversation_id;
        if (convId) {
          this.activeConversationId = convId;
          this.activeTeammate = undefined;
          // Add new conversation to the list with real server ID
          if (!this.conversations.some(c => c.id === convId)) {
            const lastCustomerMsg = [...this.messages].reverse().find(m => m.role === 'customer');
            const preview = lastCustomerMsg?.content;
            this.conversations = [{
              id: convId,
              subject: preview ? (preview.length > 100 ? preview.slice(0, 100) + '...' : preview) : 'New conversation',
              status: 'open',
              lastMessage: preview,
              lastMessageAt: new Date().toISOString(),
            }, ...this.conversations];
          }
          this.triggerCallback('onConversationStarted', convId);
          this.render();
        }
        break;
      }

      case 'typing:start': {
        // Hub already filters out widget's own typing — this is always agent-origin
        this.isTyping = true;
        const typingData = data.data;
        if (typingData) {
          this.typingAgentName = typingData.agent_name || undefined;
          this.typingAgentAvatar = typingData.agent_avatar || undefined;
        }
        this.render();
        break;
      }

      case 'typing:stop':
        this.isTyping = false;
        this.typingAgentName = undefined;
        this.typingAgentAvatar = undefined;
        this.render();
        break;

      case 'ai:thinking:start':
        this.isAIThinking = true;
        this.render();
        break;

      case 'ai:thinking:stop':
        this.isAIThinking = false;
        this.render();
        break;

      case 'conversations:listed': {
        const convs = data.data?.conversations;
        if (Array.isArray(convs)) {
          this.conversations = convs.map((c: any) => {
            const conversation = this.mapConversation(c);
            return {
              ...conversation,
              unreadCount: (this.isOpen && this.currentView === 'conversation' && this.activeConversationId === c.id)
                ? 0
                : conversation.unreadCount,
            };
          });
          if (this.activeConversationId) {
            this.activeTeammate = this.conversations.find((c) => c.id === this.activeConversationId)?.activeTeammate;
          }
          this.syncUnreadCount();
          this.render();
        }
        break;
      }

      case 'conversation:messages': {
        const msgs = data.data?.messages;
        if (Array.isArray(msgs)) {
          this.activeTeammate = this.mapActiveTeammate(data.data?.active_teammate)
            || (this.activeConversationId ? this.conversations.find((c) => c.id === this.activeConversationId)?.activeTeammate : undefined);
          this.messages = msgs.map((m: any) => this.mapSupportMessage(m));
          this.render();
        }
        break;
      }

      case 'conversation:escalated': {
        this.activeTeammate = this.mapActiveTeammate(data.data?.active_teammate) || this.activeTeammate;
        this.render();
        break;
      }

      case 'teammate:presence': {
        const userId = typeof data.data?.user_id === 'string' ? data.data.user_id : '';
        const status = data.data?.status;
        if (!userId || (status !== 'online' && status !== 'away' && status !== 'offline')) {
          break;
        }

        const configChanged = this.updateAvailableTeammateStatus(userId, status);

        this.conversations = this.conversations.map((conversation) => {
          const teammate = conversation.activeTeammate;
          if (!teammate || teammate.userId !== userId) {
            return conversation;
          }
          return {
            ...conversation,
            activeTeammate: {
              userId: teammate.userId,
              name: teammate.name,
              avatarUrl: teammate.avatarUrl,
              status,
            },
          };
        });

        let shouldRender = configChanged;
        const activeTeammate = this.activeTeammate;
        if (activeTeammate && activeTeammate.userId === userId) {
          this.activeTeammate = {
            userId: activeTeammate.userId,
            name: activeTeammate.name,
            avatarUrl: activeTeammate.avatarUrl,
            status,
          };
          shouldRender = true;
        }

        if (shouldRender) {
          this.render();
        }
        break;
      }

      case 'pong':
        // Server acknowledged keepalive ping — no action needed.
        break;

      case 'config:updated': {
        // Admin changed widget settings — apply new config in real time
        const newConfig = data.data;
        if (newConfig && typeof newConfig === 'object') {
          this.widgetConfig = normalizeWidgetConfig(newConfig);
          // Update localStorage cache with fresh config
          if (this.widgetKey) {
            cacheConfig(this.widgetKey, this.widgetConfig);
          }
          this.render();
        }
        break;
      }

      case 'connection:error':
        console.error('Widget server error:', data.data);
        break;
    }
  }

  reconnectWebSocket(): void {
    if (this.wsRetryTimer) {
      clearTimeout(this.wsRetryTimer);
      this.wsRetryTimer = null;
    }
    this.wsRetryCount = 0;
    this.connectionIssueStartedAt = null;
    this.connectionStatus = 'idle';
    this.disconnectWebSocket();
    this.connectWebSocket();
  }

  /**
   * Sends a session:upgrade message over WebSocket if connected.
   * Used by the analytics client to upgrade anonymous sessions via the
   * identify() / lead() SDK methods without going through the HTTP fallback.
   * Returns true if the message was sent, false if WS is not open.
   */
  public sendSessionUpgrade(email: string, name: string, source: string, firstName: string = '', lastName: string = ''): boolean {
    if (this.wsConnection?.readyState === WebSocket.OPEN) {
      this.wsSend('session:upgrade', {
        email,
        name,
        first_name: firstName,
        last_name: lastName,
        source,
      });
      // Persist identity so it survives page refresh
      if (this.widgetKey && email) {
        persistIdentity(this.widgetKey, email, name, firstName, lastName);
      }
      return true;
    }
    return false;
  }

  private wsSend(type: string, data: Record<string, any>): void {
    if (this.wsConnection?.readyState === WebSocket.OPEN) {
      this.wsConnection.send(JSON.stringify({ type, data }));
    }
  }

  private disconnectWebSocket(): void {
    if (this.keepaliveTimer) {
      clearInterval(this.keepaliveTimer);
      this.keepaliveTimer = null;
    }
    if (this.wsConnection) {
      this.wsConnection.onclose = null;
      this.wsConnection.close();
      this.wsConnection = null;
    }
  }

  // Returns '#ffffff' or '#000000' based on which has better contrast against the given hex color.
  private getContrastColor(hex: string): string {
    const rgb = this.hexToRgb(hex);
    if (!rgb) return '#ffffff';
    // Relative luminance (WCAG formula)
    const luminance = (0.299 * rgb.r + 0.587 * rgb.g + 0.114 * rgb.b) / 255;
    return luminance > 0.5 ? '#000000' : '#ffffff';
  }

  // Darkens a hex color by a percentage (0-100).
  private darkenColor(hex: string, percent: number): string {
    const rgb = this.hexToRgb(hex);
    if (!rgb) return hex;
    const factor = 1 - percent / 100;
    const r = Math.round(rgb.r * factor);
    const g = Math.round(rgb.g * factor);
    const b = Math.round(rgb.b * factor);
    return `#${r.toString(16).padStart(2, '0')}${g.toString(16).padStart(2, '0')}${b.toString(16).padStart(2, '0')}`;
  }

  private hexToRgb(hex: string): { r: number; g: number; b: number } | null {
    const match = hex.replace('#', '').match(/^([0-9a-f]{2})([0-9a-f]{2})([0-9a-f]{2})$/i);
    if (!match) return null;
    return { r: parseInt(match[1], 16), g: parseInt(match[2], 16), b: parseInt(match[3], 16) };
  }

  private triggerCallback(name: string, ...args: any[]): void {
    this.callbacks[name]?.forEach((cb) => cb(...args));
  }
}
