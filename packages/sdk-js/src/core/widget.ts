import type { Config } from './types';

export interface WidgetConfig {
  workspaceId: string;
  branding: {
    primaryColor: string;
    logoUrl?: string;
    welcomeMessage: string;
    widgetPosition: 'bottom-right' | 'bottom-left';
  };
  features: {
    aiEnabled: boolean;
    fileUploads: boolean;
    preChatForm: boolean;
    csatRating: boolean;
  };
}

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
  private host = 'sdk.helpin.ai';

  // Store bound event handlers for cleanup
  private boundHandlers: { element: Element; event: string; handler: EventListener }[] = [];

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

    if (this.wsRetryTimer) {
      clearTimeout(this.wsRetryTimer);
      this.wsRetryTimer = null;
    }

    this.disconnectWebSocket();
    this.removeEventListeners();
    this.removeWidget();
  }

  show(): void {
    this.isOpen = true;
    this.ensureWidget();
    this.updateWidgetVisibility(true);
    this.triggerCallback('onShow');
  }

  hide(): void {
    this.isOpen = false;
    this.updateWidgetVisibility(false);
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
    this.show();
  }

  showNewMessage(content?: string): void {
    this.show();
  }

  showConversation(conversationId: string): void {
    this.show();
  }

  showArticle(articleId: string): void {
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
    } catch (error) {
      console.error('Failed to fetch widget config:', error);
    }
  }

  private ensureWidget(): void {
    if (document.getElementById('helpin-widget-container')) return;

    const container = document.createElement('div');
    container.id = 'helpin-widget-container';
    container.innerHTML = `
      <style>
        ${this.getWidgetStyles()}
      </style>
      <div class="helpin-widget">
        <button class="helpin-launcher" aria-label="Open chat">
          <svg viewBox="0 0 24 24" width="28" height="28" fill="white">
            <path d="M20 2H4c-1.1 0-2 .9-2 2v18l4-4h14c1.1 0 2-.9 2-2V4c0-1.1-.9-2-2-2zm0 14H6l-2 2V4h16v12z"/>
          </svg>
        </button>
        <div class="helpin-chat-window" style="display: none;">
          <div class="helpin-widget-header">
            <div class="helpin-header-content">
              <div class="helpin-header-title">Support</div>
            </div>
            <button class="helpin-header-close">
              <svg viewBox="0 0 24 24" width="20" height="20" fill="white">
                <path d="M19 6.41L17.59 5 12 10.59 6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 12 13.41 17.59 19 19 17.59 13.41 12z"/>
              </svg>
            </button>
          </div>
          <div class="helpin-chat-content">
            <div class="helpin-pre-chat-form">
              <div class="helpin-pre-chat-welcome">Hi! How can we help you today?</div>
              <form class="helpin-pre-chat-email-form">
                <input type="email" class="helpin-input" placeholder="Enter your email" required>
                <button type="submit" class="helpin-btn-primary">Continue</button>
              </form>
            </div>
          </div>
        </div>
      </div>
    `;

    document.body.appendChild(container);
    this.attachEventListeners();
  }

  private getWidgetStyles(): string {
    const brandColor = this.widgetConfig?.branding?.primaryColor || '#6366f1';
    return `
      .helpin-widget {
        position: fixed;
        bottom: 20px;
        right: 20px;
        z-index: 999999;
        font-family: system-ui, -apple-system, sans-serif;
      }
      .helpin-launcher {
        width: 60px;
        height: 60px;
        border-radius: 50%;
        border: none;
        background-color: ${brandColor};
        cursor: pointer;
        display: flex;
        align-items: center;
        justify-content: center;
        box-shadow: 0 4px 24px rgba(0, 0, 0, 0.12);
        transition: transform 0.2s;
      }
      .helpin-launcher:hover { transform: scale(1.05); }
      .helpin-chat-window {
        position: absolute;
        bottom: 80px;
        right: 0;
        width: 380px;
        max-width: calc(100vw - 40px);
        height: 600px;
        max-height: calc(100vh - 120px);
        background: #fff;
        border-radius: 12px;
        box-shadow: 0 8px 32px rgba(0, 0, 0, 0.16);
        display: flex;
        flex-direction: column;
        overflow: hidden;
      }
      .helpin-widget-header {
        padding: 16px;
        background: ${brandColor};
        color: white;
        display: flex;
        align-items: center;
        justify-content: space-between;
      }
      .helpin-header-title { font-weight: 600; font-size: 16px; }
      .helpin-header-close {
        background: transparent;
        border: none;
        cursor: pointer;
        padding: 4px;
        opacity: 0.8;
      }
      .helpin-header-close:hover { opacity: 1; }
      .helpin-chat-content {
        flex: 1;
        overflow-y: auto;
        padding: 16px;
      }
      .helpin-pre-chat-welcome {
        font-size: 16px;
        font-weight: 500;
        margin-bottom: 16px;
        text-align: center;
      }
      .helpin-input {
        width: 100%;
        padding: 12px;
        border: 1px solid #e5e7eb;
        border-radius: 8px;
        font-size: 14px;
        margin-bottom: 12px;
        box-sizing: border-box;
      }
      .helpin-input:focus { outline: none; border-color: ${brandColor}; }
      .helpin-btn-primary {
        width: 100%;
        padding: 12px;
        border: none;
        border-radius: 8px;
        background: ${brandColor};
        color: white;
        font-size: 14px;
        font-weight: 500;
        cursor: pointer;
      }
      .helpin-btn-primary:hover { opacity: 0.9; }
      .helpin-sr-only {
        position: absolute;
        width: 1px;
        height: 1px;
        padding: 0;
        margin: -1px;
        overflow: hidden;
        clip: rect(0, 0, 0, 0);
        white-space: nowrap;
        border-width: 0;
      }
    `;
  }

  private attachEventListeners(): void {
    const launcher = document.querySelector('.helpin-launcher');
    const closeBtn = document.querySelector('.helpin-header-close');
    const emailForm = document.querySelector('.helpin-pre-chat-email-form') as HTMLFormElement;

    if (launcher) {
      const handler = () => this.toggle();
      launcher.addEventListener('click', handler);
      this.boundHandlers.push({ element: launcher, event: 'click', handler });
    }

    if (closeBtn) {
      const handler = () => this.hide();
      closeBtn.addEventListener('click', handler);
      this.boundHandlers.push({ element: closeBtn, event: 'click', handler });
    }

    if (emailForm) {
      const handler = async (e: Event) => {
        e.preventDefault();
        const emailInput = emailForm.querySelector('input[type="email"]') as HTMLInputElement;
        const email = emailInput.value;

        this.triggerCallback('onUserEmailSupplied', email);
        try {
          await this.initializeSession({ email });
        } catch (error) {
          console.error('Failed to initialize session from form:', error);
        }
      };
      emailForm.addEventListener('submit', handler);
      this.boundHandlers.push({ element: emailForm, event: 'submit', handler });
    }
  }

  private removeEventListeners(): void {
    for (const { element, event, handler } of this.boundHandlers) {
      element.removeEventListener(event, handler);
    }
    this.boundHandlers = [];
  }

  private updateWidgetVisibility(open: boolean): void {
    const chatWindow = document.querySelector('.helpin-chat-window') as HTMLElement;
    if (chatWindow) {
      chatWindow.style.display = open ? 'flex' : 'none';
    }
  }

  private removeWidget(): void {
    const container = document.getElementById('helpin-widget-container');
    if (container) {
      container.remove();
    }
  }

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
            this.unreadCount++;
            this.triggerCallback('onUnreadCountChange', this.unreadCount);
            this.triggerCallback('onMessageReceived', data);
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
