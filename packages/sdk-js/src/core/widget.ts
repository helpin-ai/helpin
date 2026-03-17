import { mountWidget, unmountWidget } from '@helpin/widget-core';
import type { WidgetConfig, Message, Conversation, MountWidgetOptions, WidgetView } from '@helpin/widget-core';
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

type WidgetCallback = (...args: any[]) => void;
type ConnectionStatus = 'idle' | 'connecting' | 'connected' | 'disconnected' | 'failed';

const MAX_WS_RETRIES = 10;
const WS_BASE_DELAY_MS = 1000;
const WS_MAX_DELAY_MS = 30000;

export class WidgetManager {
  private config: WidgetSettings | null = null;
  private widgetConfig: WidgetConfig | null = null;
  private isOpen = false;
  private unreadCount = 0;
  private sessionToken: string | null = null;
  private wsConnection: WebSocket | null = null;
  private wsRetryCount = 0;
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
  private isTyping = false;
  private currentEmail: string | null = null;

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
    this.unreadCount = 0;
    if (!this.hasBeenOpened) {
      this.hasBeenOpened = true;
      this.currentView = 'conversation';
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

  showArticle(articleId: string): void {
    this.currentView = 'help';
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

    // Apply brand color as CSS variables on the shadow root container
    const primaryColor = this.widgetConfig.branding?.primaryColor;
    if (primaryColor && this.mountContainer.parentElement) {
      const root = this.mountContainer.parentElement as HTMLElement;
      root.style.setProperty('--helpin-primary', primaryColor);
      root.style.setProperty('--helpin-primary-hover', this.darkenColor(primaryColor, 15));
      // Auto-detect foreground color for readability on both light and dark brand colors
      root.style.setProperty('--helpin-primary-foreground', this.getContrastColor(primaryColor));
    }

    const showPreChat = this.widgetConfig.features?.preChatForm && !this.sessionToken;

    mountWidget(this.mountContainer, {
      config: this.widgetConfig,
      messages: this.messages,
      isOpen: this.isOpen,
      onClose: () => this.hide(),
      onSendMessage: (content: string) => this.handleSendMessage(content),
      onSendMessageFromHome: (content: string) => this.handleSendMessage(content, { startNewConversation: true }),
      onQuickReply: (content: string) => this.handleSendMessage(content),
      onTyping: (content: string) => this.handleTyping(content),
      showPreChatForm: showPreChat,
      onPreChatSubmit: (data: { name: string; email: string }) => this.handlePreChatSubmit(data),
      isTyping: this.isTyping,
      initialView: this.currentView,
      showLauncher: true,
      onLauncherClick: () => this.toggle(),
      unreadCount: this.unreadCount,
      connectionStatus: this.connectionStatus,
      conversations: this.conversations,
      onSelectConversation: (id: string) => this.handleSelectConversation(id),
      onStartNewConversation: () => this.handleStartNewConversation(),
      onViewChange: (view: WidgetView) => {
        this.currentView = view;
        // Refresh conversations list from server when navigating to Messages tab
        if (view === 'messages' && this.wsConnection?.readyState === WebSocket.OPEN) {
          this.wsSend('conversations:list', {});
        }
      },
    });
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

  private handlePreChatSubmit(data: { name: string; email: string }): void {
    this.triggerCallback('onUserEmailSupplied', data.email);
    this.currentEmail = data.email;

    // Upgrade session via WS
    if (this.wsConnection?.readyState === WebSocket.OPEN) {
      this.wsSend('session:upgrade', { email: data.email, name: data.name });
    }
  }

  // ─── Conversation Switching ─────────────────────────────────

  private handleSelectConversation(conversationId: string): void {
    this.activeConversationId = conversationId;

    // Request messages for this conversation via WS
    if (this.wsConnection?.readyState === WebSocket.OPEN) {
      this.wsSend('conversation:select', { conversation_id: conversationId });
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

      this.wsConnection.onclose = () => {
        if (this.isShutdown) return;

        this.connectionStatus = 'disconnected';
        this.render();

        if (this.wsRetryCount >= MAX_WS_RETRIES) {
          console.error(`WebSocket: gave up after ${MAX_WS_RETRIES} retries`);
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
          }));
        }

        // Set active conversation from messages (if session has one)
        if (payload.messages && payload.messages.length > 0 && payload.messages[0].conversation_id) {
          this.activeConversationId = payload.messages[0].conversation_id;
        }

        // Load conversation history from server
        if (payload.messages && payload.messages.length > 0) {
          this.messages = payload.messages.map((m: any) => ({
            id: m.id,
            conversationId: m.conversation_id,
            role: m.sender_type === 'customer' ? 'customer' : m.sender_type === 'ai' ? 'ai' : 'agent',
            content: m.content,
            isInternal: m.is_internal || false,
            createdAt: m.created_at,
          }));
        }

        this.connectionStatus = 'connected';

        // Start keepalive ping every 60s to refresh server-side visitor online keys.
        if (this.keepaliveTimer) {
          clearInterval(this.keepaliveTimer);
        }
        this.keepaliveTimer = setInterval(() => {
          if (this.wsConnection?.readyState === WebSocket.OPEN) {
            this.wsSend('ping', {});
          }
        }, 60_000);

        // If user data was provided at boot, upgrade the session
        if (this.config?.user?.email && payload.is_anonymous) {
          this.wsSend('session:upgrade', {
            email: this.config.user.email,
            name: this.config.user.name || '',
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
          isInternal: false,
          createdAt: msg.created_at || new Date().toISOString(),
        };

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
        }

        if (!this.isOpen) {
          this.unreadCount++;
          this.triggerCallback('onUnreadCountChange', this.unreadCount);
        }

        // Update conversation in the list (lastMessage preview + move to top)
        if (newMsg.conversationId) {
          const convIdx = this.conversations.findIndex(c => c.id === newMsg.conversationId);
          if (convIdx >= 0) {
            const updated = {
              ...this.conversations[convIdx],
              lastMessage: newMsg.content,
              lastMessageAt: newMsg.createdAt,
            };
            this.conversations = [updated, ...this.conversations.filter((_, i) => i !== convIdx)];
          }
        }

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

      case 'typing:start':
        // Hub already filters out widget's own typing — this is always agent-origin
        this.isTyping = true;
        this.render();
        break;

      case 'typing:stop':
        this.isTyping = false;
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
          }));
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
