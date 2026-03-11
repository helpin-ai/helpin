import { test, expect } from '@playwright/test';
import './types';

test.describe('Helpin AMD Support Tests', () => {
  test('should load and initialize in AMD environment', async ({ page }) => {
    try {
      // Navigate to the AMD test page
      const pageResponse = await page.goto('/test/e2e/amd-test.html');
      expect(pageResponse?.ok()).toBeTruthy();

      // Wait for network idle state
      await page.waitForLoadState('networkidle');

      // Wait for RequireJS to load and initialize Helpin
      await page.waitForFunction(
        () => {
          return window.testResults && window.testResults.amdLoaded;
        },
        { timeout: 10000 },
      );

      // Get test results from the page
      const testResults = await page.evaluate(() => window.testResults);

      // Verify AMD module loaded correctly
      expect(testResults?.amdLoaded, 'AMD module should be loaded').toBe(true);

      // Verify client instance was created
      expect(
        testResults?.clientCreated,
        'Client instance should be created',
      ).toBe(true);

      // Verify tracking worked
      expect(testResults?.trackingWorked, 'Event tracking should work').toBe(
        true,
      );

      // Verify no errors occurred
      expect(testResults?.errors.length, 'There should be no errors').toBe(0);
    } catch (error) {
      console.error('Test failed:', error);
      throw error;
    }
  });

  test('should track events through AMD-loaded client', async ({ page }) => {
    const requests: Array<{ url: string; postData?: string }> = [];

    // Setup request interception before navigation
    await page.route('**/api/v1/event**', async (route) => {
      const request = route.request();
      const postData = request.postData() || undefined;
      requests.push({ url: request.url(), postData });
      await route.continue();
    });

    try {
      // Navigate to the AMD test page
      const pageResponse = await page.goto('/test/e2e/amd-test.html');
      expect(pageResponse?.ok()).toBeTruthy();

      // Wait for network idle state
      await page.waitForLoadState('networkidle');

      // Wait for AMD module to load
      await page.waitForFunction(
        () => {
          return window.testResults && window.testResults.amdLoaded;
        },
        { timeout: 10000 },
      );

      // Click the manual test button to trigger an event
      await page.click('button');

      // Wait for the event to be processed
      await page.waitForTimeout(3000);

      // Find the AMD test event in requests
      const amdTestEvent = requests.find((req) => {
        if (!req.postData) return false;
        try {
          const data = JSON.parse(req.postData);
          const events = Array.isArray(data) ? data : [data];
          return events.some(
            (event) =>
              event.event_type === 'manual_amd_test' ||
              event.event_type === 'amd_test_event',
          );
        } catch (e) {
          return false;
        }
      });

      expect(amdTestEvent, 'AMD test event should be captured').toBeTruthy();

      if (amdTestEvent?.postData) {
        const data = JSON.parse(amdTestEvent.postData);
        const payload = Array.isArray(data)
          ? data.find(
              (event) =>
                event.event_type === 'manual_amd_test' ||
                event.event_type === 'amd_test_event',
            )
          : data;

        expect(payload).toBeTruthy();
        expect(['manual_amd_test', 'amd_test_event']).toContain(
          payload.event_type,
        );
      }
    } catch (error) {
      console.error('Test failed:', error);
      console.log('Captured requests:', JSON.stringify(requests, null, 2));
      throw error;
    }
  });
});
