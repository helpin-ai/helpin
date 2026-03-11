import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { WidgetManager } from '../../../src/core/widget';

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

    it('should toggle visibility', async () => {
      widget.boot({ key: 'test-key' });
      await new Promise((r) => setTimeout(r, 100));
      
      widget.show();
      const chatWindow = document.querySelector('.helpin-chat-window') as HTMLElement;
      expect(chatWindow.style.display).not.toBe('none');
      
      widget.hide();
      const chatWindowHidden = document.querySelector('.helpin-chat-window') as HTMLElement;
      expect(chatWindowHidden.style.display).toBe('none');
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
});
