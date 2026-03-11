import { test, expect } from '@playwright/test';
import { HelpinGlobal } from '../../src/core/types';

// Add missing window property types
declare global {
  interface Window {
    helpin?: HelpinGlobal;
  }
}

test.describe('Helpin API Styles', () => {
  test('should support both command-style and object-oriented API', async ({
    page,
  }) => {
    // Navigate to the test page
    await page.goto('/test/e2e/test.html');

    // Wait for Helpin to initialize
    await page.waitForFunction(
      () => {
        return window?.helpin && typeof window.helpin === 'function';
      },
      { timeout: 10000 },
    );

    // Verify command-style API is available
    const hasCommandStyleAPI = await page.evaluate(() => {
      console.log('Checking command-style API');
      return window.helpin && typeof window.helpin === 'function';
    });

    expect(hasCommandStyleAPI).toBe(true);
    console.log('✅ Command-style API is available');

    // Verify object-oriented API methods are available
    const objectMethods = await page.evaluate(() => {
      console.log('Checking object-oriented API');
      if (!window.helpin) return [];

      // Get all methods attached to the helpin function
      return Object.keys(window.helpin).filter(
        (key) => typeof (window.helpin as any)[key] === 'function',
      );
    });

    console.log('Object-oriented API methods:', objectMethods);

    // Verify essential methods are available
    const requiredMethods = [
      'track',
      'pageview',
      'id',
      'group',
      'reset',
      'set',
      'unset',
    ];
    for (const method of requiredMethods) {
      expect(objectMethods).toContain(method);
      console.log(`✅ Object-oriented API has '${method}' method`);
    }

    // Verify getConfig method is available (special case for object-oriented API)
    const hasGetConfig = await page.evaluate(() => {
      return (
        window.helpin &&
        typeof (window.helpin as any).getConfig === 'function'
      );
    });

    expect(hasGetConfig).toBe(true);
    console.log('✅ Object-oriented API has getConfig method');

    // Verify both API styles are callable without errors
    const apiCallsWork = await page.evaluate(() => {
      try {
        // Test command-style API
        if (window.helpin) {
          (window.helpin as any)('track', 'test_command_api', {
            test: true,
          });
          console.log('Command-style API call succeeded');

          // Test object-oriented API
          (window.helpin as any).track('test_object_api', { test: true });
          console.log('Object-oriented API call succeeded');

          return true;
        }
        return false;
      } catch (e) {
        console.error('Error testing API calls:', e);
        return false;
      }
    });

    expect(apiCallsWork).toBe(true);
    console.log('✅ Both API styles are callable without errors');
  });

  test('should handle queued events for both API styles before ready', async ({
    page,
  }) => {
    // Create a test page that delays initialization
    await page.setContent(`
      <html>
        <head>
          <script>
            // Setup capture array
            window.capturedEvents = [];
            
            // Create a mock transport that captures events
            class MockTransport {
              send(payload) {
                window.capturedEvents.push(payload);
                return Promise.resolve();
              }
            }
            
            // Mock the HelpinClient
            class HelpinClient {
              constructor() {
                this.transport = new MockTransport();
              }
              
              track(eventName, payload) {
                this.transport.send({
                  event_type: eventName,
                  event_attributes: payload || {}
                });
              }
              
              pageview() {
                this.transport.send({
                  event_type: 'pageview'
                });
              }
            }
            
            // Setup namespace
            function initializeNamespacedClient(namespace, client) {
              let isReady = false;
              const queue = [];
              const onLoadCallbacks = [];
            
              function processQueue() {
                while (queue.length > 0) {
                  const item = queue.shift();
                  if (item) {
                    namespacedFunction.apply(null, item);
                  }
                }
              }
            
              function executeOnLoadCallbacks() {
                onLoadCallbacks.forEach(callback => callback());
                onLoadCallbacks.length = 0;
              }
            
              // Create the main function that handles command-style calls
              function namespacedFunction(...args) {
                const method = args[0];
            
                if (method === 'onLoad') {
                  if (typeof args[1] === 'function') {
                    if (isReady) {
                      args[1]();
                    } else {
                      onLoadCallbacks.push(args[1]);
                    }
                  }
                  return;
                }
            
                if (!isReady) {
                  queue.push(args);
                  return;
                }
            
                if (typeof client[method] === 'function') {
                  return client[method].apply(client, args.slice(1));
                } else {
                  console.error(\`Method \${method} not found on HelpinClient\`);
                }
              }
            
              // Attach methods directly to the function for object-oriented API
              const methods = [
                'track',
                'pageview'
              ];
            
              methods.forEach(method => {
                namespacedFunction[method] = function(...args) {
                  if (!isReady) {
                    queue.push([method, ...args]);
                    return;
                  }
                  
                  if (typeof client[method] === 'function') {
                    return client[method].apply(client, args);
                  }
                };
              });
            
              // Set the function on the window
              window[namespace] = namespacedFunction;
            
              // Initialize queue processing
              const queueName = \`\${namespace}Q\`;
              const existingQueue = window[queueName] || [];
              window[queueName] = existingQueue;
            
              existingQueue.push = function(...args) {
                namespacedFunction.apply(null, args);
                return Array.prototype.push.apply(this, args);
              };
            
              // Queue events before ready
              window.helpin('track', 'queued_command_event', { queued: true });
              window.helpin.track('queued_object_event', { queued: true });
            
              // Set client as ready after a delay
              setTimeout(() => {
                isReady = true;
                processQueue();
                executeOnLoadCallbacks();
                console.log(\`Helpin client for namespace \${namespace} is ready\`);
              }, 500);
            }
            
            // Initialize with a delay
            const client = new HelpinClient();
            initializeNamespacedClient('helpin', client);
          </script>
        </head>
        <body>
          <h1>API Styles Test</h1>
        </body>
      </html>
    `);

    // Wait for initialization to complete
    await page.waitForFunction(
      () => {
        return window.capturedEvents && window.capturedEvents.length >= 2;
      },
      { timeout: 2000 },
    );

    // Verify queued events were processed
    const events = await page.evaluate(() => window.capturedEvents || []);
    expect(events.length).toBeGreaterThanOrEqual(2);

    // Verify command-style queued event
    const commandEvent = await page.evaluate(() => {
      return (window.capturedEvents || []).find(
        (e) => e.event_type === 'queued_command_event',
      );
    });
    expect(commandEvent).toBeTruthy();
    expect(commandEvent.event_attributes.queued).toBe(true);

    // Verify object-oriented queued event
    const objectEvent = await page.evaluate(() => {
      return (window.capturedEvents || []).find(
        (e) => e.event_type === 'queued_object_event',
      );
    });
    expect(objectEvent).toBeTruthy();
    expect(objectEvent.event_attributes.queued).toBe(true);
  });
});
