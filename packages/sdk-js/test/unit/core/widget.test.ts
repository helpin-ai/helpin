import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { WidgetManager } from '../../../src/core/widget';
import { mountWidget } from '@helpin/widget-core';

describe('WidgetManager', () => {
  let widget: WidgetManager;

  beforeEach(() => {
    widget = new WidgetManager();
    document.body.innerHTML = '';
    localStorage.clear();
    
    vi.stubGlobal('fetch', vi.fn(() =>
      Promise.resolve({
        ok: true,
        json: () => Promise.resolve({
          workspaceId: 'ws_test',
          branding: { primaryColor: '#6366f1' },
          features: {}
        }),
      })
    ));
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
      
      expect(() => widget.showArticle('article-123')).not.toThrow();
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
  });
});
