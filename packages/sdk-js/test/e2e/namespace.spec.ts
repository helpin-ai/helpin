import { test, expect } from '@playwright/test';
import './types';

test.describe('Helpin Namespace Tests', () => {
  test('should load multiple instances with different namespaces', async ({
    page,
  }) => {
    try {
      // Navigate to the namespace test page
      const pageResponse = await page.goto('/test/e2e/namespace-test.html');
      expect(pageResponse?.ok()).toBeTruthy();

      // Wait for network idle state
      await page.waitForLoadState('networkidle');

      // Wait for all namespaces to be initialized
      await page.waitForFunction(
        () => {
          return (
            typeof window.helpin === 'function' &&
            typeof window.analytics === 'function' &&
            typeof window.tracker === 'function'
          );
        },
        { timeout: 10000 },
      );

      // Check if all namespaces are properly loaded
      const namespaceStatus = await page.evaluate(() => {
        return window.testHelpers?.checkNamespaces
          ? window.testHelpers.checkNamespaces()
          : undefined;
      });

      expect(
        namespaceStatus?.default,
        'Default namespace should be loaded',
      ).toBe(true);
      expect(
        namespaceStatus?.analytics,
        'Analytics namespace should be loaded',
      ).toBe(true);
      expect(
        namespaceStatus?.tracker,
        'Tracker namespace should be loaded',
      ).toBe(true);
    } catch (error) {
      console.error('Test failed:', error);
      throw error;
    }
  });

  test('should track events with different namespaces', async ({ page }) => {
    const requests: Array<{ url: string; postData?: string }> = [];

    // Setup request interception before navigation
    await page.route('**/api/v1/event**', async (route) => {
      const request = route.request();
      const postData = request.postData() || undefined;
      requests.push({ url: request.url(), postData });
      await route.continue();
    });

    try {
      // Navigate to the namespace test page
      const pageResponse = await page.goto('/test/e2e/namespace-test.html');
      expect(pageResponse?.ok()).toBeTruthy();

      // Wait for network idle state
      await page.waitForLoadState('networkidle');

      // Wait for all namespaces to be initialized
      await page.waitForFunction(
        () => {
          return (
            typeof window.helpin === 'function' &&
            typeof window.analytics === 'function' &&
            typeof window.tracker === 'function'
          );
        },
        { timeout: 10000 },
      );

      // Wait for initial pageview events (default and analytics should have auto pageview)
      await page.waitForTimeout(2000);

      // Verify initial pageview events
      const pageviewEvents = requests.filter((req) => {
        if (!req.postData) return false;
        try {
          const data = JSON.parse(req.postData);
          const events = Array.isArray(data) ? data : [data];
          return events.some((event) => event.event_type === 'pageview');
        } catch (e) {
          return false;
        }
      });

      // We expect at least 2 pageview events (from default and analytics namespaces)
      expect(pageviewEvents.length).toBeGreaterThanOrEqual(2);

      // Track events with each namespace
      await page.click('button:has-text("Track with Default Namespace")');
      await page.click('button:has-text("Track with Analytics Namespace")');
      await page.click('button:has-text("Track with Tracker Namespace")');

      // Wait for events to be processed
      await page.waitForTimeout(3000);

      // Verify events were tracked in the UI
      const eventCounts = await page.evaluate(() => {
        return window.testHelpers?.getEventCounts
          ? window.testHelpers.getEventCounts()
          : undefined;
      });

      expect(
        eventCounts?.default,
        'Default namespace events should be tracked',
      ).toBeGreaterThan(0);
      expect(
        eventCounts?.analytics,
        'Analytics namespace events should be tracked',
      ).toBeGreaterThan(0);
      expect(
        eventCounts?.tracker,
        'Tracker namespace events should be tracked',
      ).toBeGreaterThan(0);

      // Verify custom events were sent to the server
      const customEvents = {
        default: findEventInRequests(requests, 'default_namespace_event'),
        analytics: findEventInRequests(requests, 'analytics_namespace_event'),
        tracker: findEventInRequests(requests, 'tracker_namespace_event'),
      };

      expect(
        customEvents.default,
        'Default namespace event should be captured',
      ).toBeTruthy();
      expect(
        customEvents.analytics,
        'Analytics namespace event should be captured',
      ).toBeTruthy();
      expect(
        customEvents.tracker,
        'Tracker namespace event should be captured',
      ).toBeTruthy();

      // Verify each event has the correct source property
      if (customEvents.default?.postData) {
        const data = JSON.parse(customEvents.default.postData);
        const payload = Array.isArray(data)
          ? data.find((event) => event.event_type === 'default_namespace_event')
          : data;

        expect(payload.event_attributes.source).toBe('default');
      }

      if (customEvents.analytics?.postData) {
        const data = JSON.parse(customEvents.analytics.postData);
        const payload = Array.isArray(data)
          ? data.find(
              (event) => event.event_type === 'analytics_namespace_event',
            )
          : data;

        expect(payload.event_attributes.source).toBe('analytics');
      }

      if (customEvents.tracker?.postData) {
        const data = JSON.parse(customEvents.tracker.postData);
        const payload = Array.isArray(data)
          ? data.find((event) => event.event_type === 'tracker_namespace_event')
          : data;

        expect(payload.event_attributes.source).toBe('tracker');
      }
    } catch (error) {
      console.error('Test failed:', error);
      console.log('Captured requests:', JSON.stringify(requests, null, 2));
      throw error;
    }
  });

  test('should handle auto-pageview settings correctly for each namespace', async ({
    page,
  }) => {
    const requests: Array<{ url: string; postData?: string }> = [];

    // Setup request interception before navigation
    await page.route('**/api/v1/event**', async (route) => {
      const request = route.request();
      const postData = request.postData() || undefined;
      requests.push({ url: request.url(), postData });
      await route.continue();
    });

    try {
      // Navigate to the namespace test page
      const pageResponse = await page.goto('/test/e2e/namespace-test.html');
      expect(pageResponse?.ok()).toBeTruthy();

      // Wait for network idle state
      await page.waitForLoadState('networkidle');

      // Wait for all namespaces to be initialized
      await page.waitForFunction(
        () => {
          return (
            typeof window.helpin === 'function' &&
            typeof window.analytics === 'function' &&
            typeof window.tracker === 'function'
          );
        },
        { timeout: 10000 },
      );

      // Wait for initial pageview events
      await page.waitForTimeout(2000);

      // Count pageview events by namespace
      const pageviewEvents = requests.filter((req) => {
        if (!req.postData) return false;
        try {
          const data = JSON.parse(req.postData);
          const events = Array.isArray(data) ? data : [data];
          return events.some((event) => event.event_type === 'pageview');
        } catch (e) {
          return false;
        }
      });

      // Extract namespace from each pageview event
      const pageviewNamespaces = pageviewEvents
        .map((req) => {
          if (!req.postData) return null;
          try {
            const data = JSON.parse(req.postData);
            const events = Array.isArray(data) ? data : [data];
            const pageviewEvent = events.find(
              (event) => event.event_type === 'pageview',
            );
            return pageviewEvent?.namespace || 'default';
          } catch (e) {
            return null;
          }
        })
        .filter(Boolean);

      // Check if all namespaces sent pageviews (current SDK behavior sends pageviews for all namespaces)
      expect(pageviewNamespaces).toContain('helpin');
      expect(pageviewNamespaces).toContain('analytics');
      expect(pageviewNamespaces).toContain('tracker');

      // All 3 namespaces should send pageviews with current SDK implementation
      expect(pageviewEvents.length).toBe(3);

      // Now manually trigger a pageview with the tracker namespace
      await page.evaluate(() => {
        if (typeof window.tracker === 'function') {
          window.tracker('pageview');
        }
      });

      // Wait for the event to be processed
      await page.waitForTimeout(2000);

      // Verify that now we have 4 pageview events (3 automatic + 1 manual)
      const updatedPageviewEvents = requests.filter((req) => {
        if (!req.postData) return false;
        try {
          const data = JSON.parse(req.postData);
          const events = Array.isArray(data) ? data : [data];
          return events.some((event) => event.event_type === 'pageview');
        } catch (e) {
          return false;
        }
      });

      expect(updatedPageviewEvents.length).toBe(4);

      // Verify there's a pageview from the tracker namespace
      const trackerPageview = updatedPageviewEvents.find((req) => {
        if (!req.postData) return false;
        try {
          const data = JSON.parse(req.postData);
          const events = Array.isArray(data) ? data : [data];
          const pageviewEvent = events.find(
            (event) => event.event_type === 'pageview',
          );
          return pageviewEvent?.namespace === 'tracker';
        } catch (e) {
          return false;
        }
      });

      expect(
        trackerPageview,
        'Should have a pageview from tracker namespace',
      ).toBeTruthy();
    } catch (error) {
      console.error('Test failed:', error);
      console.log('Captured requests:', JSON.stringify(requests, null, 2));
      throw error;
    }
  });
});

// Helper function to find an event in the captured requests
function findEventInRequests(
  requests: Array<{ url: string; postData?: string }>,
  eventType: string,
) {
  return requests.find((req) => {
    if (!req.postData) return false;
    try {
      const data = JSON.parse(req.postData);
      const events = Array.isArray(data) ? data : [data];
      return events.some((event) => event.event_type === eventType);
    } catch (e) {
      return false;
    }
  });
}
