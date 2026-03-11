import { test, expect } from '@playwright/test';
import { HelpinGlobal } from '../../src/core/types';

declare global {
  interface Window {
    helpin?: HelpinGlobal;
    helpinQ?: any[];
  }
}

test.describe('Helpin Pixel Tests', () => {
  test('should load pixel script correctly', async ({ page }) => {
    try {
      // Add initial wait for stability
      await page.waitForTimeout(1000);

      // Navigate to the test page
      const pageResponse = await page.goto('/test/e2e/test.html');
      expect(pageResponse?.ok()).toBeTruthy();

      // Wait for network idle state
      await page.waitForLoadState('networkidle');

      // Check for script load with correct attributes
      const script = await page.waitForSelector(
        'script#um-tracker[data-key="UMaugVPOWz"]',
        { state: 'attached', timeout: 10000 },
      );
      expect(script).toBeTruthy();

      // Verify page title
      const title = await page.title();
      expect(title).toBe('Helpin Pixel Test Page');
    } catch (error) {
      console.error('Test failed:', error);
      throw error;
    }
  });

  test('should fire pageview event', async ({ page }) => {
    const requests: Array<{ url: string; postData?: string }> = [];

    // Setup request interception before navigation
    await page.route('**/api/v1/event**', async (route) => {
      const request = route.request();
      const postData = request.postData() || undefined;
      requests.push({ url: request.url(), postData });
      await route.continue();
    });

    try {
      // Navigate to the test page
      const pageResponse = await page.goto('/test/e2e/test.html');
      expect(pageResponse?.ok()).toBeTruthy();

      // Wait for script to be loaded and initialized
      await page.waitForSelector('script#um-tracker[data-key="UMaugVPOWz"]', {
        state: 'attached',
      });
      await page.waitForFunction(
        () => window.helpin && typeof window.helpin === 'function',
      );

      // Wait for network idle
      await page.waitForLoadState('networkidle');

      // Wait for the pageview event
      const pageviewEvent = requests.find((req) => {
        if (!req.postData) return false;
        try {
          const data = JSON.parse(req.postData);
          const events = Array.isArray(data) ? data : [data];
          return events.some((event) => event.event_type === 'pageview');
        } catch (e) {
          return false;
        }
      });

      expect(pageviewEvent, 'Pageview event should be captured').toBeTruthy();

      // Verify the request was intercepted
      expect(requests.length).toBeGreaterThan(0);

      // Verify the request URL contains the correct token
      const eventRequest = requests[0];
      expect(eventRequest.url).toContain('/api/v1/event?token=UMaugVPOWz');

      // Verify the request payload
      if (eventRequest.postData) {
        const data = JSON.parse(eventRequest.postData);
        const payload = Array.isArray(data)
          ? data.find((event) => event.event_type === 'pageview')
          : data;
        expect(payload).toMatchObject({
          event_type: 'pageview',
          src: 'helpin',
          api_key: 'UMaugVPOWz',
          page_title: 'Helpin Pixel Test Page',
        });
      }
    } catch (error) {
      console.error('Test failed:', error);
      throw error;
    }
  });

  test('should fire custom event', async ({ page }) => {
    const requests: Array<{ url: string; postData?: string }> = [];

    // Setup request interception before navigation
    await page.route('**/api/v1/event**', async (route) => {
      const request = route.request();
      const postData = request.postData() || undefined;
      requests.push({ url: request.url(), postData });
      await route.continue();
    });

    try {
      // Navigate to the test page
      const pageResponse = await page.goto('/test/e2e/test.html');
      expect(pageResponse?.ok()).toBeTruthy();

      // Wait for script to be loaded and initialized
      await page.waitForSelector('script#um-tracker[data-key="UMaugVPOWz"]', {
        state: 'attached',
      });
      await page.waitForFunction(
        () => window.helpin && typeof window.helpin === 'function',
      );

      // Wait for network idle
      await page.waitForLoadState('networkidle');

      // Click the test button
      const button = await page.waitForSelector('#testButton');
      await button.click();

      // Wait longer for the event to be captured and processed
      await page.waitForTimeout(3000);

      // Verify the event was captured
      const buttonClickEvent = requests.find((req) => {
        if (!req.postData) return false;
        try {
          const data = JSON.parse(req.postData);
          const events = Array.isArray(data) ? data : [data];
          return events.some((event) => event.event_type === 'button_click');
        } catch (e) {
          return false;
        }
      });

      expect(
        buttonClickEvent,
        'Button click event should be captured',
      ).toBeTruthy();
      if (buttonClickEvent?.postData) {
        const data = JSON.parse(buttonClickEvent.postData);
        const payload = Array.isArray(data)
          ? data.find((event) => event.event_type === 'button_click')
          : data;
        expect(payload).toMatchObject({
          event_type: 'button_click',
          src: 'helpin',
          api_key: 'UMaugVPOWz',
        });
      }
    } catch (error) {
      console.error('Test failed:', error);
      console.log('Captured requests:', JSON.stringify(requests, null, 2));
      throw error;
    }
  });

  test('should capture autocapture events', async ({ page }) => {
    const requests: Array<{ url: string; postData?: string }> = [];

    // Setup request interception before navigation
    await page.route('**/api/v1/event**', async (route) => {
      const request = route.request();
      const postData = request.postData() || undefined;
      requests.push({ url: request.url(), postData });
      await route.continue();
    });

    try {
      // Navigate to the test page
      const pageResponse = await page.goto('/test/e2e/test.html');
      expect(pageResponse?.ok()).toBeTruthy();

      // Wait for script to be loaded and initialized
      await page.waitForSelector('script#um-tracker[data-key="UMaugVPOWz"]', {
        state: 'attached',
      });
      await page.waitForFunction(
        () => window.helpin && typeof window.helpin === 'function',
      );

      // Wait for network idle
      await page.waitForLoadState('networkidle');

      // Click the autocapture button
      const autoButton = await page.waitForSelector('#test-auto-capture');
      await autoButton.click();

      // Wait longer for the event to be captured and processed
      await page.waitForTimeout(3000);

      // Find the autocapture click event in requests
      const autoClickEvent = requests.find((req) => {
        if (!req.postData) return false;
        try {
          const data = JSON.parse(req.postData);
          const events = Array.isArray(data) ? data : [data];
          return events.some((event) => event.event_type === '$autocapture');
        } catch (e) {
          return false;
        }
      });

      expect(
        autoClickEvent,
        'Autocapture event should be captured',
      ).toBeTruthy();
      if (autoClickEvent?.postData) {
        const data = JSON.parse(autoClickEvent.postData);
        const payload = Array.isArray(data)
          ? data.find((event) => event.event_type === '$autocapture')
          : data;
        expect(payload).toMatchObject({
          event_type: '$autocapture',
          src: 'helpin',
          api_key: 'UMaugVPOWz',
          autocapture_attributes: {
            tag_name: 'button',
            event_type: 'click',
          },
        });
      }
    } catch (error) {
      console.error('Test failed:', error);
      console.log('Captured requests:', JSON.stringify(requests, null, 2));
      throw error;
    }
  });
});
