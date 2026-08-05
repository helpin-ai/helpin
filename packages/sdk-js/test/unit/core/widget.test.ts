import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { WidgetManager } from '../../../src/core/widget';
import { mountWidget } from '@helpin-ai/widget-core';

class MockWebSocket {
  static OPEN = 1;
  static CLOSED = 3;

  readyState = MockWebSocket.OPEN;
  sent: string[] = [];
  onopen: ((event: Event) => void) | null = null;
  onmessage: ((event: MessageEvent) => void) | null = null;
  onclose: ((event: CloseEvent) => void) | null = null;
  onerror: ((event: Event) => void) | null = null;

  constructor(public url: string) {
    MockWebSocket.instances.push(this);
    setTimeout(() => this.onopen?.(new Event('open')), 0);
  }

  send(payload: string): void {
    this.sent.push(payload);
  }

  close(): void {
    this.readyState = MockWebSocket.CLOSED;
    this.onclose?.(new CloseEvent('close'));
  }

  static instances: MockWebSocket[] = [];
  static reset(): void {
    MockWebSocket.instances = [];
  }
}

const flushAsync = async () => {
  await Promise.resolve();
  await Promise.resolve();
};

describe('WidgetManager', () => {
  let widget: WidgetManager;
  let fetchMock: ReturnType<typeof vi.fn>;
  let hasFocusSpy: ReturnType<typeof vi.spyOn>;

  const setDocumentVisibility = (state: 'visible' | 'hidden') => {
    Object.defineProperty(document, 'visibilityState', {
      configurable: true,
      value: state,
    });
    Object.defineProperty(document, 'hidden', {
      configurable: true,
      value: state === 'hidden',
    });
  };

  beforeEach(() => {
    widget = new WidgetManager();
    document.body.innerHTML = '';
    document.title = 'Original title';
    setDocumentVisibility('visible');
    MockWebSocket.reset();
    localStorage.clear();
    hasFocusSpy = vi.spyOn(document, 'hasFocus').mockReturnValue(true);

    (localStorage.getItem as any).mockReset();
    (localStorage.setItem as any).mockReset();
    (localStorage.removeItem as any).mockReset();
    (localStorage.clear as any).mockReset();
    (mountWidget as ReturnType<typeof vi.fn>).mockClear();

    fetchMock = vi.fn(() =>
      Promise.resolve({
        ok: true,
        json: () => Promise.resolve({
          workspaceId: 'ws_test',
          branding: { primaryColor: '#6366f1' },
          features: {}
        }),
      })
    );
    vi.stubGlobal('fetch', fetchMock);
  });

  afterEach(() => {
    widget.shutdown();
    hasFocusSpy.mockRestore();
    vi.unstubAllGlobals();
  });

  describe('boot', () => {
    it('should initialize with widget key', async () => {
      widget.boot({ key: 'test-key' });
      await new Promise((r) => setTimeout(r, 100));
      expect(widget.isWidgetReady()).toBe(true);
    });

    it('should initialize session when user is provided', async () => {
      const fetchMock = vi.fn(() =>
        Promise.resolve({
          ok: true,
          json: () => Promise.resolve({ session_token: 'test-token', conversation_id: 'conv-1' }),
        })
      );
      vi.stubGlobal('fetch', fetchMock);

      widget.boot({ key: 'test-key', user: { email: 'test@example.com' } });

      await new Promise((r) => setTimeout(r, 100));

      expect(fetchMock).toHaveBeenCalled();
    });

    it('should not throw without widget key', () => {
      expect(() => widget.boot({ } as any)).not.toThrow();
    });

    it('logs and skips boot when widget key is missing', () => {
      const errorSpy = vi.spyOn(console, 'error').mockImplementation(() => {});

      widget.boot({ host: 'https://test.helpin.ai' });

      expect(fetchMock).not.toHaveBeenCalled();
      expect(errorSpy).toHaveBeenCalledWith(
        '[Helpin] Widget boot skipped: widgetKey is required.',
      );
      errorSpy.mockRestore();
    });

    it('uses cached config and skips the network fetch', async () => {
      (localStorage.getItem as any).mockImplementation((key: string) => {
        if (key === 'helpin_wc_test-key') {
          return JSON.stringify({
            config: {
              workspaceId: 'ws_cached',
              branding: { primaryColor: '#112233' },
              features: { preChatForm: true },
            },
            cached_at: Date.now(),
          });
        }
        return null;
      });

      widget.boot({ key: 'test-key' });
      await flushAsync();

      expect(fetchMock).not.toHaveBeenCalled();
      expect(widget.isWidgetReady()).toBe(true);
      const latestOptions = (mountWidget as ReturnType<typeof vi.fn>).mock.calls.at(-1)?.[1];
      expect(latestOptions?.config?.workspaceId).toBe('ws_cached');
    });

    it('restores a persisted session token on websocket open', async () => {
      (localStorage.getItem as any).mockImplementation((key: string) => {
        if (key === 'helpin_ws_test-key') {
          return JSON.stringify({
            session_token: 'persisted-token',
            expires_at: new Date(Date.now() + 60_000).toISOString(),
          });
        }
        return null;
      });
      vi.stubGlobal('WebSocket', MockWebSocket as unknown as typeof WebSocket);

      widget.boot({ key: 'test-key' });
      await new Promise((r) => setTimeout(r, 50));

      expect(MockWebSocket.instances).toHaveLength(1);
      expect(MockWebSocket.instances[0].url).toBe('wss://client.prod.helpin.ai/widget/ws?key=test-key');
      expect(MockWebSocket.instances[0].sent.map((frame) => JSON.parse(frame))).toContainEqual({
        type: 'session:restore',
        data: { session_token: 'persisted-token' },
      });
    });

    it('retries with session:create after session:error clears an invalid stored token', async () => {
      vi.stubGlobal('WebSocket', MockWebSocket as unknown as typeof WebSocket);
      (localStorage.getItem as any).mockImplementation((key: string) => {
        if (key === 'helpin_ws_test-key') {
          return JSON.stringify({
            session_token: 'expired-token',
            expires_at: new Date(Date.now() + 60_000).toISOString(),
          });
        }
        return null;
      });

      widget.boot({ key: 'test-key' });
      await new Promise((r) => setTimeout(r, 50));

      const socket = MockWebSocket.instances[0];
      (widget as any).handleWSMessage({ type: 'session:error' });

      const frames = socket.sent.map((frame) => JSON.parse(frame));
      expect(frames).toContainEqual({
        type: 'session:create',
        data: expect.objectContaining({
          anonymous_id: expect.any(String),
          page_url: expect.any(String),
        }),
      });
      expect(localStorage.removeItem).toHaveBeenCalledWith('helpin_ws_test-key');
    });
  });

  describe('visibility and open state', () => {
    it('should create widget element when shown', async () => {
      widget.boot({ key: 'test-key' });
      await new Promise((r) => setTimeout(r, 100));
      
      widget.show();
      
      expect(document.getElementById('helpin-widget-container')).not.toBeNull();
    });

    it('should not call onOpen callback for programmatic open', async () => {
      widget.boot({ key: 'test-key' });
      await new Promise((r) => setTimeout(r, 100));
      
      const callback = vi.fn();
      widget.onOpen(callback);
      widget.open();
      expect(callback).not.toHaveBeenCalled();
    });

    it('should not call onClose callback for programmatic close', async () => {
      widget.boot({ key: 'test-key' });
      await new Promise((r) => setTimeout(r, 100));
      
      const callback = vi.fn();
      widget.onClose(callback);
      widget.open();
      widget.close();
      expect(callback).not.toHaveBeenCalled();
    });

    it('should call onOpen callback for user-initiated launcher open', async () => {
      widget.boot({ key: 'test-key' });
      await new Promise((r) => setTimeout(r, 100));

      const callback = vi.fn();
      widget.onOpen(callback);

      const mockMount = mountWidget as ReturnType<typeof vi.fn>;
      const latestOptions = mockMount.mock.calls.at(-1)?.[1];
      latestOptions?.onLauncherClick();

      expect(callback).toHaveBeenCalledTimes(1);
    });

    it('should call onClose callback for user-initiated close', async () => {
      widget.boot({ key: 'test-key' });
      await new Promise((r) => setTimeout(r, 100));

      const callback = vi.fn();
      widget.onClose(callback);
      widget.open();

      const mockMount = mountWidget as ReturnType<typeof vi.fn>;
      const latestOptions = mockMount.mock.calls.at(-1)?.[1];
      latestOptions?.onClose();

      expect(callback).toHaveBeenCalledTimes(1);
    });

    it('allows close() inside onClose without recursion', async () => {
      widget.boot({ key: 'test-key' });
      await new Promise((r) => setTimeout(r, 100));

      const callback = vi.fn(() => widget.close());
      widget.onClose(callback);
      widget.open();

      const mockMount = mountWidget as ReturnType<typeof vi.fn>;
      const latestOptions = mockMount.mock.calls.at(-1)?.[1];
      latestOptions?.onClose();

      expect(callback).toHaveBeenCalledTimes(1);
    });

    it('should separate visibility from open state via mountWidget', async () => {
      widget.boot({ key: 'test-key' });
      await new Promise((r) => setTimeout(r, 100));

      const mockMount = mountWidget as ReturnType<typeof vi.fn>;
      mockMount.mockClear();

      widget.show();
      expect(mockMount).toHaveBeenCalled();
      const showCall = mockMount.mock.calls[mockMount.mock.calls.length - 1];
      expect(showCall[1].showLauncher).toBe(true);
      expect(showCall[1].isOpen).toBe(false);

      mockMount.mockClear();
      widget.open();
      expect(mockMount).toHaveBeenCalled();
      const openCall = mockMount.mock.calls[mockMount.mock.calls.length - 1];
      expect(openCall[1].showLauncher).toBe(true);
      expect(openCall[1].isOpen).toBe(true);

      mockMount.mockClear();
      widget.close();
      expect(mockMount).toHaveBeenCalled();
      const closeCall = mockMount.mock.calls[mockMount.mock.calls.length - 1];
      expect(closeCall[1].showLauncher).toBe(true);
      expect(closeCall[1].isOpen).toBe(false);

      mockMount.mockClear();
      widget.hide();
      expect(mockMount).toHaveBeenCalled();
      const hideCall = mockMount.mock.calls[mockMount.mock.calls.length - 1];
      expect(hideCall[1].showLauncher).toBe(false);
      expect(hideCall[1].isOpen).toBe(false);
    });
  });

  describe('callbacks', () => {
    it('should handle onUnreadCountChange', () => {
      const callback = vi.fn();
      widget.onUnreadCountChange(callback);
      expect(callback).toHaveBeenCalledWith(0);
    });

    it('should handle onUserEmailSupplied', () => {
      const callback = vi.fn();
      widget.onUserEmailSupplied(callback);
      callback('test@example.com');
      expect(callback).toHaveBeenCalledWith('test@example.com');
    });

    it('should handle onConversationStarted', () => {
      const callback = vi.fn();
      widget.onConversationStarted(callback);
      callback('conv-123');
      expect(callback).toHaveBeenCalledWith('conv-123');
    });

    it('should handle onMessageReceived', () => {
      const callback = vi.fn();
      widget.onMessageReceived(callback);
      callback({ id: 'msg-1' });
      expect(callback).toHaveBeenCalledWith({ id: 'msg-1' });
    });
  });

  describe('visitor id', () => {
    it('should return empty string initially', () => {
      expect(widget.getVisitorId()).toBe('');
    });
  });

  describe('shutdown', () => {
    it('should clean up widget elements', async () => {
      widget.boot({ key: 'test-key' });
      await new Promise((r) => setTimeout(r, 100));
      
      widget.show();
      expect(document.getElementById('helpin-widget-container')).not.toBeNull();
      
      widget.shutdown();
      expect(document.getElementById('helpin-widget-container')).toBeNull();
    });

    it('revokes over websocket when the socket is open', () => {
      const close = vi.fn();
      (widget as any).widgetKey = 'test-key';
      (widget as any).wsConnection = {
        readyState: WebSocket.OPEN,
        send: vi.fn(),
        close,
        onclose: null,
      };

      widget.shutdown();

      expect((widget as any).wsConnection).toBeNull();
      expect(close).toHaveBeenCalled();
      expect(localStorage.removeItem).toHaveBeenCalledWith('helpin_ws_test-key');
      expect(localStorage.removeItem).toHaveBeenCalledWith('helpin_wc_test-key');
      expect(localStorage.removeItem).toHaveBeenCalledWith('helpin_prechat_test-key');
    });

    it('falls back to HTTP revoke when websocket is unavailable', async () => {
      (widget as any).widgetKey = 'test-key';
      (widget as any).host = 'client.prod.helpin.ai';
      (widget as any).sessionToken = 'session-123';
      (widget as any).wsConnection = { readyState: 3, close: vi.fn(), onclose: null };

      widget.shutdown();
      await flushAsync();

      expect(fetchMock).toHaveBeenCalledWith(
        'https://client.prod.helpin.ai/widget/session/revoke',
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify({ session_token: 'session-123' }),
        }),
      );
    });
  });

  describe('API methods', () => {
    it('should have openMessages method', async () => {
      widget.boot({ key: 'test-key' });
      await new Promise((r) => setTimeout(r, 100));
      
      expect(() => widget.openMessages()).not.toThrow();
    });

    it('should have openConversation method', async () => {
      widget.boot({ key: 'test-key' });
      await new Promise((r) => setTimeout(r, 100));
      
      expect(() => widget.openConversation('conv-123')).not.toThrow();
    });

    it('should have openArticle method', async () => {
      widget.boot({ key: 'test-key' });
      await new Promise((r) => setTimeout(r, 100));

      const mockMount = mountWidget as ReturnType<typeof vi.fn>;
      mockMount.mockClear();

      widget.openArticle('article-123');

      expect(mockMount).toHaveBeenCalled();
      const latestOptions = mockMount.mock.calls[mockMount.mock.calls.length - 1][1];
      expect(latestOptions.initialView).toBe('help-article');
      expect(latestOptions.openArticleRequest).toEqual({
        key: 1,
        articleSlug: 'article-123',
      });
    });
  });

  describe('websocket reconnect policy', () => {
    it('keeps retrying in the background after initial connection failures', async () => {
      vi.useFakeTimers();
      const randomSpy = vi.spyOn(Math, 'random').mockReturnValue(0);

      class NoOpenWebSocket {
        static OPEN = 1;
        static CLOSED = 3;
        static instances: NoOpenWebSocket[] = [];

        readyState = NoOpenWebSocket.OPEN;
        sent: string[] = [];
        onopen: ((event: Event) => void) | null = null;
        onmessage: ((event: MessageEvent) => void) | null = null;
        onclose: ((event: CloseEvent) => void) | null = null;
        onerror: ((event: Event) => void) | null = null;

        constructor(public url: string) {
          NoOpenWebSocket.instances.push(this);
        }

        send(payload: string): void {
          this.sent.push(payload);
        }

        close(): void {
          this.readyState = NoOpenWebSocket.CLOSED;
          this.onclose?.(new CloseEvent('close'));
        }
      }

      vi.stubGlobal('WebSocket', NoOpenWebSocket as unknown as typeof WebSocket);
      (widget as any).widgetKey = 'test-key';
      (widget as any).widgetConfig = {
        workspaceId: 'ws_test',
        branding: { primaryColor: '#6366f1' },
        features: {},
      };
      (widget as any).mountContainer = document.createElement('div');

      (widget as any).connectWebSocket();
      expect(NoOpenWebSocket.instances).toHaveLength(1);

      (widget as any).wsRetryCount = 3;
      NoOpenWebSocket.instances[0].close();

      expect((widget as any).connectionStatus).toBe('failed');
      expect((widget as any).wsRetryTimer).toBeTruthy();

      await vi.advanceTimersByTimeAsync(8000);
      expect(NoOpenWebSocket.instances).toHaveLength(2);

      randomSpy.mockRestore();
      vi.useRealTimers();
    });

    it('keeps retrying in the background after a long outage post-connect', async () => {
      vi.useFakeTimers();
      const randomSpy = vi.spyOn(Math, 'random').mockReturnValue(0);
      vi.stubGlobal('WebSocket', MockWebSocket as unknown as typeof WebSocket);

      (widget as any).widgetKey = 'test-key';
      (widget as any).widgetConfig = {
        workspaceId: 'ws_test',
        branding: { primaryColor: '#6366f1' },
        features: {},
      };
      (widget as any).mountContainer = document.createElement('div');

      (widget as any).connectWebSocket();
      await vi.advanceTimersByTimeAsync(0);
      expect(MockWebSocket.instances).toHaveLength(1);

      (widget as any).wsRetryCount = 10;
      (widget as any).connectionIssueStartedAt = Date.now() - 30_000;
      MockWebSocket.instances[0].close();

      expect((widget as any).connectionStatus).toBe('failed');
      expect((widget as any).wsRetryTimer).toBeTruthy();

      await vi.advanceTimersByTimeAsync(120_000);
      expect(MockWebSocket.instances).toHaveLength(2);

      randomSpy.mockRestore();
      vi.useRealTimers();
    });
  });

  describe('conversation routing', () => {
    it('should start a fresh conversation when home sends a new message', async () => {
      const sockets: MockWebSocket[] = [];

      class MockWebSocket {
        static OPEN = 1;
        static CLOSED = 3;

        readyState = MockWebSocket.OPEN;
        sent: string[] = [];
        onopen: ((event: Event) => void) | null = null;
        onmessage: ((event: MessageEvent) => void) | null = null;
        onclose: ((event: CloseEvent) => void) | null = null;
        onerror: ((event: Event) => void) | null = null;

        constructor(_url: string) {
          sockets.push(this);
          setTimeout(() => this.onopen?.(new Event('open')), 0);
        }

        send(payload: string): void {
          this.sent.push(payload);
        }

        close(): void {
          this.readyState = MockWebSocket.CLOSED;
          this.onclose?.(new CloseEvent('close'));
        }
      }

      vi.stubGlobal('WebSocket', MockWebSocket as unknown as typeof WebSocket);

      widget.boot({ key: 'test-key' });
      await new Promise((r) => setTimeout(r, 100));

      const ws = sockets[0];
      expect(ws).toBeTruthy();

      (widget as any).activeConversationId = 'conv-old';
      (widget as any).messages = [{
        id: 'msg-old',
        conversationId: 'conv-old',
        role: 'agent',
        content: 'Older thread',
        isInternal: false,
        createdAt: new Date().toISOString(),
      }];
      (widget as any).render();

      const mockMount = mountWidget as ReturnType<typeof vi.fn>;
      const latestOptions = mockMount.mock.calls[mockMount.mock.calls.length - 1][1];
      latestOptions.onSendMessageFromHome('Fresh question');

      const outgoingFrames = ws.sent.slice(-2).map((frame) => JSON.parse(frame));
      expect(outgoingFrames).toEqual([
        { type: 'conversation:new', data: {} },
        { type: 'message:send', data: { content: 'Fresh question' } },
      ]);

      const postSendOptions = mockMount.mock.calls[mockMount.mock.calls.length - 1][1];
      expect(postSendOptions.messages).toHaveLength(1);
      expect(postSendOptions.messages[0].content).toBe('Fresh question');
    });

    it('shows AI thinking immediately after sending an AI-first widget message', () => {
      const sent: string[] = [];
      (widget as any).widgetConfig = {
        workspaceId: 'ws_test',
        branding: { primaryColor: '#6366f1' },
        features: { aiEnabled: true, aiFirst: true },
      };
      (widget as any).mountContainer = document.createElement('div');
      (widget as any).wsConnection = {
        readyState: WebSocket.OPEN,
        send: (payload: string) => sent.push(payload),
        close: vi.fn(),
        onclose: null,
      };

      (widget as any).render();
      const mockMount = mountWidget as ReturnType<typeof vi.fn>;
      const latestOptions = mockMount.mock.calls.at(-1)?.[1];

      latestOptions.onSendMessage('Need help');

      const postSendOptions = mockMount.mock.calls.at(-1)?.[1];
      expect(postSendOptions.messages.at(-1)?.content).toBe('Need help');
      expect(postSendOptions.isAIThinking).toBe(true);
      expect(sent.map((frame) => JSON.parse(frame))).toContainEqual({
        type: 'message:send',
        data: { content: 'Need help' },
      });
    });

    it('clears optimistic AI thinking when a non-customer reply arrives', () => {
      (widget as any).widgetConfig = {
        workspaceId: 'ws_test',
        branding: { primaryColor: '#6366f1' },
        features: { aiEnabled: true, aiFirst: true },
      };
      (widget as any).mountContainer = document.createElement('div');
      (widget as any).activeConversationId = 'conv-1';
      (widget as any).isAIThinking = true;

      (widget as any).handleWSMessage({
        type: 'message:received',
        data: {
          id: 'msg-ai',
          conversation_id: 'conv-1',
          sender_type: 'ai',
          message_type: 'reply',
          content: 'Here is what I found.',
          created_at: new Date().toISOString(),
        },
      });

      const latestOptions = (mountWidget as ReturnType<typeof vi.fn>).mock.calls.at(-1)?.[1];
      expect(latestOptions?.isAIThinking).toBe(false);
      expect(latestOptions?.messages.at(-1)?.content).toBe('Here is what I found.');
    });

    it('maps email projection fields from live message payloads', () => {
      (widget as any).widgetConfig = {
        workspaceId: 'ws_test',
        branding: { primaryColor: '#6366f1' },
        features: {},
      };
      (widget as any).mountContainer = document.createElement('div');
      (widget as any).activeConversationId = 'conv-1';

      (widget as any).handleWSMessage({
        type: 'message:received',
        data: {
          id: 'msg-email',
          conversation_id: 'conv-1',
          sender_type: 'user',
          content: 'Legacy fallback',
          via_channel: 'email',
          email_visible_text: 'Visible reply',
          email_quoted_text: '',
          email_has_quoted_content: false,
          email_projection_confidence: 'none',
          email_projection_version: 1,
          created_at: new Date().toISOString(),
        },
      });

      const latestOptions = (mountWidget as ReturnType<typeof vi.fn>).mock.calls.at(-1)?.[1];
      expect(latestOptions?.messages.at(-1)).toMatchObject({
        emailVisibleText: 'Visible reply',
        emailQuotedText: '',
        emailHasQuotedContent: false,
        emailProjectionConfidence: 'none',
        emailProjectionVersion: 1,
      });
    });

    it('does not show optimistic AI thinking after a conversation is escalated to a human', () => {
      const sent: string[] = [];
      (widget as any).widgetConfig = {
        workspaceId: 'ws_test',
        branding: { primaryColor: '#6366f1' },
        features: { aiEnabled: true, aiFirst: true },
      };
      (widget as any).mountContainer = document.createElement('div');
      (widget as any).activeConversationId = 'conv-1';
      (widget as any).conversations = [{
        id: 'conv-1',
        subject: 'Support',
        status: 'open',
        aiState: 'escalated',
        flowState: 'waiting_for_human',
      }];
      (widget as any).wsConnection = {
        readyState: WebSocket.OPEN,
        send: (payload: string) => sent.push(payload),
        close: vi.fn(),
        onclose: null,
      };

      (widget as any).render();
      const latestOptions = (mountWidget as ReturnType<typeof vi.fn>).mock.calls.at(-1)?.[1];

      latestOptions.onSendMessage('Are you there?');

      const postSendOptions = (mountWidget as ReturnType<typeof vi.fn>).mock.calls.at(-1)?.[1];
      expect(postSendOptions.isAIThinking).toBe(false);
      expect(sent.map((frame) => JSON.parse(frame))).toContainEqual({
        type: 'message:send',
        data: { content: 'Are you there?' },
      });
    });
  });

  describe('typing fallback', () => {
    it('should send typing indicators over HTTP when websocket is unavailable', async () => {
      vi.useFakeTimers();

      const fetchMock = vi.fn(() =>
        Promise.resolve({
          ok: true,
          json: () => Promise.resolve({
            workspaceId: 'ws_test',
            branding: { primaryColor: '#6366f1' },
            features: {}
          }),
        })
      );
      vi.stubGlobal('fetch', fetchMock);

      (widget as any).host = 'client.prod.helpin.ai';
      (widget as any).sessionToken = 'session-123';
      (widget as any).wsConnection = { readyState: 3, close: vi.fn(), onclose: null };

      (widget as any).handleTyping('hello');
      await Promise.resolve();

      expect(fetchMock).toHaveBeenCalledWith(
        'https://client.prod.helpin.ai/widget/typing',
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify({ session_token: 'session-123', is_typing: true }),
        }),
      );

      await vi.advanceTimersByTimeAsync(5000);

      expect(fetchMock).toHaveBeenCalledWith(
        'https://client.prod.helpin.ai/widget/typing',
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify({ session_token: 'session-123', is_typing: false }),
        }),
      );

      vi.useRealTimers();
    });

    it('throttles websocket typing:start payloads and sends typing:stop once idle', async () => {
      vi.useFakeTimers();

      const sent: string[] = [];
      (widget as any).sessionToken = 'session-123';
      (widget as any).wsConnection = {
        readyState: WebSocket.OPEN,
        send: (payload: string) => sent.push(payload),
        close: vi.fn(),
        onclose: null,
      };

      (widget as any).handleTyping('h');
      (widget as any).handleTyping('he');
      await vi.advanceTimersByTimeAsync(100);
      (widget as any).handleTyping('hel');
      await vi.advanceTimersByTimeAsync(220);
      (widget as any).handleTyping('hello');

      const startFrames = sent.map((frame) => JSON.parse(frame)).filter((frame) => frame.type === 'typing:start');
      expect(startFrames).toEqual([
        { type: 'typing:start', data: { content: 'h' } },
        { type: 'typing:start', data: { content: 'hello' } },
      ]);

      await vi.advanceTimersByTimeAsync(5000);
      expect(sent.map((frame) => JSON.parse(frame))).toContainEqual({
        type: 'typing:stop',
        data: {},
      });

      vi.useRealTimers();
    });
  });

  describe('widget unread counts', () => {
    it('keeps conversation badges and launcher unread totals in sync for support replies', async () => {
      widget.boot({ key: 'test-key' });
      await new Promise((r) => setTimeout(r, 100));

      (widget as any).conversations = [{
        id: 'conv-1',
        subject: 'Question',
        status: 'open',
        lastMessage: 'Customer message',
        lastMessageAt: new Date().toISOString(),
        unreadCount: 0,
      }];

      const unreadSpy = vi.fn();
      widget.onUnreadCountChange(unreadSpy);
      widget.show();

      (widget as any).activeConversationId = 'conv-2';
      (widget as any).handleWSMessage({
        type: 'message:received',
        data: {
          id: 'msg-1',
          conversation_id: 'conv-1',
          sender_type: 'user',
          content: 'Agent follow-up',
          created_at: new Date().toISOString(),
        },
      });

      const latestOptions = (mountWidget as ReturnType<typeof vi.fn>).mock.calls.at(-1)?.[1];
      expect(latestOptions?.conversations?.[0]?.unreadCount).toBe(1);
      expect(latestOptions?.unreadCount).toBe(1);
      expect(unreadSpy).toHaveBeenLastCalledWith(1);
    });

    it('updates the document title for hidden-tab inbound replies', async () => {
      widget.boot({ key: 'test-key' });
      await new Promise((r) => setTimeout(r, 100));

      (widget as any).conversations = [{
        id: 'conv-1',
        subject: 'Question',
        status: 'open',
        lastMessage: 'Customer message',
        lastMessageAt: new Date().toISOString(),
        unreadCount: 0,
      }];

      hasFocusSpy.mockReturnValue(false);
      setDocumentVisibility('hidden');
      document.dispatchEvent(new Event('visibilitychange'));

      (widget as any).activeConversationId = 'conv-2';
      (widget as any).handleWSMessage({
        type: 'message:received',
        data: {
          id: 'msg-1',
          conversation_id: 'conv-1',
          sender_type: 'user',
          content: 'Agent follow-up',
          created_at: new Date().toISOString(),
        },
      });

      expect(document.title).toBe('(1) New reply');
    });

    it('treats a hidden active conversation as unread until focus returns', async () => {
      widget.boot({ key: 'test-key' });
      await new Promise((r) => setTimeout(r, 100));

      const sentFrames: string[] = [];
      (widget as any).wsConnection = {
        readyState: WebSocket.OPEN,
        send: (payload: string) => sentFrames.push(payload),
        close: vi.fn(),
      };

      (widget as any).isOpen = true;
      (widget as any).currentView = 'conversation';
      (widget as any).activeConversationId = 'conv-1';
      (widget as any).conversations = [{
        id: 'conv-1',
        subject: 'Question',
        status: 'open',
        lastMessage: 'Customer message',
        lastMessageAt: new Date().toISOString(),
        unreadCount: 0,
      }];

      hasFocusSpy.mockReturnValue(false);
      setDocumentVisibility('hidden');
      document.dispatchEvent(new Event('visibilitychange'));

      (widget as any).handleWSMessage({
        type: 'message:received',
        data: {
          id: 'msg-hidden-active',
          conversation_id: 'conv-1',
          sender_type: 'user',
          content: 'Agent follow-up',
          created_at: new Date().toISOString(),
        },
      });

      expect(document.title).toBe('(1) New reply');
      expect((widget as any).conversations[0].unreadCount).toBe(1);
      expect(sentFrames).toEqual([]);

      hasFocusSpy.mockReturnValue(true);
      setDocumentVisibility('visible');
      window.dispatchEvent(new Event('focus'));

      expect(document.title).toBe('Original title');
      expect((widget as any).conversations[0].unreadCount).toBe(0);
      expect(sentFrames.map((frame) => JSON.parse(frame))).toContainEqual({
        type: 'conversation:read',
        data: { conversation_id: 'conv-1' },
      });
    });

    it('does not update the document title for system messages', async () => {
      widget.boot({ key: 'test-key' });
      await new Promise((r) => setTimeout(r, 100));

      (widget as any).conversations = [{
        id: 'conv-1',
        subject: 'Question',
        status: 'open',
        lastMessage: 'Customer message',
        lastMessageAt: new Date().toISOString(),
        unreadCount: 0,
      }];

      hasFocusSpy.mockReturnValue(false);
      setDocumentVisibility('hidden');
      document.dispatchEvent(new Event('visibilitychange'));

      (widget as any).activeConversationId = 'conv-2';
      (widget as any).handleWSMessage({
        type: 'message:received',
        data: {
          id: 'msg-2',
          conversation_id: 'conv-1',
          sender_type: 'user',
          message_type: 'system',
          system_event_type: 'teammate_joined',
          content: 'Jarek joined the conversation',
          created_at: new Date().toISOString(),
        },
      });

      expect(document.title).toBe('Original title');
    });

    it('restores the original document title when the page regains focus', async () => {
      widget.boot({ key: 'test-key' });
      await new Promise((r) => setTimeout(r, 100));

      (widget as any).conversations = [{
        id: 'conv-1',
        subject: 'Question',
        status: 'open',
        lastMessage: 'Customer message',
        lastMessageAt: new Date().toISOString(),
        unreadCount: 0,
      }];

      hasFocusSpy.mockReturnValue(false);
      setDocumentVisibility('hidden');
      document.dispatchEvent(new Event('visibilitychange'));

      (widget as any).activeConversationId = 'conv-2';
      (widget as any).handleWSMessage({
        type: 'message:received',
        data: {
          id: 'msg-3',
          conversation_id: 'conv-1',
          sender_type: 'user',
          content: 'Agent follow-up',
          created_at: new Date().toISOString(),
        },
      });

      expect(document.title).toBe('(1) New reply');

      hasFocusSpy.mockReturnValue(true);
      setDocumentVisibility('visible');
      window.dispatchEvent(new Event('focus'));

      expect(document.title).toBe('Original title');
    });

    it('clears title notifications when the unread conversation is opened', async () => {
      widget.boot({ key: 'test-key' });
      await new Promise((r) => setTimeout(r, 100));

      (widget as any).conversations = [{
        id: 'conv-1',
        subject: 'Question',
        status: 'open',
        lastMessage: 'Customer message',
        lastMessageAt: new Date().toISOString(),
        unreadCount: 0,
      }];

      hasFocusSpy.mockReturnValue(false);
      setDocumentVisibility('hidden');
      document.dispatchEvent(new Event('visibilitychange'));

      (widget as any).activeConversationId = 'conv-2';
      (widget as any).handleWSMessage({
        type: 'message:received',
        data: {
          id: 'msg-4',
          conversation_id: 'conv-1',
          sender_type: 'user',
          content: 'Agent follow-up',
          created_at: new Date().toISOString(),
        },
      });

      expect(document.title).toBe('(1) New reply');

      (widget as any).handleSelectConversation('conv-1');

      expect(document.title).toBe('Original title');
    });

    it('maps via_channel and respects #helpin-conv deep links on session join', async () => {
      window.location.hash = '#helpin-conv=conv-2';

      widget.boot({ key: 'test-key' });
      await new Promise((r) => setTimeout(r, 100));

      const sentFrames: string[] = [];
      (widget as any).wsConnection = {
        readyState: WebSocket.OPEN,
        send: (payload: string) => sentFrames.push(payload),
        close: vi.fn(),
      };

      (widget as any).handleWSMessage({
        type: 'session:joined',
        data: {
          session_token: 'session-1',
          expires_at: new Date(Date.now() + 60_000).toISOString(),
          is_anonymous: true,
          conversations: [
            {
              id: 'conv-1',
              subject: 'First',
              status: 'open',
              created_at: new Date().toISOString(),
              updated_at: new Date().toISOString(),
            },
            {
              id: 'conv-2',
              subject: 'Second',
              status: 'open',
              created_at: new Date().toISOString(),
              updated_at: new Date().toISOString(),
            },
          ],
          messages: [
            {
              id: 'msg-1',
              conversation_id: 'conv-2',
              sender_type: 'user',
              content: 'Email reply',
              via_channel: 'email',
              email_visible_text: 'Fresh email reply',
              email_quoted_text: 'Earlier email',
              email_has_quoted_content: true,
              email_projection_confidence: 'high',
              email_projection_version: 1,
              created_at: new Date().toISOString(),
            },
          ],
        },
      });

      const latestOptions = (mountWidget as ReturnType<typeof vi.fn>).mock.calls.at(-1)?.[1];
      expect(latestOptions?.messages?.[0]?.viaChannel).toBe('email');
      expect(latestOptions?.messages?.[0]).toMatchObject({
        emailVisibleText: 'Fresh email reply',
        emailQuotedText: 'Earlier email',
        emailHasQuotedContent: true,
        emailProjectionConfidence: 'high',
        emailProjectionVersion: 1,
      });
      expect((widget as any).activeConversationId).toBe('conv-2');
      expect((widget as any).currentView).toBe('conversation');
      expect((widget as any).isOpen).toBe(true);
      expect(sentFrames.map((frame) => JSON.parse(frame))).toContainEqual({
        type: 'conversation:select',
        data: { conversation_id: 'conv-2' },
      });

      window.location.hash = '';
    });
  });

  describe('uploads and transcripts', () => {
    it('uploads attachments through init, put, and confirm requests', async () => {
      fetchMock
        .mockResolvedValueOnce({
          ok: true,
          json: async () => ({
            attachment: { id: 'att-1' },
            upload_url: 'https://upload.example.com/att-1',
            public_url: 'https://cdn.example.com/att-1.png',
          }),
        })
        .mockResolvedValueOnce({ ok: true })
        .mockResolvedValueOnce({ ok: true });

      (widget as any).host = 'client.prod.helpin.ai';
      (widget as any).sessionToken = 'session-123';

      const file = new File(['hello'], 'note.txt', { type: 'text/plain' });
      const result = await (widget as any).handleUploadAttachment(file, 'local-1');

      expect(result).toEqual({
        attachmentId: 'att-1',
        url: 'https://cdn.example.com/att-1.png',
      });
      expect(fetchMock).toHaveBeenNthCalledWith(
        1,
        'https://client.prod.helpin.ai/widget/support/attachments',
        expect.objectContaining({
          method: 'POST',
          headers: expect.objectContaining({ 'X-Session-Token': 'session-123' }),
        }),
      );
      expect(fetchMock).toHaveBeenNthCalledWith(
        2,
        'https://upload.example.com/att-1',
        expect.objectContaining({ method: 'PUT', body: file }),
      );
      expect(fetchMock).toHaveBeenNthCalledWith(
        3,
        'https://client.prod.helpin.ai/widget/support/attachments/att-1/confirm',
        expect.objectContaining({ method: 'PATCH' }),
      );
    });

    it('returns null when attachment confirm fails', async () => {
      fetchMock
        .mockResolvedValueOnce({
          ok: true,
          json: async () => ({
            attachment: { id: 'att-2' },
            upload_url: 'https://upload.example.com/att-2',
            public_url: 'https://cdn.example.com/att-2.png',
          }),
        })
        .mockResolvedValueOnce({ ok: true })
        .mockResolvedValueOnce({ ok: false, status: 500 });

      (widget as any).host = 'client.prod.helpin.ai';
      (widget as any).sessionToken = 'session-123';

      const file = new File(['hello'], 'note.txt', { type: 'text/plain' });
      await expect((widget as any).handleUploadAttachment(file, 'local-2')).resolves.toBeNull();
    });

    it('requests a transcript and stores the supplied email after success', async () => {
      fetchMock.mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          success: true,
          message: 'Transcript sent to person@example.com',
        }),
      });

      (widget as any).host = 'client.prod.helpin.ai';
      (widget as any).sessionToken = 'session-123';
      (widget as any).activeConversationId = 'conv-1';

      await expect(widget.requestConversationTranscript('person@example.com')).resolves.toEqual({
        success: true,
        message: 'Transcript sent to person@example.com',
      });
      expect((widget as any).currentEmail).toBe('person@example.com');
      expect(fetchMock).toHaveBeenCalledWith(
        'https://client.prod.helpin.ai/widget/conversations/conv-1/transcript',
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify({
            session_token: 'session-123',
            email: 'person@example.com',
          }),
        }),
      );
    });

    it('surfaces transcript precondition and server errors', async () => {
      await expect(widget.requestConversationTranscript('person@example.com')).rejects.toThrow('Session not ready');

      (widget as any).sessionToken = 'session-123';
      await expect(widget.requestConversationTranscript('person@example.com')).rejects.toThrow('No active conversation');

      (widget as any).activeConversationId = 'conv-1';
      fetchMock.mockResolvedValueOnce({
        ok: false,
        json: async () => ({ error: 'bad transcript request' }),
      });

      await expect(widget.requestConversationTranscript('person@example.com')).rejects.toThrow('bad transcript request');
    });
  });

  describe('pre-chat and websocket event handling', () => {
    it('persists pre-chat completion, upgrades the session, and tracks leads', () => {
      const sent: string[] = [];
      const track = vi.fn();
      (globalThis as any).helpin = { track };

      const emailSpy = vi.fn();
      widget.onUserEmailSupplied(emailSpy);
      (widget as any).widgetKey = 'test-key';
      (widget as any).wsConnection = {
        readyState: WebSocket.OPEN,
        send: (payload: string) => sent.push(payload),
        close: vi.fn(),
        onclose: null,
      };

      (widget as any).handlePreChatSubmit({ email: 'lead@example.com', phone: '+1234567890' });

      expect(emailSpy).toHaveBeenCalledWith('lead@example.com');
      expect(localStorage.setItem).toHaveBeenCalledWith('helpin_prechat_test-key', '1');
      expect(track).toHaveBeenCalledWith('lead', { email: 'lead@example.com' });
      expect(sent.map((frame) => JSON.parse(frame))).toContainEqual({
        type: 'session:upgrade',
        data: {
          email: 'lead@example.com',
          phone: '+1234567890',
          source: 'widget_prechat',
        },
      });
      expect((widget as any).currentEmail).toBe('lead@example.com');
    });

    it('upgrades an anonymous restored session when boot user email is provided', async () => {
      const sent: string[] = [];
      (widget as any).wsConnection = {
        readyState: WebSocket.OPEN,
        send: (payload: string) => sent.push(payload),
        close: vi.fn(),
        onclose: null,
      };
      (widget as any).config = {
        key: 'test-key',
        user: { email: 'boot@example.com', name: 'Boot User' },
      };
      (widget as any).widgetKey = 'test-key';

      (widget as any).handleWSMessage({
        type: 'session:joined',
        data: {
          session_token: 'session-1',
          expires_at: new Date(Date.now() + 60_000).toISOString(),
          is_anonymous: true,
          conversations: [],
          messages: [],
        },
      });

      expect(sent.map((frame) => JSON.parse(frame))).toContainEqual({
        type: 'session:upgrade',
        data: {
          email: 'boot@example.com',
          first_name: '',
          last_name: '',
          name: 'Boot User',
          source: 'sdk_identify',
        },
      });
      expect((widget as any).currentEmail).toBe('boot@example.com');
    });

    it('maps escalation system messages to the system role for restored sessions', () => {
      (widget as any).widgetConfig = {
        workspaceId: 'ws_test',
        branding: { primaryColor: '#6366f1' },
        features: {},
      };
      (widget as any).mountContainer = document.createElement('div');

      (widget as any).handleWSMessage({
        type: 'session:joined',
        data: {
          session_token: 'session-1',
          expires_at: new Date(Date.now() + 60_000).toISOString(),
          is_anonymous: true,
          conversations: [
            {
              id: 'conv-1',
              subject: 'Support',
              status: 'open',
              ai_state: 'escalated',
              flow_state: 'waiting_for_human',
              created_at: new Date().toISOString(),
              updated_at: new Date().toISOString(),
            },
          ],
          messages: [
            {
              id: 'msg-1',
              conversation_id: 'conv-1',
              sender_type: 'agent',
              message_type: 'system',
              content: 'Let me connect you with a team member who can help further.',
              created_at: new Date().toISOString(),
            },
          ],
        },
      });

      const latestOptions = (mountWidget as ReturnType<typeof vi.fn>).mock.calls.at(-1)?.[1];
      expect(latestOptions?.messages?.[0]?.role).toBe('system');
      expect(latestOptions?.activeConversation?.aiState).toBe('escalated');
      expect(latestOptions?.activeConversation?.flowState).toBe('waiting_for_human');
    });

    it('marks the active conversation escalated when an escalation event arrives', () => {
      (widget as any).widgetConfig = {
        workspaceId: 'ws_test',
        branding: { primaryColor: '#6366f1' },
        features: {},
      };
      (widget as any).mountContainer = document.createElement('div');
      (widget as any).activeConversationId = 'conv-1';
      (widget as any).conversations = [
        {
          id: 'conv-1',
          subject: 'Support',
          status: 'open',
          aiState: 'pending',
          flowState: 'ai_handling',
          unreadCount: 0,
        },
      ];

      (widget as any).handleWSMessage({
        type: 'conversation:escalated',
        data: {
          conversation_id: 'conv-1',
          flow_state: 'waiting_for_human',
          active_teammate: {
            user_id: 'user-1',
            name: 'Agent One',
            status: 'online',
          },
        },
      });

      const latestOptions = (mountWidget as ReturnType<typeof vi.fn>).mock.calls.at(-1)?.[1];
      expect(latestOptions?.activeConversation?.aiState).toBe('escalated');
      expect(latestOptions?.activeConversation?.flowState).toBe('waiting_for_human');
      expect(latestOptions?.activeConversation?.activeTeammate).toMatchObject({
        userId: 'user-1',
        name: 'Agent One',
        status: 'online',
      });
    });

    it('does not request human escalation when the active conversation is already escalated', () => {
      const sent: string[] = [];
      (widget as any).activeConversationId = 'conv-1';
      (widget as any).conversations = [
        {
          id: 'conv-1',
          subject: 'Support',
          status: 'open',
          aiState: 'escalated',
        },
      ];
      (widget as any).wsConnection = {
        readyState: WebSocket.OPEN,
        send: (payload: string) => sent.push(payload),
        close: vi.fn(),
        onclose: null,
      };

      (widget as any).handleEscalateToHuman();

      expect(sent).toEqual([]);
    });

    it('only sends one human escalation request while one is in flight', () => {
      const sent: string[] = [];
      (widget as any).activeConversationId = 'conv-1';
      (widget as any).conversations = [
        {
          id: 'conv-1',
          subject: 'Support',
          status: 'open',
          aiState: 'pending',
        },
      ];
      (widget as any).wsConnection = {
        readyState: WebSocket.OPEN,
        send: (payload: string) => sent.push(payload),
        close: vi.fn(),
        onclose: null,
      };

      (widget as any).handleEscalateToHuman();
      (widget as any).handleEscalateToHuman();

      expect(sent.map((frame) => JSON.parse(frame))).toEqual([
        { type: 'conversation:escalate', data: {} },
      ]);
    });

    it('refreshes conversation list when the rendered widget switches to messages view', async () => {
      const sent: string[] = [];
      (widget as any).widgetConfig = {
        workspaceId: 'ws_test',
        branding: { primaryColor: '#6366f1' },
        features: {},
      };
      (widget as any).mountContainer = document.createElement('div');
      (widget as any).wsConnection = {
        readyState: WebSocket.OPEN,
        send: (payload: string) => sent.push(payload),
        close: vi.fn(),
        onclose: null,
      };

      (widget as any).render();

      const latestOptions = (mountWidget as ReturnType<typeof vi.fn>).mock.calls.at(-1)?.[1];
      latestOptions.onViewChange('messages');

      expect(sent.map((frame) => JSON.parse(frame))).toContainEqual({
        type: 'conversations:list',
        data: {},
      });
    });

    it('updates cache and normalizes teammate payloads on config:updated', () => {
      (widget as any).widgetKey = 'test-key';
      (widget as any).widgetConfig = {
        workspaceId: 'ws_test',
        branding: { primaryColor: '#6366f1' },
        features: {},
      };
      (widget as any).mountContainer = document.createElement('div');

      (widget as any).handleWSMessage({
        type: 'config:updated',
        data: {
          workspaceId: 'ws_test',
          branding: { primaryColor: '#123456' },
          features: {},
          availableTeammates: [
            { user_id: 'user-1', name: 'Agent One', avatar_url: 'https://example.com/1.png', status: 'online' },
            { user_id: '', name: 'Ignored' },
          ],
        },
      });

      const latestOptions = (mountWidget as ReturnType<typeof vi.fn>).mock.calls.at(-1)?.[1];
      expect(latestOptions?.config?.availableTeammates).toEqual([
        {
          userId: 'user-1',
          name: 'Agent One',
          avatarUrl: 'https://example.com/1.png',
          status: 'online',
        },
      ]);
      expect(localStorage.setItem).toHaveBeenCalledWith(
        'helpin_wc_test-key',
        expect.stringContaining('"primaryColor":"#123456"'),
      );
    });

    it('updates home teammate presence from teammate:presence events', () => {
      (widget as any).widgetConfig = {
        workspaceId: 'ws_test',
        branding: { primaryColor: '#6366f1' },
        features: {},
        availableTeammates: [
          { userId: 'user-1', name: 'Agent One', status: 'offline' },
          { userId: 'user-2', name: 'Agent Two', status: 'away' },
        ],
      };
      (widget as any).mountContainer = document.createElement('div');

      (widget as any).handleWSMessage({
        type: 'teammate:presence',
        data: {
          user_id: 'user-1',
          status: 'online',
        },
      });

      const latestOptions = (mountWidget as ReturnType<typeof vi.fn>).mock.calls.at(-1)?.[1];
      expect(latestOptions?.config?.availableTeammates).toEqual([
        { userId: 'user-1', name: 'Agent One', status: 'online' },
        { userId: 'user-2', name: 'Agent Two', status: 'away' },
      ]);
    });

    it('marks selected conversations read locally and requests their message history', () => {
      const sent: string[] = [];
      (widget as any).widgetConfig = {
        workspaceId: 'ws_test',
        branding: { primaryColor: '#6366f1' },
        features: {},
      };
      (widget as any).mountContainer = document.createElement('div');
      (widget as any).conversations = [
        {
          id: 'conv-1',
          subject: 'Question',
          status: 'open',
          unreadCount: 2,
          activeTeammate: { userId: 'user-1', name: 'Agent One', status: 'online' },
        },
      ];
      (widget as any).wsConnection = {
        readyState: WebSocket.OPEN,
        send: (payload: string) => sent.push(payload),
        close: vi.fn(),
        onclose: null,
      };

      (widget as any).handleSelectConversation('conv-1');

      const latestOptions = (mountWidget as ReturnType<typeof vi.fn>).mock.calls.at(-1)?.[1];
      expect(latestOptions?.activeConversation?.id).toBe('conv-1');
      expect(latestOptions?.conversations?.[0]?.unreadCount).toBe(0);
      expect(sent.map((frame) => JSON.parse(frame))).toContainEqual({
        type: 'conversation:select',
        data: { conversation_id: 'conv-1' },
      });
    });
  });
});
