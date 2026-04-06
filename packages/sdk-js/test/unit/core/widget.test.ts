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

  beforeEach(() => {
    widget = new WidgetManager();
    document.body.innerHTML = '';
    MockWebSocket.reset();
    localStorage.clear();

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

  describe('show/hide', () => {
    it('should create widget element when shown', async () => {
      widget.boot({ key: 'test-key' });
      await new Promise((r) => setTimeout(r, 100));
      
      widget.show();
      
      expect(document.getElementById('helpin-widget-container')).not.toBeNull();
    });

    it('should call onShow callback', async () => {
      widget.boot({ key: 'test-key' });
      await new Promise((r) => setTimeout(r, 100));
      
      const callback = vi.fn();
      widget.onShow(callback);
      widget.show();
      expect(callback).toHaveBeenCalled();
    });

    it('should call onHide callback', async () => {
      widget.boot({ key: 'test-key' });
      await new Promise((r) => setTimeout(r, 100));
      
      const callback = vi.fn();
      widget.onHide(callback);
      widget.show();
      widget.hide();
      expect(callback).toHaveBeenCalled();
    });

    it('should toggle visibility via mountWidget', async () => {
      widget.boot({ key: 'test-key' });
      await new Promise((r) => setTimeout(r, 100));

      const mockMount = mountWidget as ReturnType<typeof vi.fn>;
      mockMount.mockClear();

      widget.show();
      expect(mockMount).toHaveBeenCalled();
      const showCall = mockMount.mock.calls[mockMount.mock.calls.length - 1];
      expect(showCall[1].isOpen).toBe(true);

      mockMount.mockClear();
      widget.hide();
      expect(mockMount).toHaveBeenCalled();
      const hideCall = mockMount.mock.calls[mockMount.mock.calls.length - 1];
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
    it('should have showMessages method', async () => {
      widget.boot({ key: 'test-key' });
      await new Promise((r) => setTimeout(r, 100));
      
      expect(() => widget.showMessages()).not.toThrow();
    });

    it('should have showConversation method', async () => {
      widget.boot({ key: 'test-key' });
      await new Promise((r) => setTimeout(r, 100));
      
      expect(() => widget.showConversation('conv-123')).not.toThrow();
    });

    it('should have showArticle method', async () => {
      widget.boot({ key: 'test-key' });
      await new Promise((r) => setTimeout(r, 100));

      const mockMount = mountWidget as ReturnType<typeof vi.fn>;
      mockMount.mockClear();

      widget.showArticle('article-123');

      expect(mockMount).toHaveBeenCalled();
      const latestOptions = mockMount.mock.calls[mockMount.mock.calls.length - 1][1];
      expect(latestOptions.initialView).toBe('help-article');
      expect(latestOptions.openArticleRequest).toEqual({
        key: 1,
        articleSlug: 'article-123',
      });
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
              created_at: new Date().toISOString(),
            },
          ],
        },
      });

      const latestOptions = (mountWidget as ReturnType<typeof vi.fn>).mock.calls.at(-1)?.[1];
      expect(latestOptions?.messages?.[0]?.viaChannel).toBe('email');
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
          name: 'Boot User',
          source: 'sdk_identify',
        },
      });
      expect((widget as any).currentEmail).toBe('boot@example.com');
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
