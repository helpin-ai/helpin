import { mountWidget, unmountWidget } from '@helpin/widget-core';
import type { WidgetConfig, Message, MountWidgetOptions, WidgetView } from '@helpin/widget-core';
// @ts-ignore — Vite ?inline import returns CSS as a string
import widgetStyles from '@helpin/widget-core/styles?inline';

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
  private host = 'api.helpin.ai';

  // Preact mount state
  private mountContainer: HTMLElement | null = null;
  private messages: Message[] = [];
  private currentView: WidgetView = 'home';
  private isTyping = false;

  private callbacks: Record<string, WidgetCallback[]> = {
    onShow: [],
    onHide: [],
    onUnreadCountChange: [],
    onUserEmailSupplied: [],
    onConversationStarted: [],
    onMessageReceived: [],
  };

  boot(settings: WidgetSettings): void {
    // Clean up previous boot if any
    if (this.config) {
      this.cleanup();
    }

    this.isShutdown = false;
    this.config = settings;
    this.host = settings.host || this.host;

    if (settings.user) {
      this.initializeSession(settings.user).catch((error) => {
        console.error('Failed to initialize session during boot:', error);
      });
    } else {
      this.fetchWidgetConfig().catch((error) => {
        console.error('Failed to fetch widget config during boot:', error);
      });
    }
  }

  shutdown(): void {
    this.isShutdown = true;
    this.cleanup();
  }

  private cleanup(): void {
    this.config = null;
    this.widgetConfig = null;
    this.sessionToken = null;
    this.isOpen = false;
    this.unreadCount = 0;
    this.wsRetryCount = 0;
    this.messages = [];
    this.currentView = 'home';
    this.isTyping = false;

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
    return localStorage.getItem('helpin_visitor_id') || '';
  }

  isWidgetReady(): boolean {
    return this.widgetConfig !== null;
  }

  // ─── Preact Rendering ─────────────────────────────────────

  private ensureWidget(): void {
    if (this.mountContainer) return;

    // Inject widget-core CSS once
    if (!document.getElementById('helpin-widget-styles')) {
      const style = document.createElement('style');
      style.id = 'helpin-widget-styles';
      style.textContent = widgetStyles;
      document.head.appendChild(style);
    }

    const container = document.createElement('div');
    container.id = 'helpin-widget-container';
    document.body.appendChild(container);
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
    });
  }

  private removeWidget(): void {
    if (this.mountContainer) {
      unmountWidget(this.mountContainer);
      this.mountContainer.remove();
      this.mountContainer = null;
    }
  }

  // ─── Message Handling ──────────────────────────────────────

  private async handleSendMessage(content: string): Promise<void> {
    if (!content.trim()) return;

    // If no session yet, try initializing with just the message
    if (!this.sessionToken) {
      console.warn('No session token — message not sent. Complete pre-chat form first.');
      return;
    }

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

    try {
      await fetch(`https://${this.host}/v1/widget/messages`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${this.sessionToken}`,
        },
        body: JSON.stringify({ content }),
      });
    } catch (error) {
      console.error('Failed to send message:', error);
    }
  }

  private async handlePreChatSubmit(data: { name: string; email: string }): Promise<void> {
    this.triggerCallback('onUserEmailSupplied', data.email);
    try {
      await this.initializeSession({ email: data.email, name: data.name });
      this.render(); // Re-render to hide pre-chat form
    } catch (error) {
      console.error('Failed to initialize session from pre-chat form:', error);
    }
  }

  // ─── API / Session ─────────────────────────────────────────

  private async initializeSession(user: WidgetUser): Promise<void> {
    if (!this.config?.key) return;

    try {
      const response = await fetch(`https://${this.host}/v1/widget/session`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          widget_key: this.config.key,
          ...user,
        }),
      });

      if (!response.ok) {
        throw new Error(`Session init failed: ${response.status}`);
      }

      const data = await response.json();
      this.sessionToken = data.session_token;

      if (user.email) {
        this.triggerCallback('onUserEmailSupplied', user.email);
      }

      await this.fetchWidgetConfig();
      this.connectWebSocket();
      this.triggerCallback('onConversationStarted', data.conversation_id);
    } catch (error) {
      console.error('Failed to initialize session:', error);
    }
  }

  private async fetchWidgetConfig(): Promise<void> {
    if (!this.config?.key) return;

    try {
      const response = await fetch(
        `https://${this.host}/v1/widget/config?key=${encodeURIComponent(this.config.key)}`
      );

      if (!response.ok) {
        throw new Error(`Config fetch failed: ${response.status}`);
      }

      this.widgetConfig = await response.json();
      this.ensureWidget();
      this.render();
    } catch (error) {
      console.error('Failed to fetch widget config:', error);
    }
  }

  // ─── WebSocket ──────────────────────────────────────────────

  private connectWebSocket(): void {
    if (!this.sessionToken || this.isShutdown) return;

    try {
      this.wsConnection = new WebSocket(
        `wss://${this.host}/v1/ws?session_token=${this.sessionToken}`
      );

      this.wsConnection.onmessage = (event) => {
        try {
          const data = JSON.parse(event.data);
          if (data.entity === 'support_conversation_message') {
            const msg: Message = {
              id: data.id || `ws-${Date.now()}`,
              conversationId: data.conversation_id || '',
              role: data.role || 'agent',
              content: data.content || '',
              senderId: data.sender_id,
              isInternal: false,
              createdAt: data.created_at || new Date().toISOString(),
              sources: data.sources,
              attachments: data.attachments,
            };
            this.messages = [...this.messages, msg];

            if (!this.isOpen) {
              this.unreadCount++;
              this.triggerCallback('onUnreadCountChange', this.unreadCount);
            }

            this.triggerCallback('onMessageReceived', data);
            this.render();
          }
        } catch {
          console.error('Failed to parse WebSocket message');
        }
      };

      this.wsConnection.onopen = () => {
        this.wsRetryCount = 0;
      };

      this.wsConnection.onclose = () => {
        if (this.isShutdown) return;

        if (this.wsRetryCount >= MAX_WS_RETRIES) {
          console.error(`WebSocket: gave up after ${MAX_WS_RETRIES} retries`);
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
    }
  }

  private disconnectWebSocket(): void {
    if (this.wsConnection) {
      this.wsConnection.onclose = null; // Prevent reconnection on intentional close
      this.wsConnection.close();
      this.wsConnection = null;
    }
  }

  private triggerCallback(name: string, ...args: any[]): void {
    this.callbacks[name]?.forEach((cb) => cb(...args));
  }
}
