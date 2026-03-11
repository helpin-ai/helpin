import { test, expect } from '@playwright/test';
import './types';

test.describe('Helpin AMD/jQuery Integration Tests', () => {
  test('should load jQuery and RequireJS environment correctly', async ({
    page,
  }) => {
    // Navigate to the AMD/jQuery test page
    const pageResponse = await page.goto('/test/e2e/amd-jquery-test.html');
    expect(pageResponse?.ok()).toBeTruthy();

    // Wait for page to load
    await page.waitForLoadState('networkidle');

    // Run Test 1 to verify jQuery and RequireJS
    await page.click('button:has-text("Run Test 1")');

    // Wait for test results
    await page.waitForSelector(
      '#test1-results .success:has-text("jQuery loaded")',
    );
    await page.waitForSelector(
      '#test1-results .success:has-text("RequireJS loaded")',
    );

    // Verify jQuery functionality
    const jqueryWorks = await page.isVisible(
      '#test1-results .success:has-text("jQuery DOM manipulation works")',
    );
    expect(jqueryWorks).toBeTruthy();

    // Verify RequireJS module loading
    const requirejsWorks = await page.isVisible(
      '#test1-results .success:has-text("jQuery loaded via RequireJS")',
    );
    expect(requirejsWorks).toBeTruthy();
  });

  test('should load Helpin via AMD module correctly', async ({ page }) => {
    const requests: Array<{ url: string; postData?: string }> = [];

    // Setup request interception
    await page.route('**/api/v1/event**', async (route) => {
      const request = route.request();
      const postData = request.postData() || undefined;
      requests.push({ url: request.url(), postData });
      await route.continue();
    });

    // Navigate to the AMD/jQuery test page
    const pageResponse = await page.goto('/test/e2e/amd-jquery-test.html');
    expect(pageResponse?.ok()).toBeTruthy();

    // Wait for page to load
    await page.waitForLoadState('networkidle');

    // Run Test 3 to test AMD module loading
    await page.click('button:has-text("Run Test 3")');

    // Wait for AMD module to load
    await page.waitForSelector(
      '#test3-results .success:has-text("Helpin AMD module loaded")',
    );

    // Verify helpinClient function is available
    const clientAvailable = await page.isVisible(
      '#test3-results .success:has-text("helpinClient function available")',
    );
    expect(clientAvailable).toBeTruthy();

    // Verify client was created
    const clientCreated = await page.isVisible(
      '#test3-results .success:has-text("Client created via AMD")',
    );
    expect(clientCreated).toBeTruthy();

    // Verify event was tracked
    const eventTracked = await page.isVisible(
      '#test3-results .success:has-text("Event tracked via AMD client")',
    );
    expect(eventTracked).toBeTruthy();

    // Wait for the tracking request to complete
    await page.waitForTimeout(2000);

    // Verify that an AMD test event was sent
    const amdEvent = findEventInRequests(requests, 'amd_test_event');
    expect(amdEvent, 'AMD test event should be captured').toBeTruthy();

    if (amdEvent?.postData) {
      const data = JSON.parse(amdEvent.postData);
      const payload = Array.isArray(data)
        ? data.find((event) => event.event_type === 'amd_test_event')
        : data;

      expect(payload.event_attributes.source).toBe('amd_module');
    }
  });

  test('should handle event tracking in AMD environment', async ({ page }) => {
    const requests: Array<{ url: string; postData?: string }> = [];

    // Setup request interception
    await page.route('**/api/v1/event**', async (route) => {
      const request = route.request();
      const postData = request.postData() || undefined;
      requests.push({ url: request.url(), postData });
      await route.continue();
    });

    // Navigate to the AMD/jQuery test page
    const pageResponse = await page.goto('/test/e2e/amd-jquery-test.html');
    expect(pageResponse?.ok()).toBeTruthy();

    // Wait for page to load
    await page.waitForLoadState('networkidle');

    // Run Test 4 to test event tracking functionality
    await page.click('button:has-text("Run Test 4")');

    // Wait for module to initialize
    await page.waitForSelector(
      '#test4-results .success:has-text("Custom module initialized with jQuery and Helpin")',
    );

    // Verify pageview was tracked
    const pageviewTracked = await page.isVisible(
      '#test4-results .success:has-text("Pageview tracked")',
    );
    expect(pageviewTracked).toBeTruthy();

    // Verify purchase event was tracked
    const purchaseTracked = await page.isVisible(
      '#test4-results .success:has-text("Purchase event tracked")',
    );
    expect(purchaseTracked).toBeTruthy();

    // Verify jQuery event handlers were set up
    const handlersSetup = await page.isVisible(
      '#test4-results .success:has-text("jQuery event handlers with tracking setup")',
    );
    expect(handlersSetup).toBeTruthy();

    // Wait for the tracking requests to complete
    await page.waitForTimeout(2000);

    // Verify that a purchase event was sent
    const purchaseEvent = findEventInRequests(requests, 'purchase');
    expect(purchaseEvent, 'Purchase event should be captured').toBeTruthy();

    if (purchaseEvent?.postData) {
      const data = JSON.parse(purchaseEvent.postData);
      const payload = Array.isArray(data)
        ? data.find((event) => event.event_type === 'purchase')
        : data;

      expect(payload.event_attributes.amount).toBe(99.99);
      expect(payload.event_attributes.currency).toBe('USD');
    }
  });

  test('should not have AMD conflicts with other modules', async ({ page }) => {
    // Navigate to the AMD/jQuery test page
    const pageResponse = await page.goto('/test/e2e/amd-jquery-test.html');
    expect(pageResponse?.ok()).toBeTruthy();

    // Wait for page to load
    await page.waitForLoadState('networkidle');

    // Run Test 5 to test AMD conflict resolution
    await page.click('button:has-text("Run Test 5")');

    // Wait for modules to load
    await page.waitForSelector(
      '#test5-results .success:has-text("All modules loaded successfully")',
    );

    // Verify Module 1 works correctly
    const module1Works = await page.isVisible(
      '#test5-results .success:has-text("Module 1 works correctly")',
    );
    expect(module1Works).toBeTruthy();

    // Verify Module 2 works correctly with jQuery
    const module2Works = await page.isVisible(
      '#test5-results .success:has-text("Module 2 works correctly with jQuery")',
    );
    expect(module2Works).toBeTruthy();

    // Verify Helpin module is intact
    const helpinIntact = await page.isVisible(
      '#test5-results .success:has-text("Helpin module intact")',
    );
    expect(helpinIntact).toBeTruthy();

    // Verify new modules can still be defined
    const canDefineModules = await page.isVisible(
      '#test5-results .success:has-text("Can still define new modules after Helpin load")',
    );
    expect(canDefineModules).toBeTruthy();
  });

  test('should handle script tag initialization in non-AMD environment', async ({
    page,
  }) => {
    // Create a separate test for script tag initialization
    // This test will use a different page without RequireJS to avoid AMD conflicts

    // First, check if we need to create a non-AMD test page
    const nonAmdTestExists = await page.evaluate(async () => {
      try {
        const response = await fetch('/test/e2e/non-amd-jquery-test.html');
        return response.ok;
      } catch (e) {
        return false;
      }
    });

    // If the non-AMD test page doesn't exist, we'll skip this test
    if (!nonAmdTestExists) {
      test.skip(true, 'Non-AMD test page not found, skipping script tag test');
      return;
    }

    const requests: Array<{ url: string; postData?: string }> = [];

    // Setup request interception
    await page.route('**/api/v1/event**', async (route) => {
      const request = route.request();
      const postData = request.postData() || undefined;
      requests.push({ url: request.url(), postData });
      await route.continue();
    });

    // Navigate to the non-AMD test page
    const pageResponse = await page.goto('/test/e2e/non-amd-jquery-test.html');
    expect(pageResponse?.ok()).toBeTruthy();

    // Wait for page to load
    await page.waitForLoadState('networkidle');

    // Run Test 2 to test script tag initialization
    await page.click('button:has-text("Run Test 2")');

    // Wait for script to load
    await page.waitForSelector(
      '#test2-results .success:has-text("Script loaded successfully via script tag")',
    );

    // Verify helpinClient global is available
    const clientAvailable = await page.isVisible(
      '#test2-results .success:has-text("helpinClient global available")',
    );
    expect(clientAvailable).toBeTruthy();

    // Verify event was tracked
    const eventTracked = await page.isVisible(
      '#test2-results .success:has-text("Event tracked via helpinClient")',
    );
    expect(eventTracked).toBeTruthy();

    // Wait for the tracking request to complete
    await page.waitForTimeout(2000);

    // Verify that a script tag test event was sent
    const scriptTagEvent = findEventInRequests(requests, 'non_amd_test_event');
    expect(
      scriptTagEvent,
      'Script tag test event should be captured',
    ).toBeTruthy();

    if (scriptTagEvent?.postData) {
      const data = JSON.parse(scriptTagEvent.postData);
      const payload = Array.isArray(data)
        ? data.find((event) => event.event_type === 'non_amd_test_event')
        : data;

      expect(payload.event_attributes.source).toBe('script_tag');
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
