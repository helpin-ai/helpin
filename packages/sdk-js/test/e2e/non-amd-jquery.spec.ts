import { test, expect } from '@playwright/test';
import './types';

test.describe('Helpin Non-AMD/jQuery Integration Tests', () => {
  test('should load jQuery correctly', async ({ page }) => {
    // Navigate to the non-AMD/jQuery test page
    const pageResponse = await page.goto('/test/e2e/non-amd-jquery-test.html');
    expect(pageResponse?.ok()).toBeTruthy();

    // Wait for page to load
    await page.waitForLoadState('networkidle');

    // Run Test 1 to verify jQuery
    await page.click('button:has-text("Run Test 1")');

    // Wait for test results
    await page.waitForSelector(
      '#test1-results .success:has-text("jQuery loaded")',
    );

    // Verify jQuery functionality
    const jqueryWorks = await page.isVisible(
      '#test1-results .success:has-text("DOM manipulation works")',
    );
    expect(jqueryWorks).toBeTruthy();
  });

  test('should load Helpin via script tag correctly', async ({ page }) => {
    const requests: Array<{ url: string; postData?: string }> = [];

    // Setup request interception
    await page.route('**/api/v1/event**', async (route) => {
      const request = route.request();
      const postData = request.postData() || undefined;
      requests.push({ url: request.url(), postData });
      await route.continue();
    });

    // Navigate to the non-AMD/jQuery test page
    const pageResponse = await page.goto('/test/e2e/non-amd-jquery-test.html');
    expect(pageResponse?.ok()).toBeTruthy();

    // Wait for page to load
    await page.waitForLoadState('networkidle');

    // Run Test 2 to test script tag initialization
    await page.click('button:has-text("Run Test 2")');

    // Wait for script to load with a longer timeout
    await page.waitForSelector(
      '#test2-results .success:has-text("Script loaded successfully")',
      { timeout: 10000 },
    );

    // Wait a bit for the script to fully initialize
    await page.waitForTimeout(2000);

    // Check if the success message for helpinClient is present
    const successMessages = await page.$$eval(
      '#test2-results .success',
      (elements) => {
        return elements.map((el) => el.textContent);
      },
    );

    console.log('Success messages:', successMessages);

    // Verify helpinClient global is available
    const clientAvailable = successMessages.some(
      (msg) => msg && msg.includes('helpinClient global'),
    );
    expect(
      clientAvailable,
      'helpinClient global should be found',
    ).toBeTruthy();

    // Verify event was tracked
    const eventTracked = successMessages.some(
      (msg) => msg && msg.includes('Event sent via global'),
    );
    expect(eventTracked, 'Event should be tracked').toBeTruthy();

    // Wait for the tracking request to complete
    await page.waitForTimeout(2000);

    // Verify that a script tag test event was sent
    const scriptTagEvent = findEventInRequests(
      requests,
      'test_event_script_load',
    );
    expect(
      scriptTagEvent,
      'Script tag test event should be captured',
    ).toBeTruthy();

    if (scriptTagEvent?.postData) {
      const data = JSON.parse(scriptTagEvent.postData);
      const payload = Array.isArray(data)
        ? data.find((event) => event.event_type === 'test_event_script_load')
        : data;

      expect(payload.event_attributes.source).toBe('non_amd');
    }
  });

  test('should track events via global client', async ({ page }) => {
    const requests: Array<{ url: string; postData?: string }> = [];

    // Setup request interception
    await page.route('**/api/v1/event**', async (route) => {
      const request = route.request();
      const postData = request.postData() || undefined;
      requests.push({ url: request.url(), postData });
      await route.continue();
    });

    // Navigate to the non-AMD/jQuery test page
    const pageResponse = await page.goto('/test/e2e/non-amd-jquery-test.html');
    expect(pageResponse?.ok()).toBeTruthy();

    // Wait for page to load
    await page.waitForLoadState('networkidle');

    // First run Test 2 to ensure the script is loaded
    await page.click('button:has-text("Run Test 2")');
    await page.waitForSelector(
      '#test2-results .success:has-text("Script loaded successfully")',
      { timeout: 10000 },
    );

    // Wait a bit for the script to fully initialize
    await page.waitForTimeout(2000);

    // Then run Test 3 to test event tracking
    await page.click('button:has-text("Run Test 3")');

    // Wait for the test to complete
    await page.waitForTimeout(2000);

    // Check if the success message for event tracking is present
    const successMessages = await page.$$eval(
      '#test3-results .success',
      (elements) => {
        return elements.map((el) => el.textContent);
      },
    );

    console.log('Test 3 success messages:', successMessages);

    // Verify event was tracked
    const eventTracked = successMessages.some(
      (msg) => msg && msg.includes('Event tracked using global client'),
    );
    expect(
      eventTracked,
      'Event should be tracked using global client',
    ).toBeTruthy();

    // Wait for the tracking request to complete
    await page.waitForTimeout(2000);

    // Verify that a manual event was sent
    const manualEvent = findEventInRequests(requests, 'manual_event');
    expect(manualEvent, 'Manual event should be captured').toBeTruthy();

    if (manualEvent?.postData) {
      const data = JSON.parse(manualEvent.postData);
      const payload = Array.isArray(data)
        ? data.find((event) => event.event_type === 'manual_event')
        : data;

      expect(payload.event_attributes.env).toBe('non-amd');
    }
  });

  test('should track events via jQuery event handlers', async ({ page }) => {
    const requests: Array<{ url: string; postData?: string }> = [];

    // Setup request interception
    await page.route('**/api/v1/event**', async (route) => {
      const request = route.request();
      const postData = request.postData() || undefined;
      requests.push({ url: request.url(), postData });
      await route.continue();
    });

    // Navigate to the non-AMD/jQuery test page
    const pageResponse = await page.goto('/test/e2e/non-amd-jquery-test.html');
    expect(pageResponse?.ok()).toBeTruthy();

    // Wait for page to load
    await page.waitForLoadState('networkidle');

    // First run Test 2 to ensure the script is loaded
    await page.click('button:has-text("Run Test 2")');
    await page.waitForSelector(
      '#test2-results .success:has-text("Script loaded successfully")',
      { timeout: 10000 },
    );

    // Wait a bit for the script to fully initialize
    await page.waitForTimeout(2000);

    // Then run Test 4 to test jQuery event handler integration
    await page.click('button:has-text("Run Test 4")');

    // Wait for the handlers to be set up
    await page.waitForTimeout(1000);

    // Check if the success message for event handlers is present
    const handlerMessages = await page.$$eval(
      '#test4-results .success',
      (elements) => {
        return elements.map((el) => el.textContent);
      },
    );

    console.log('Test 4 success messages:', handlerMessages);

    // Verify event handlers were bound
    const handlersSetup = handlerMessages.some(
      (msg) => msg && msg.includes('jQuery event handler bound'),
    );
    expect(handlersSetup, 'jQuery event handlers should be bound').toBeTruthy();

    // Click a button to trigger the event handler
    await page.click('button:has-text("Run Test 1")');

    // Wait for the click to be processed
    await page.waitForTimeout(1000);

    // Check if the success message for button click is present
    const clickMessages = await page.$$eval(
      '#test4-results .success',
      (elements) => {
        return elements.map((el) => el.textContent);
      },
    );

    console.log('Button click success messages:', clickMessages);

    // Verify the click was tracked
    const clickTracked = clickMessages.some(
      (msg) => msg && msg.includes('Tracked button click'),
    );
    expect(clickTracked, 'Button click should be tracked').toBeTruthy();

    // Wait for the tracking request to complete
    await page.waitForTimeout(2000);

    // Verify that a button click event was sent
    const buttonClickEvent = findEventInRequests(requests, 'button_clicked');
    expect(
      buttonClickEvent,
      'Button click event should be captured',
    ).toBeTruthy();
  });

  test('should run all tests sequentially', async ({ page }) => {
    const requests: Array<{ url: string; postData?: string }> = [];

    // Setup request interception
    await page.route('**/api/v1/event**', async (route) => {
      const request = route.request();
      const postData = request.postData() || undefined;
      requests.push({ url: request.url(), postData });
      await route.continue();
    });

    // Navigate to the non-AMD/jQuery test page
    const pageResponse = await page.goto('/test/e2e/non-amd-jquery-test.html');
    expect(pageResponse?.ok()).toBeTruthy();

    // Wait for page to load
    await page.waitForLoadState('networkidle');

    // Run all tests sequentially
    await page.click('button:has-text("Run All Sequentially")');

    // Wait for all tests to complete (approximately 15 seconds)
    await page.waitForTimeout(15000);

    // Get all success messages
    const allSuccessMessages = await page.$$eval('.success', (elements) => {
      return elements.map((el) => ({
        testId:
          el.closest('.test-section')?.querySelector('h2')?.textContent ||
          'unknown',
        message: el.textContent || '',
      }));
    });

    console.log('All success messages:', allSuccessMessages);

    // Verify jQuery test passed
    const jqueryWorks = allSuccessMessages.some(
      (msg) =>
        msg.testId.includes('Test 1') &&
        msg.message.includes('DOM manipulation works'),
    );
    expect(jqueryWorks, 'jQuery DOM manipulation should work').toBeTruthy();

    // Verify script tag test passed
    const scriptLoaded = allSuccessMessages.some(
      (msg) =>
        msg.testId.includes('Test 2') &&
        msg.message.includes('Script loaded successfully'),
    );
    expect(scriptLoaded, 'Script should load successfully').toBeTruthy();

    // Verify event tracking test passed
    const eventTracked = allSuccessMessages.some(
      (msg) =>
        msg.testId.includes('Test 3') &&
        msg.message.includes('Event tracked using global client'),
    );
    expect(
      eventTracked,
      'Event should be tracked using global client',
    ).toBeTruthy();

    // Verify jQuery event handler test passed
    const handlersSetup = allSuccessMessages.some(
      (msg) =>
        msg.testId.includes('Test 4') &&
        msg.message.includes('jQuery event handler bound'),
    );
    expect(handlersSetup, 'jQuery event handlers should be bound').toBeTruthy();

    // Verify that events were sent to the server
    expect(
      requests.length,
      'Should have captured network requests',
    ).toBeGreaterThan(0);

    // Check for specific events
    const scriptTagEvent = findEventInRequests(
      requests,
      'test_event_script_load',
    );
    expect(
      scriptTagEvent,
      'Script tag test event should be captured',
    ).toBeTruthy();

    const manualEvent = findEventInRequests(requests, 'manual_event');
    expect(manualEvent, 'Manual event should be captured').toBeTruthy();
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
