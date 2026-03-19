import { mountWidget, unmountWidget } from '@helpin/widget-core';
import type { WidgetConfig, Message, Conversation, WidgetView } from '@helpin/widget-core';
// @ts-ignore — Vite ?inline import returns CSS as a string
import widgetStyles from '@helpin/widget-core/styles?inline';
import { isBot } from '../utils/bot-detect';
import {
  getOrCreateAnonymousId,
  getStoredSession,
  persistSession,
  clearSession,
  clearConfigCache,
  getCachedConfig,
  cacheConfig,
} from './identity';

export { type WidgetConfig };

export interface WidgetUser {
  email?: string;
  name?: string;
  userId?: string;
  createdAt?: string;
  metadata?: Record<string, unknown>;
}

export interface WidgetSettings {
  key: string;
  host?: string;
  user?: WidgetUser;
}

export interface ShowArticleOptions {
  collectionId?: string;
  spaceId?: string;
}

type WidgetCallback = (...args: any[]) => void;
type ConnectionStatus = 'idle' | 'connecting' | 'connected' | 'disconnected' | 'failed';

const MAX_WS_RETRIES = 10;
const MAX_WS_INITIAL_RETRIES = 3; // retries before first successful connection (invalid key, server down)
const WS_BASE_DELAY_MS = 1000;
const WS_MAX_DELAY_MS = 30000;
const NOTIFICATION_SOUND_URL = 'https://cdn.helpin.ai/sounds/ping.mp3';

export class WidgetManager {
  private config: WidgetSettings | null = null;
  private widgetConfig: WidgetConfig | null = null;
  private isOpen = false;
  private unreadCount = 0;
  private sessionToken: string | null = null;
  private wsConnection: WebSocket | null = null;
  private wsRetryCount = 0;
  private wsHasConnected = false;
  private wsRetryTimer: ReturnType<typeof setTimeout> | null = null;
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
  private conversations: Conversation[] = [];
  private activeConversationId: string | null = null;
  private currentView: WidgetView = 'home';
  private openArticleRequest: { key: number; articleSlug: string } | null = null;
  private articleRequestKey = 0;
  private isTyping = false;
  private isAIThinking = false;
  private typingAgentName: string | undefined;
  private typingAgentAvatar: string | undefined;
  private currentEmail: string | null = null;
  private notificationAudio: HTMLAudioElement | null = null;

  private callbacks: Record<string, WidgetCallback[]> = {
    onShow: [],
    onHide: [],
    onUnreadCountChange: [],
    onUserEmailSupplied: [],
    onConversationStarted: [],
    onMessageReceived: [],
  };

  boot(settings: WidgetSettings): void {
    // Bot/crawler filtering
    if (isBot()) return;

    // Clean up previous boot if any
    if (this.config) {
      this.cleanup();
    }

    this.isShutdown = false;
    this.config = settings;
    this.widgetKey = settings.key;

    if (settings.host) {
      this.host = settings.host.replace(/^https?:\/\//, '');
    }

    // Get or create anonymous ID from cookie
    this.anonymousId = getOrCreateAnonymousId(settings.key);

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

    // 2. Clear persisted session (but NOT anonymous_id cookie)
    if (this.widgetKey) {
      clearSession(this.widgetKey);
      clearConfigCache(this.widgetKey);
    }

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
    this.config = null;
    this.widgetConfig = null;
    this.sessionToken = null;
    this.isOpen = false;
    this.unreadCount = 0;
    this.hasBeenOpened = false;
    this.wsRetryCount = 0;
    this.messages = [];
    this.conversations = [];
    this.activeConversationId = null;
    this.currentView = 'home';
    this.openArticleRequest = null;
    this.articleRequestKey = 0;
    this.isTyping = false;
    this.connectionStatus = 'idle';
    this.currentEmail = null;

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
    this.isOpen = true;
    this.unlockNotificationSound();
    if (!this.hasBeenOpened && this.currentView === 'home') {
      this.hasBeenOpened = true;
      // If there's an active conversation (restored session), resume it;
      // otherwise start fresh in conversation view.
      this.currentView = 'conversation';
    } else {
      this.hasBeenOpened = true;
    }
    this.ensureWidget();
    this.render();
    this.triggerCallback('onShow');
  }

  hide(): void {
    this.isOpen = false;
    this.render();
    this.triggerCallback('onHide');
  }

  toggle(): void {
    if (this.isOpen) {
      this.hide();
    } else {
      this.show();
    }
  }

  showMessages(): void {
    this.currentView = 'messages';
    this.show();
  }

  showNewMessage(content?: string): void {
    this.resetActiveConversation();
    this.currentView = 'conversation';

    // Tell server to clear active conversation so next message creates a new one
    if (this.wsConnection?.readyState === WebSocket.OPEN) {
      this.wsSend('conversation:new', {});
    }

    this.show();

    // If content provided, send it as the first message
    if (content?.trim()) {
      this.handleSendMessage(content);
    }
  }

  showConversation(conversationId: string): void {
    this.currentView = 'home';
    this.show();
  }

  showArticle(articleId: string, _options?: ShowArticleOptions): void {
    this.articleRequestKey += 1;
    this.openArticleRequest = {
      key: this.articleRequestKey,
      articleSlug: articleId,
    };
    this.currentView = 'help-article';
    this.show();
  }

  onShow(callback: WidgetCallback): void {
    this.callbacks.onShow.push(callback);
  }

  onHide(callback: WidgetCallback): void {
    this.callbacks.onHide.push(callback);
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

    // Show pre-chat form only when: feature is enabled AND session is anonymous (no email yet)
    const alreadyIdentified = !!this.currentEmail || !!this.config?.user?.email;
    const showPreChat = !!this.widgetConfig.features?.preChatForm && !alreadyIdentified;

    const mountOptions: Parameters<typeof mountWidget>[1] & {
      openArticleRequest?: {
        key: number;
        articleSlug: string;
      };
    } = {
      config: this.widgetConfig,
      messages: this.messages,
      isOpen: this.isOpen,
      onClose: () => this.hide(),
      onSendMessage: (content: string) => this.handleSendMessage(content),
      onSendMessageFromHome: (content: string) => this.handleSendMessage(content, { startNewConversation: true }),
      onQuickReply: (content: string) => this.handleSendMessage(content),
      onTyping: (content: string) => this.handleTyping(content),
      showPreChatForm: showPreChat,
      onPreChatSubmit: (data: { phone: string; email: string }) => this.handlePreChatSubmit(data),
      isTyping: this.isTyping,
      isAIThinking: this.isAIThinking,
      onEscalateToHuman: () => this.handleEscalateToHuman(),
      typingAgentName: this.typingAgentName,
      typingAgentAvatar: this.typingAgentAvatar,
      initialView: this.currentView,
      showLauncher: true,
      onLauncherClick: () => this.toggle(),
      unreadCount: this.unreadCount,
      connectionStatus: this.connectionStatus,
      conversations: this.conversations,
      onSelectConversation: (id: string) => this.handleSelectConversation(id),
      onStartNewConversation: () => this.handleStartNewConversation(),
      widgetKey: this.widgetKey || undefined,
      host: this.host,
      openArticleRequest: this.openArticleRequest || undefined,
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
  private unlockNotificationSound(): void {
    try {
      if (!this.notificationAudio) {
        this.notificationAudio = new Audio(NOTIFICATION_SOUND_URL);
      }
      // Silent play to unlock autoplay policy, then reset
      this.notificationAudio.volume = 0;
      this.notificationAudio.play().then(() => {
        this.notificationAudio!.pause();
        this.notificationAudio!.currentTime = 0;
        this.notificationAudio!.volume = 0.5;
      }).catch(() => {/* ignore */});
    } catch { /* audio not supported */ }
  }

  private playNotificationSound(): void {
    try {
      if (!this.notificationAudio) {
        this.notificationAudio = new Audio(NOTIFICATION_SOUND_URL);
        this.notificationAudio.volume = 0.5;
      }
      this.notificationAudio.currentTime = 0;
      this.notificationAudio.play().catch(() => {/* autoplay blocked — ignore */});
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
  }

  private clearActiveConversationUnread(): void {
    if (!this.activeConversationId) return;
    const convIdx = this.conversations.findIndex(c => c.id === this.activeConversationId);
    if (convIdx >= 0 && this.conversations[convIdx].unreadCount) {
      const updated = { ...this.conversations[convIdx], unreadCount: 0 };
      this.conversations = this.conversations.map((c, i) => i === convIdx ? updated : c);
      this.syncUnreadCount();
    }
  }

  // ─── Message Handling ──────────────────────────────────────

  private resetActiveConversation(): void {
    this.activeConversationId = null;
    this.messages = [];
    this.isTyping = false;
  }

  private handleSendMessage(content: string, options: { startNewConversation?: boolean } = {}): void {
    if (!content.trim()) return;

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

    // Stop typing indicator before sending
    this.stopTyping();

    // Send via WS
    if (this.wsConnection?.readyState === WebSocket.OPEN) {
      this.wsSend('message:send', { content });
    } else {
      // Fallback to HTTP if WS not available
      this.sendMessageHTTP(content);
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
    this.triggerCallback('onUserEmailSupplied', data.email);
    this.currentEmail = data.email;

    // Upgrade session via WS with source=prechat
    if (this.wsConnection?.readyState === WebSocket.OPEN) {
      this.wsSend('session:upgrade', { email: data.email, phone: data.phone, source: 'widget_prechat' });
    }

    // Fire lead tracking event to events pipeline (ClickHouse)
    if ((globalThis as any).helpin?.track) {
      (globalThis as any).helpin.track('lead', { email: data.email });
    }
  }

  // ─── Escalation ────────────────────────────────────────────

  private handleEscalateToHuman(): void {
    if (!this.activeConversationId || !this.sessionToken) return;

    const url = `https://${this.host}/api/widget/support/${this.activeConversationId}/escalate`;
    fetch(url, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'X-Session-Token': this.sessionToken },
    }).catch((err) => {
      console.error('Failed to escalate to human:', err);
    });
  }

  // ─── Conversation Switching ─────────────────────────────────

  private handleSelectConversation(conversationId: string): void {
    this.activeConversationId = conversationId;

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
    if (!this.config?.key) return;

    // Try localStorage cache first
    const cached = getCachedConfig(this.config.key);
    if (cached) {
      this.widgetConfig = cached;
      this.ensureWidget();
      this.render();
      return;
    }

    try {
      const response = await fetch(
        `https://${this.host}/widget/config?widget_key=${encodeURIComponent(this.config.key)}`
      );

      if (!response.ok) {
        throw new Error(`Config fetch failed: ${response.status}`);
      }

      this.widgetConfig = await response.json();

      // Cache in localStorage
      if (this.widgetConfig && this.config.key) {
        cacheConfig(this.config.key, this.widgetConfig);
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
        this.connectionStatus = 'connected';

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

        this.connectionStatus = 'disconnected';
        this.render();

        // Server rejected before WS upgrade (e.g. invalid widget key → HTTP 400).
        // Code 1006 = abnormal closure (no close frame received — typical for HTTP rejection).
        const maxRetries = this.wsHasConnected ? MAX_WS_RETRIES : MAX_WS_INITIAL_RETRIES;

        if (this.wsRetryCount >= maxRetries) {
          if (!this.wsHasConnected) {
            console.error(
              `Helpin widget: failed to connect after ${MAX_WS_INITIAL_RETRIES} attempts. ` +
              'Please verify your widget key is correct and the server is reachable.'
            );
          } else {
            console.error(`Helpin widget: lost connection, gave up after ${MAX_WS_RETRIES} retries`);
          }
          this.connectionStatus = 'failed';
          this.render();
          return;
        }

        const delay = Math.min(
          WS_BASE_DELAY_MS * Math.pow(2, this.wsRetryCount) + Math.random() * 1000,
          WS_MAX_DELAY_MS
        );
        this.wsRetryCount++;

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
          this.conversations = payload.conversations.map((c: any) => ({
            id: c.id,
            subject: c.subject || 'Untitled',
            status: c.status || 'open',
            lastMessage: c.last_message,
            lastMessageAt: c.updated_at || c.created_at,
            unreadCount: c.unread_count ?? 0,
          }));
        }

        // Set active conversation from messages (if session has one)
        if (payload.messages && payload.messages.length > 0 && payload.messages[0].conversation_id) {
          this.activeConversationId = payload.messages[0].conversation_id;
          // Auto-navigate to the active conversation so the user resumes where they left off
          this.currentView = 'conversation';
        }

        // Load conversation history from server
        if (payload.messages && payload.messages.length > 0) {
          this.messages = payload.messages.map((m: any) => {
            const msg: any = {
              id: m.id,
              conversationId: m.conversation_id,
              role: m.sender_type === 'customer' ? 'customer' : m.sender_type === 'ai' ? 'ai' : 'agent',
              content: m.content,
              senderName: m.sender_display_name || undefined,
              senderAvatar: m.sender_avatar_url || undefined,
              isInternal: m.is_internal || false,
              createdAt: m.created_at,
            };
            // Map AI metadata to widget Message fields
            if (m.metadata) {
              try {
                const meta = typeof m.metadata === 'string' ? JSON.parse(m.metadata) : m.metadata;
                if (meta.ai_sources) msg.sources = meta.ai_sources;
                if (meta.ai_confidence !== undefined) msg.aiConfidence = meta.ai_confidence;
              } catch { /* ignore parse errors */ }
            }
            return msg;
          });
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

        // If user data was provided at boot, upgrade the session (source=identify for SDK)
        if (this.config?.user?.email && payload.is_anonymous) {
          this.wsSend('session:upgrade', {
            email: this.config.user.email,
            name: this.config.user.name || '',
            source: 'sdk_identify',
          });
          this.currentEmail = this.config.user.email;
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
        const newMsg: Message = {
          id: msg.id || `ws-${Date.now()}`,
          conversationId: msg.conversation_id || '',
          role: msg.sender_type === 'customer' ? 'customer' : msg.sender_type === 'ai' ? 'ai' : 'agent',
          content: msg.content || '',
          senderName: msg.sender_name || undefined,
          senderAvatar: msg.sender_avatar || undefined,
          isInternal: false,
          createdAt: msg.created_at || new Date().toISOString(),
        };
        // Map AI metadata from WS payload
        if (msg.metadata) {
          try {
            const meta = typeof msg.metadata === 'string' ? JSON.parse(msg.metadata) : msg.metadata;
            if (meta.ai_sources) (newMsg as any).sources = meta.ai_sources;
            if (meta.ai_confidence !== undefined) (newMsg as any).aiConfidence = meta.ai_confidence;
          } catch { /* ignore parse errors */ }
        }

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
          this.playNotificationSound();
        }

        // Update conversation in the list (lastMessage preview + unread count + move to top)
        if (newMsg.conversationId) {
          const convIdx = this.conversations.findIndex(c => c.id === newMsg.conversationId);
          const isActiveAndOpen = this.isOpen && this.currentView === 'conversation' && this.activeConversationId === newMsg.conversationId;
          const nextUnreadCount = msg.sender_type !== 'customer' && !isActiveAndOpen ? 1 : 0;
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

          if (msg.sender_type !== 'customer' && isActiveAndOpen && this.wsConnection?.readyState === WebSocket.OPEN) {
            this.wsSend('conversation:read', { conversation_id: newMsg.conversationId });
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
          this.conversations = convs.map((c: any) => ({
            id: c.id,
            subject: c.subject || 'Untitled',
            status: c.status || 'open',
            lastMessage: c.last_message,
            lastMessageAt: c.updated_at || c.created_at,
            // Don't show unread badge for the conversation the user is actively viewing
            unreadCount: (this.isOpen && this.currentView === 'conversation' && this.activeConversationId === c.id)
              ? 0
              : (c.unread_count ?? 0),
          }));
          this.syncUnreadCount();
          this.render();
        }
        break;
      }

      case 'conversation:messages': {
        const msgs = data.data?.messages;
        if (Array.isArray(msgs)) {
          this.messages = msgs.map((m: any) => ({
            id: m.id,
            conversationId: m.conversation_id,
            role: m.sender_type === 'customer' ? 'customer' : m.sender_type === 'ai' ? 'ai' : 'agent',
            content: m.content,
            senderName: m.sender_display_name || undefined,
            senderAvatar: m.sender_avatar_url || undefined,
            isInternal: m.is_internal || false,
            createdAt: m.created_at,
          }));
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
          this.widgetConfig = newConfig;
          // Update localStorage cache with fresh config
          if (this.widgetKey) {
            cacheConfig(this.widgetKey, newConfig);
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
    this.wsRetryCount = 0;
    this.connectionStatus = 'idle';
    this.disconnectWebSocket();
    this.connectWebSocket();
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
