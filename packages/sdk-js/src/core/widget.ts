import { mountWidget, unmountWidget } from '@helpin/widget-core';
import type { WidgetConfig, Message, MountWidgetOptions, WidgetView } from '@helpin/widget-core';
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
    this.config = null;
    this.widgetConfig = null;
    this.sessionToken = null;
    this.isOpen = false;
    this.unreadCount = 0;
    this.hasBeenOpened = false;
    this.wsRetryCount = 0;
    this.messages = [];
    this.currentView = 'home';
    this.isTyping = false;
    this.connectionStatus = 'idle';
    this.currentEmail = null;

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
    this.currentView = 'home';
    this.show();
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

    const showPreChat = this.widgetConfig.features?.preChatForm && !this.sessionToken;

    mountWidget(this.mountContainer, {
      config: this.widgetConfig,
      messages: this.messages,
      isOpen: this.isOpen,
      onClose: () => this.hide(),
      onSendMessage: (content: string) => this.handleSendMessage(content),
      onQuickReply: (content: string) => this.handleSendMessage(content),
      showPreChatForm: showPreChat,
      onPreChatSubmit: (data: { name: string; email: string }) => this.handlePreChatSubmit(data),
      isTyping: this.isTyping,
      initialView: this.currentView,
      showLauncher: true,
      onLauncherClick: () => this.toggle(),
      unreadCount: this.unreadCount,
      connectionStatus: this.connectionStatus,
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

  private handleSendMessage(content: string): void {
    if (!content.trim()) return;

    // Optimistic update
    const optimisticMsg: Message = {
      id: `temp-${Date.now()}`,
      conversationId: '',
      role: 'customer',
      content,
      isInternal: false,
      createdAt: new Date().toISOString(),
    };
    this.messages = [...this.messages, optimisticMsg];
    this.render();

    // Send via WS
    if (this.wsConnection?.readyState === WebSocket.OPEN) {
      this.wsSend('message:send', { content });
    } else {
      // Fallback to HTTP if WS not available
      this.sendMessageHTTP(content);
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

        if (!this.isOpen) {
          this.unreadCount++;
          this.triggerCallback('onUnreadCountChange', this.unreadCount);
        }

        this.triggerCallback('onMessageReceived', msg);
        this.render();
        break;
      }

      case 'conversation:created': {
        const convId = data.data?.conversation_id;
        if (convId) {
          this.triggerCallback('onConversationStarted', convId);
        }
        break;
      }

      case 'typing:start':
        this.isTyping = true;
        this.render();
        break;

      case 'typing:stop':
        this.isTyping = false;
        this.render();
        break;

      case 'conversations:listed':
        // Future: handle conversation list display
        break;

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
    if (this.wsConnection) {
      this.wsConnection.onclose = null;
      this.wsConnection.close();
      this.wsConnection = null;
    }
  }

  private triggerCallback(name: string, ...args: any[]): void {
    this.callbacks[name]?.forEach((cb) => cb(...args));
  }
}
