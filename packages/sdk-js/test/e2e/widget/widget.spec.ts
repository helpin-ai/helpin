import { test, expect, type Page } from '@playwright/test';
import { startServer, stopServer } from './testServer';

const WIDGET_KEY = 'test-widget-key';
const API_HOST = 'http://localhost:3456';
const WS_HOST = 'ws://localhost:3457';

test.describe('Widget E2E Tests', () => {
  test.beforeAll(async () => {
    await startServer();
  });

  test.afterAll(async () => {
    await stopServer();
  });

  test.describe('Widget Boot & Initialization', () => {
    test('should boot widget with key only', async ({ page }) => {
      await page.goto('/test/e2e/widget/test-page.html');
      
      await page.evaluate((config) => {
        (window as any).helpin('boot', config);
      }, { key: WIDGET_KEY, host: 'localhost:3456' });

      await page.waitForTimeout(500);

      const isReady = await page.evaluate(() => {
        return (window as any).helpin('isWidgetReady');
      });
      
      expect(isReady).toBe(true);
    });

    test('should boot with user info', async ({ page }) => {
      await page.goto('/test/e2e/widget/test-page.html');
      
      await page.evaluate((config) => {
        (window as any).helpin('boot', {
          ...config,
          user: { email: 'test@example.com', name: 'Test User' }
        });
      }, { key: WIDGET_KEY, host: 'localhost:3456' });

      await page.waitForTimeout(500);

      const isReady = await page.evaluate(() => {
        return (window as any).helpin('isWidgetReady');
      });

      expect(isReady).toBe(true);
    });

    test('should throw error without widget key', async ({ page }) => {
      await page.goto('/test/e2e/widget/test-page.html');
      
      const errors: string[] = [];
      page.on('pageerror', (err) => errors.push(err.message));

      await page.evaluate(() => {
        (window as any).helpin('boot', { host: 'localhost:3456' });
      });

      await page.waitForTimeout(200);
      expect(errors.some(e => e.includes('key'))).toBe(true);
    });
  });

  test.describe('Widget Show/Hide', () => {
    test('should show widget when show() is called', async ({ page }) => {
      await page.goto('/test/e2e/widget/test-page.html');
      
      await page.evaluate((config) => {
        (window as any).helpin('boot', config);
      }, { key: WIDGET_KEY, host: 'localhost:3456' });

      await page.waitForTimeout(500);

      await page.evaluate(() => {
        (window as any).helpin('show');
      });

      const isVisible = await page.evaluate(() => {
        const chatWindow = document.querySelector('.helpin-chat-window') as HTMLElement;
        return chatWindow && chatWindow.style.display !== 'none';
      });

      expect(isVisible).toBe(true);
    });

    test('should hide widget when hide() is called', async ({ page }) => {
      await page.goto('/test/e2e/widget/test-page.html');
      
      await page.evaluate((config) => {
        (window as any).helpin('boot', config);
      }, { key: WIDGET_KEY, host: 'localhost:3456' });

      await page.waitForTimeout(500);

      await page.evaluate(() => {
        (window as any).helpin('show');
      });

      await page.evaluate(() => {
        (window as any).helpin('hide');
      });

      const isHidden = await page.evaluate(() => {
        const chatWindow = document.querySelector('.helpin-chat-window') as HTMLElement;
        return chatWindow && chatWindow.style.display === 'none';
      });

      expect(isHidden).toBe(true);
    });

    test('should toggle widget on toggle()', async ({ page }) => {
      await page.goto('/test/e2e/widget/test-page.html');
      
      await page.evaluate((config) => {
        (window as any).helpin('boot', config);
      }, { key: WIDGET_KEY, host: 'localhost:3456' });

      await page.waitForTimeout(500);

      const isOpenInitial = await page.evaluate(() => {
        const chatWindow = document.querySelector('.helpin-chat-window') as HTMLElement;
        return chatWindow && chatWindow.style.display !== 'none';
      });

      if (!isOpenInitial) {
        await page.evaluate(() => {
          (window as any).helpin('show');
        });
      }

      await page.evaluate(() => {
        (window as any).helpin('toggle');
      });

      const isOpenAfterToggle = await page.evaluate(() => {
        const chatWindow = document.querySelector('.helpin-chat-window') as HTMLElement;
        return chatWindow && chatWindow.style.display !== 'none';
      });

      expect(isOpenAfterToggle).toBe(false);
    });
  });

  test.describe('Callbacks', () => {
    test('should call onShow callback', async ({ page }) => {
      await page.goto('/test/e2e/widget/test-page.html');
      
      let callbackCalled = false;
      await page.exposeBinding('onShowCallback', () => {
        callbackCalled = true;
      });

      await page.evaluate(() => {
        (window as any).helpin('boot', { key: 'test-widget-key', host: 'localhost:3456' });
        (window as any).helpin('onShow', (window as any).onShowCallback);
        (window as any).helpin('show');
      });

      await page.waitForTimeout(300);
      expect(callbackCalled).toBe(true);
    });

    test('should call onHide callback', async ({ page }) => {
      await page.goto('/test/e2e/widget/test-page.html');
      
      let callbackCalled = false;
      await page.exposeBinding('onHideCallback', () => {
        callbackCalled = true;
      });

      await page.evaluate(() => {
        (window as any).helpin('boot', { key: 'test-widget-key', host: 'localhost:3456' });
        (window as any).helpin('show');
        (window as any).helpin('onHide', (window as any).onHideCallback);
        (window as any).helpin('hide');
      });

      await page.waitForTimeout(300);
      expect(callbackCalled).toBe(true);
    });

    test('should call onUnreadCountChange callback', async ({ page }) => {
      await page.goto('/test/e2e/widget/test-page.html');
      
      let receivedCount = -1;
      await page.exposeBinding('unreadCallback', (count: number) => {
        receivedCount = count;
      });

      await page.evaluate(() => {
        (window as any).helpin('boot', { key: 'test-widget-key', host: 'localhost:3456' });
        (window as any).helpin('onUnreadCountChange', (window as any).unreadCallback);
      });

      await page.waitForTimeout(300);
      expect(receivedCount).toBe(0);
    });

    test('should call onUserEmailSupplied callback', async ({ page }) => {
      await page.goto('/test/e2e/widget/test-page.html');
      
      let receivedEmail = '';
      await page.exposeBinding('emailCallback', (email: string) => {
        receivedEmail = email;
      });

      await page.evaluate(() => {
        (window as any).helpin('boot', { 
          key: 'test-widget-key', 
          host: 'localhost:3456',
          user: { email: 'callback@test.com' }
        });
        (window as any).helpin('onUserEmailSupplied', (window as any).emailCallback);
      });

      await page.waitForTimeout(500);
      expect(receivedEmail).toBe('callback@test.com');
    });
  });

  test.describe('Shutdown', () => {
    test('should remove widget elements on shutdown', async ({ page }) => {
      await page.goto('/test/e2e/widget/test-page.html');
      
      await page.evaluate((config) => {
        (window as any).helpin('boot', config);
      }, { key: WIDGET_KEY, host: 'localhost:3456' });

      await page.waitForTimeout(500);

      await page.evaluate(() => {
        (window as any).helpin('shutdown');
      });

      const containerExists = await page.evaluate(() => {
        return document.getElementById('helpin-widget-container') !== null;
      });

      expect(containerExists).toBe(false);
    });
  });

  test.describe('Launcher', () => {
    test('should open chat when launcher is clicked', async ({ page }) => {
      await page.goto('/test/e2e/widget/test-page.html');
      
      await page.evaluate((config) => {
        (window as any).helpin('boot', config);
      }, { key: WIDGET_KEY, host: 'localhost:3456' });

      await page.waitForTimeout(500);

      await page.click('.helpin-launcher');

      const isOpen = await page.evaluate(() => {
        const chatWindow = document.querySelector('.helpin-chat-window') as HTMLElement;
        return chatWindow && chatWindow.style.display !== 'none';
      });

      expect(isOpen).toBe(true);
    });

    test('should close chat when close button is clicked', async ({ page }) => {
      await page.goto('/test/e2e/widget/test-page.html');
      
      await page.evaluate((config) => {
        (window as any).helpin('boot', config);
      }, { key: WIDGET_KEY, host: 'localhost:3456' });

      await page.waitForTimeout(500);

      await page.click('.helpin-launcher');
      await page.click('.helpin-header-close');

      const isClosed = await page.evaluate(() => {
        const chatWindow = document.querySelector('.helpin-chat-window') as HTMLElement;
        return chatWindow && chatWindow.style.display === 'none';
      });

      expect(isClosed).toBe(true);
    });

    test('should show unread badge with count', async ({ page }) => {
      await page.goto('/test/e2e/widget/test-page.html');
      
      await page.evaluate((config) => {
        (window as any).helpin('boot', config);
      }, { key: WIDGET_KEY, host: 'localhost:3456', user: { email: 'test@test.com' } });

      await page.waitForTimeout(500);
      await page.evaluate(() => {
        (window as any).helpin('show');
      });

      const hasBadge = await page.evaluate(() => {
        const badge = document.querySelector('.helpin-unread-badge');
        return badge !== null;
      });

      expect(hasBadge).toBe(true);
    });
  });

  test.describe('Widget UI Elements', () => {
    test('should render header with workspace name', async ({ page }) => {
      await page.goto('/test/e2e/widget/test-page.html');
      
      await page.evaluate((config) => {
        (window as any).helpin('boot', config);
      }, { key: WIDGET_KEY, host: 'localhost:3456' });

      await page.waitForTimeout(500);
      await page.evaluate(() => {
        (window as any).helpin('show');
      });

      const headerText = await page.textContent('.helpin-header-title');
      expect(headerText).toContain('Support');
    });

    test('should render pre-chat form', async ({ page }) => {
      await page.goto('/test/e2e/widget/test-page.html');
      
      await page.evaluate((config) => {
        (window as any).helpin('boot', config);
      }, { key: WIDGET_KEY, host: 'localhost:3456' });

      await page.waitForTimeout(500);
      await page.evaluate(() => {
        (window as any).helpin('show');
      });

      const hasForm = await page.evaluate(() => {
        return document.querySelector('.helpin-pre-chat-form') !== null;
      });

      expect(hasForm).toBe(true);
    });

    test('should have email input in pre-chat form', async ({ page }) => {
      await page.goto('/test/e2e/widget/test-page.html');
      
      await page.evaluate((config) => {
        (window as any).helpin('boot', config);
      }, { key: WIDGET_KEY, host: 'localhost:3456' });

      await page.waitForTimeout(500);
      await page.evaluate(() => {
        (window as any).helpin('show');
      });

      const inputType = await page.getAttribute('input[type="email"]', 'type');
      expect(inputType).toBe('email');
    });

    test('should render compose bar when chat is active', async ({ page }) => {
      await page.goto('/test/e2e/widget/test-page.html');
      
      await page.evaluate((config) => {
        (window as any).helpin('boot', config);
      }, { key: WIDGET_KEY, host: 'localhost:3456', user: { email: 'test@test.com' } });

      await page.waitForTimeout(500);
      await page.evaluate(() => {
        (window as any).helpin('show');
      });

      const hasComposeBar = await page.evaluate(() => {
        return document.querySelector('.helpin-compose-bar') !== null;
      });

      expect(hasComposeBar).toBe(true);
    });
  });

  test.describe('Mobile Responsive', () => {
    test('should adjust layout on mobile viewport', async ({ page }) => {
      await page.setViewportSize({ width: 375, height: 667 });
      await page.goto('/test/e2e/widget/test-page.html');
      
      await page.evaluate((config) => {
        (window as any).helpin('boot', config);
      }, { key: WIDGET_KEY, host: 'localhost:3456', user: { email: 'test@test.com' } });

      await page.waitForTimeout(500);
      await page.evaluate(() => {
        (window as any).helpin('show');
      });

      const windowWidth = await page.evaluate(() => {
        const chatWindow = document.querySelector('.helpin-chat-window') as HTMLElement;
        return chatWindow ? chatWindow.offsetWidth : 0;
      });

      expect(windowWidth).toBeLessThanOrEqual(400);
    });
  });

  test.describe('Widget Configuration', () => {
    test('should use custom brand color', async ({ page }) => {
      await page.goto('/test/e2e/widget/test-page.html');
      
      await page.evaluate((config) => {
        (window as any).helpin('boot', config);
      }, { key: WIDGET_KEY, host: 'localhost:3456' });

      await page.waitForTimeout(500);

      const brandColor = await page.evaluate(() => {
        const launcher = document.querySelector('.helpin-launcher') as HTMLElement;
        return launcher ? launcher.style.backgroundColor : '';
      });

      expect(brandColor).toBe('rgb(99, 102, 241)');
    });

    test('should position widget at bottom-right by default', async ({ page }) => {
      await page.goto('/test/e2e/widget/test-page.html');
      
      await page.evaluate((config) => {
        (window as any).helpin('boot', config);
      }, { key: WIDGET_KEY, host: 'localhost:3456' });

      await page.waitForTimeout(500);
      await page.evaluate(() => {
        (window as any).helpin('show');
      });

      const position = await page.evaluate(() => {
        const chatWindow = document.querySelector('.helpin-chat-window') as HTMLElement;
        const style = chatWindow ? window.getComputedStyle(chatWindow) : null;
        return style ? style.right : '';
      });

      expect(position).toBe('20px');
    });
  });
});
