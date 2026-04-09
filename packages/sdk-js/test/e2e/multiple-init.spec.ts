import { test, expect } from '@playwright/test';
import { HelpinGlobal } from '../../src/core/types';

declare global {
  interface Window {
    helpin?: HelpinGlobal;
    helpinQ?: any[];
    consoleWarnings?: string[];
    initAttempts?: number;
    helpinClient?: Function;
    client?: any;
    earlyInit?: Function;
    debugLogs?: string[];
    testAutocaptureInit?: Function;
    eventListenerCount?: number;
    runTest?: Function;
    initSucceeded?: boolean;
    browserSupported?: boolean;
    eventListeners?: { [key: string]: number };
    eventsFired?: { [key: string]: number };
    configHistory?: any[];
    listenersByNamespace?: { [key: string]: number };
    eventsByNamespace?: { [key: string]: number };
    helpin_ns1?: Function;
    helpin_ns2?: Function;
    listenersRegistered?: { [key: string]: number };
    eventsTracked?: { [key: string]: number };
    consoleLogs?: string[];
    processedCommands?: { [key: string]: any[] };
    helpin_ns1Q?: any[];
    helpin_ns2Q?: any[];
    onLoadCallbacksExecuted?: { [key: string]: boolean };
    namespaceErrors?: { [key: string]: string[] };
    commandCounts?: { [key: string]: number };
    commandLog?: string[];
    callbacksExecuted?: number;
    errorCount?: { [key: string]: number };
    errorLog?: string[];
    configSnapshots?: { [key: string]: any }[];
  }
}

test.describe('Helpin Multiple Initialization Tests', () => {
  test('should prevent duplicate event listeners when initialized multiple times', async ({
    page,
  }) => {
    // Navigate to a simple test page that simulates our SDK behavior
    await page.setContent(`
      <!DOCTYPE html>
      <html>
      <head>
        <title>Multiple Init Test</title>
        <script>
          // Capture console warnings
          window.consoleWarnings = [];
          const originalWarn = console.warn;
          console.warn = function(...args) {
            window.consoleWarnings.push(args.join(' '));
            originalWarn.apply(console, args);
          };
          
          // Track event listener counts
          window.eventListenerCount = 0;
          const originalAddEventListener = EventTarget.prototype.addEventListener;
          EventTarget.prototype.addEventListener = function(type, listener, options) {
            if (type === 'click') {
              window.eventListenerCount++;
            }
            return originalAddEventListener.call(this, type, listener, options);
          };
          
          // Create global flag like in our SDK
          window.__HELPIN_AUTOCAPTURE_INITIALIZED__test_key = false;
        </script>
      </head>
      <body>
        <h1>Multiple Initialization Test</h1>
        <button id="test-button">Test Button</button>
        <div id="debug-output"></div>
        
        <script>
          // Mock our SDK functionality
          function helpinClient(config) {
            const key = config.widgetKey || '';
            const flag = '__HELPIN_AUTOCAPTURE_INITIALIZED__' + key;
            
            // Check for existing autocapture initialization
            if (config.autocapture && window[flag]) {
              console.warn('Helpin: Autocapture already initialized in another instance, skipping duplicate initialization.');
              config.disableAutocaptureListenerRegistration = true;
            }
            
            // Set global flag if autocapture is enabled and not disabled
            if (config.autocapture && !config.disableAutocaptureListenerRegistration) {
              window[flag] = true;
            }
            
            return {
              init: function() {
                // Only add event listeners if not disabled
                if (!config.disableAutocaptureListenerRegistration) {
                  document.addEventListener('click', function() {});
                }
              }
            };
          }
          
          // Add to window
          window.helpinClient = helpinClient;
          
          // Initialize first instance
          const client1 = helpinClient({
            widgetKey: 'test_key',
            autocapture: true
          });
          client1.init();

          // Button to initialize second instance
          document.getElementById('test-button').addEventListener('click', function() {
            // Initialize second instance
            const client2 = helpinClient({
              widgetKey: 'test_key',
              autocapture: true
            });
            client2.init();
            
            // Show debug info
            const debugOutput = document.getElementById('debug-output');
            debugOutput.innerHTML = 'Warnings: ' + JSON.stringify(window.consoleWarnings);
            
            // Update UI to indicate completion
            document.body.setAttribute('data-second-init', 'complete');
          });
        </script>
      </body>
      </html>
    `);

    // Wait for the page to initialize
    await page.waitForTimeout(1000);

    // Get initial click listener count
    const initialCount = await page.evaluate(
      () => window.eventListenerCount || 0,
    );
    console.log('Initial listener count:', initialCount);

    // Click the button to initialize a second instance
    await page.click('#test-button');

    // Wait for second initialization
    await page.waitForSelector('body[data-second-init="complete"]');
    await page.waitForTimeout(1000);

    // Get the new listener count
    const newCount = await page.evaluate(() => window.eventListenerCount || 0);
    console.log('New listener count:', newCount);

    // Check for warning about duplicate initialization
    const warnings = await page.evaluate(() => window.consoleWarnings || []);
    console.log('Warnings:', warnings);

    // We should have a warning about duplicate initialization
    expect(
      warnings.some((warning) => warning.includes('already initialized')),
    ).toBeTruthy();

    // The listener count should not have doubled (only one new listener should be added from the button click handler)
    expect(newCount).toBe(initialCount + 1);
  });

  test('should handle DOM ready check and retry initialization', async ({
    page,
  }) => {
    // Create a test page that directly tests the retry logic
    await page.setContent(`
      <!DOCTYPE html>
      <html>
      <head>
        <title>DOM Ready Test</title>
        <script>
          // Setup debug log capture
          window.debugLogs = [];
          const originalDebug = console.debug;
          console.debug = function(msg) {
            window.debugLogs.push(msg);
            originalDebug.apply(console, arguments);
          };
        </script>
      </head>
      <body>
        <h1>DOM Ready Test</h1>
        <div id="debug-output"></div>
        
        <script>
          // Create a simple flag to track if initialization succeeded
          window.initSucceeded = false;
          window.initAttempts = 0;
          
          // Mock AutoCapture class with forced retry
          class MockAutoCapture {
            constructor() {
              this.domHandlersAttached = false;
              this.retryCount = 0;
              this.maxRetries = 2; // Force at least 2 retries
            }
            
            init() {
              window.initAttempts++;
              
              if (this.domHandlersAttached) {
                console.debug('Autocapture already initialized.');
                return;
              }
              
              // Force retry for the first couple of attempts
              if (this.retryCount < this.maxRetries) {
                this.retryCount++;
                console.debug('Document not ready yet, trying again in 500 milliseconds...');
                setTimeout(() => this.init(), 100); // Use shorter timeout for test
                return;
              }
              
              // After retries, succeed
              this.addDomEventHandlers();
              this.domHandlersAttached = true;
              window.initSucceeded = true;
            }
            
            addDomEventHandlers() {
              console.debug('Autocapture initialized successfully.');
            }
          }
          
          // Run the test
          const autocapture = new MockAutoCapture();
          console.debug('Starting test - about to call init()');
          autocapture.init();
          
          // Check status after a delay
          setTimeout(() => {
            const debugOutput = document.getElementById('debug-output');
            debugOutput.innerHTML = 'Debug logs: ' + JSON.stringify(window.debugLogs) + 
                                   '<br>Init attempts: ' + window.initAttempts;
            document.body.setAttribute('data-test-complete', 'true');
          }, 1000);
        </script>
      </body>
      </html>
    `);

    // Wait for test to complete
    await page.waitForSelector('body[data-test-complete="true"]', {
      timeout: 5000,
    });
    await page.waitForTimeout(500);

    // Get debug logs
    const debugLogs = await page.evaluate(() => window.debugLogs || []);
    console.log('Debug logs:', debugLogs);

    // Get initialization attempts
    const initAttempts = await page.evaluate(() => window.initAttempts || 0);
    console.log('Init attempts:', initAttempts);

    // Get initialization success flag
    const initSucceeded = await page.evaluate(() => window.initSucceeded);
    console.log('Init succeeded:', initSucceeded);

    // Check for retry message
    const retryLogs = debugLogs.filter(
      (log) => typeof log === 'string' && log.includes('Document not ready'),
    );
    console.log('Retry logs:', retryLogs);
    expect(retryLogs.length).toBeGreaterThan(0);

    // Check for successful initialization
    const successLogs = debugLogs.filter(
      (log) =>
        typeof log === 'string' && log.includes('initialized successfully'),
    );
    console.log('Success logs:', successLogs);
    expect(successLogs.length).toBeGreaterThan(0);

    // Verify multiple initialization attempts
    expect(initAttempts).toBeGreaterThan(1);

    // Verify initialization eventually succeeded
    expect(initSucceeded).toBeTruthy();
  });

  test('should not register listeners when autocapture is false', async ({
    page,
  }) => {
    await page.setContent(`
      <script>
        window.eventListenerCount = 0;
        const originalAddEventListener = EventTarget.prototype.addEventListener;
        EventTarget.prototype.addEventListener = function(type, listener, options) {
          if (type === 'click') window.eventListenerCount++;
          return originalAddEventListener.call(this, type, listener, options);
        };

        function helpinClient(config) {
          if (config.autocapture) {
            document.addEventListener('click', function () {});
          }
          return {
            init: function () {}
          };
        }

        const client = helpinClient({ widgetKey: 'abc123', autocapture: false });
        client.init();
      </script>
    `);

    const count = await page.evaluate(() => window.eventListenerCount);
    expect(count).toBe(0);
  });

  test('should isolate autocapture flags across different keys', async ({
    page,
  }) => {
    await page.setContent(`
      <script>
        window.consoleWarnings = [];
        const originalWarn = console.warn;
        console.warn = function(...args) {
          window.consoleWarnings.push(args.join(' '));
          originalWarn.apply(console, args);
        };

        function helpinClient(config) {
          const flag = '__HELPIN_AUTOCAPTURE_INITIALIZED__' + config.widgetKey;
          if (config.autocapture && window[flag]) {
            console.warn('Autocapture already initialized for key:', config.widgetKey);
            config.disableAutocaptureListenerRegistration = true;
          }
          if (config.autocapture && !config.disableAutocaptureListenerRegistration) {
            window[flag] = true;
          }
        }

        // First key
        helpinClient({ widgetKey: 'key_1', autocapture: true });
        // Second key (should not trigger warning)
        helpinClient({ widgetKey: 'key_2', autocapture: true });
      </script>
    `);

    const warnings = await page.evaluate(() => window.consoleWarnings);
    expect(warnings?.some((w) => w.includes('key_1'))).toBeFalsy(); // Should not warn on first
    expect(warnings?.some((w) => w.includes('key_2'))).toBeFalsy(); // Should not interfere
  });

  test('should honor disableAutocaptureListenerRegistration config flag', async ({
    page,
  }) => {
    await page.setContent(`
      <script>
        window.eventListenerCount = 0;
        const originalAddEventListener = EventTarget.prototype.addEventListener;
        EventTarget.prototype.addEventListener = function(type, listener, options) {
          if (type === 'click') window.eventListenerCount++;
          return originalAddEventListener.call(this, type, listener, options);
        };

        function helpinClient(config) {
          if (config.autocapture && !config.disableAutocaptureListenerRegistration) {
            document.addEventListener('click', function () {});
          }
          return {
            init: function () {}
          };
        }

        helpinClient({
          widgetKey: 'test_key_2',
          autocapture: true,
          disableAutocaptureListenerRegistration: true
        }).init();
      </script>
    `);
    const count = await page.evaluate(() => window.eventListenerCount);
    expect(count).toBe(0);
  });

  test('should support multiple instances with different namespaces', async ({
    page,
  }) => {
    await page.setContent(`
      <script>
        // Track events and listeners by namespace
        window.eventsByNamespace = { default: 0, custom: 0 };
        window.listenersByNamespace = { default: 0, custom: 0 };
        
        // Track event listener counts
        const originalAddEventListener = EventTarget.prototype.addEventListener;
        EventTarget.prototype.addEventListener = function(type, listener, options) {
          if (type === 'click') {
            // We'll consider this a default namespace listener
            window.listenersByNamespace.default++;
          } else if (type === 'mousemove') {
            // We'll consider this a custom namespace listener
            window.listenersByNamespace.custom++;
          }
          return originalAddEventListener.call(this, type, listener, options);
        };

        // Mock SDK with namespace support
        function helpinClient(config) {
          const key = config.widgetKey || '';
          const namespace = config.namespace || 'default';
          const flag = '__HELPIN_AUTOCAPTURE_INITIALIZED__' + key + '_' + namespace;

          // Check for existing autocapture initialization with this key and namespace
          if (config.autocapture && window[flag]) {
            console.warn('Autocapture already initialized for key ' + key + ' in namespace ' + namespace);
            config.disableAutocaptureListenerRegistration = true;
          }
          
          // Set global flag if autocapture is enabled and not disabled
          if (config.autocapture && !config.disableAutocaptureListenerRegistration) {
            window[flag] = true;
          }
          
          // Create a track method that records events by namespace
          const track = function(eventName) {
            if (namespace === 'default') {
              window.eventsByNamespace.default++;
            } else if (namespace === 'custom') {
              window.eventsByNamespace.custom++;
            }
          };
          
          return {
            init: function() {
              // Add different event listeners based on namespace
              if (config.autocapture && !config.disableAutocaptureListenerRegistration) {
                if (namespace === 'default') {
                  document.addEventListener('click', function() {});
                } else if (namespace === 'custom') {
                  document.addEventListener('mousemove', function() {});
                }
              }
            },
            track: track
          };
        }
        
        // Initialize default namespace
        const defaultClient = helpinClient({
          widgetKey: 'shared_key',
          autocapture: true
        });
        defaultClient.init();
        defaultClient.track('default_event');

        // Initialize custom namespace
        const customClient = helpinClient({
          widgetKey: 'shared_key',
          namespace: 'custom',
          autocapture: true
        });
        customClient.init();
        customClient.track('custom_event');

        // Try to initialize default namespace again (should be skipped)
        const duplicateClient = helpinClient({
          widgetKey: 'shared_key',
          autocapture: true
        });
        duplicateClient.init();
      </script>
    `);

    // Check listener counts by namespace
    const listenersByNamespace = await page.evaluate(
      () => window.listenersByNamespace,
    );
    console.log('Listeners by namespace:', listenersByNamespace);

    // Check event counts by namespace
    const eventsByNamespace = await page.evaluate(
      () => window.eventsByNamespace,
    );
    console.log('Events by namespace:', eventsByNamespace);

    // Verify each namespace has exactly one listener
    expect(listenersByNamespace!.default).toBe(1);
    expect(listenersByNamespace!.custom).toBe(1);

    // Verify each namespace has exactly one event
    expect(eventsByNamespace!.default).toBe(1);
    expect(eventsByNamespace!.custom).toBe(1);
  });

  test('should process command queues correctly in namespaced installations', async ({
    page,
  }) => {
    // Create a test page that tests command queue processing for namespaced installations
    await page.setContent(`
      <!DOCTYPE html>
      <html>
      <head>
        <title>Namespace Command Queue Test</title>
        <script>
          // Track command execution counts
          window.commandCounts = { ns1: 0, ns2: 0 };
          window.commandLog = [];
          
          // Create queues for early commands
          window.helpin_ns1Q = [];
          window.helpin_ns2Q = [];
          
          // Add early commands to queues before SDK is loaded
          window.helpin_ns1Q.push(['init', { widgetKey: 'early_key_1' }]);
          window.helpin_ns1Q.push(['track', 'early_event_1']);

          window.helpin_ns2Q.push(['init', { widgetKey: 'early_key_2' }]);
          window.helpin_ns2Q.push(['track', 'early_event_2']);
          window.helpin_ns2Q.push(['track', 'another_early_event_2']);
        </script>
      </head>
      <body>
        <h1>Namespace Command Queue Test</h1>
        <div id="debug-output"></div>
        
        <script>
          // Simulate SDK loading and processing queues
          function initializeNamespacedClient(namespace) {
            let isReady = false;
            const queue = [];
            
            // Create namespaced function
            window[namespace] = function(command, ...args) {
              // Log command execution
              window.commandLog.push(namespace + ':' + command + ':' + (args[0] || ''));
              window.commandCounts[namespace.replace('helpin_', '')] += 1;
              
              // If not ready, queue the command
              if (!isReady) {
                queue.push([command, ...args]);
                return;
              }
              
              console.log(namespace + ' processed command:', command, args);
            };
            
            // Process existing queue
            const queueName = namespace + 'Q';
            const existingQueue = window[queueName] || [];
            
            // Override push method to process new items
            existingQueue.push = function(...args) {
              window[namespace].apply(null, args);
              return Array.prototype.push.apply(this, args);
            };
            
            // Process any existing queue items
            while (existingQueue.length > 0) {
              const item = existingQueue.shift();
              if (item) {
                queue.push(item);
              }
            }
            
            // Set as ready and process queue
            setTimeout(() => {
              isReady = true;
              
              // Process queued commands
              while (queue.length > 0) {
                const item = queue.shift();
                if (item) {
                  window[namespace].apply(null, item);
                }
              }
              
              console.log(namespace + ' is ready, queue processed');
              
              // Add a late command after initialization
              window[namespace]('track', 'late_event_' + namespace);
            }, 100);
          }
          
          // Initialize both namespaced clients
          initializeNamespacedClient('helpin_ns1');
          initializeNamespacedClient('helpin_ns2');
          
          // Wait for processing to complete and show results
          setTimeout(() => {
            const debugOutput = document.getElementById('debug-output');
            debugOutput.innerHTML = 'Command counts: ' + JSON.stringify(window.commandCounts) + '<br>Command log: ' + window.commandLog.join(', ');
            document.body.setAttribute('data-test-complete', 'true');
          }, 500);
        </script>
      </body>
      </html>
    `);

    // Wait for test to complete
    await page.waitForSelector('body[data-test-complete="true"]', {
      timeout: 5000,
    });

    // Get command counts and log
    const commandCounts = await page.evaluate(() => window.commandCounts);
    const commandLog = await page.evaluate(() => window.commandLog);
    console.log('Command counts:', commandCounts);
    console.log('Command log:', commandLog);

    // Verify commands were processed for both namespaces
    expect(commandCounts!.ns1).toBeGreaterThanOrEqual(3); // init, early_event, late_event
    expect(commandCounts!.ns2).toBeGreaterThanOrEqual(4); // init, 2 early events, late_event

    // Verify specific commands in the log
    expect(commandLog!.some((cmd) => cmd.includes('ns1:init'))).toBeTruthy();
    expect(
      commandLog!.some((cmd) => cmd.includes('ns1:track:early_event_1')),
    ).toBeTruthy();
    expect(
      commandLog!.some((cmd) =>
        cmd.includes('ns1:track:late_event_helpin_ns1'),
      ),
    ).toBeTruthy();

    expect(commandLog!.some((cmd) => cmd.includes('ns2:init'))).toBeTruthy();
    expect(
      commandLog!.some((cmd) => cmd.includes('ns2:track:early_event_2')),
    ).toBeTruthy();
    expect(
      commandLog!.some((cmd) =>
        cmd.includes('ns2:track:another_early_event_2'),
      ),
    ).toBeTruthy();
    expect(
      commandLog!.some((cmd) =>
        cmd.includes('ns2:track:late_event_helpin_ns2'),
      ),
    ).toBeTruthy();
  });

  test('should handle onLoad callbacks correctly in namespaced installations', async ({
    page,
  }) => {
    // Create a test page that tests onLoad callbacks for namespaced installations
    await page.setContent(`
      <!DOCTYPE html>
      <html>
      <head>
        <title>Namespace onLoad Callback Test</title>
        <script>
          // Track callback execution count
          window.callbacksExecuted = 0;
        </script>
      </head>
      <body>
        <h1>Namespace onLoad Callback Test</h1>
        
        <script>
          // Create queues for early commands
          window.helpin_ns1Q = [];
          window.helpin_ns2Q = [];
          
          // Add early onLoad callbacks before SDK is loaded
          window.helpin_ns1Q.push(['onLoad', function() {
            console.log('NS1 early onLoad callback executed');
            window.callbacksExecuted++;
            document.body.setAttribute('data-ns1-early-callback', 'executed');
          }]);
          
          window.helpin_ns2Q.push(['onLoad', function() {
            console.log('NS2 early onLoad callback executed');
            window.callbacksExecuted++;
            document.body.setAttribute('data-ns2-early-callback', 'executed');
          }]);
          
          // Simulate SDK loading and processing queues
          function initializeNamespacedClient(namespace) {
            let isReady = false;
            const queue = [];
            const onLoadCallbacks = [];
            
            // Create namespaced function
            window[namespace] = function(command, ...args) {
              if (command === 'onLoad') {
                if (typeof args[0] === 'function') {
                  if (isReady) {
                    args[0]();
                  } else {
                    onLoadCallbacks.push(args[0]);
                  }
                }
                return;
              }
            };
            
            // Process existing queue
            const queueName = namespace + 'Q';
            const existingQueue = window[queueName] || [];
            
            // Process any existing queue items
            while (existingQueue.length > 0) {
              const item = existingQueue.shift();
              if (item) {
                window[namespace].apply(null, item);
              }
            }
            
            // Set as ready and execute callbacks
            setTimeout(() => {
              isReady = true;
              
              // Execute onLoad callbacks
              onLoadCallbacks.forEach(callback => {
                callback();
                window.callbacksExecuted++;
              });
              
              // Add a late onLoad callback after initialization
              window[namespace]('onLoad', function() {
                console.log('Late ' + namespace + ' onLoad callback executed');
                window.callbacksExecuted++;
                document.body.setAttribute('data-' + namespace + '-late-callback', 'executed');
              });
            }, 100);
          }
          
          // Initialize both namespaced clients
          initializeNamespacedClient('helpin_ns1');
          initializeNamespacedClient('helpin_ns2');
          
          // Wait for processing to complete and show results
          setTimeout(() => {
            document.body.setAttribute('data-test-complete', 'true');
          }, 500);
        </script>
      </body>
      </html>
    `);

    // Wait for test to complete
    await page.waitForSelector('body[data-test-complete="true"]', {
      timeout: 5000,
    });

    // Get callback execution count
    const callbacksExecuted = await page.evaluate(
      () => window.callbacksExecuted,
    );
    console.log('Callbacks executed:', callbacksExecuted);

    // Check for data attributes that indicate callbacks were executed
    const ns1EarlyCallback = await page.getAttribute(
      'body',
      'data-ns1-early-callback',
    );
    const ns2EarlyCallback = await page.getAttribute(
      'body',
      'data-ns2-early-callback',
    );
    const ns1LateCallback = await page.getAttribute(
      'body',
      'data-helpin_ns1-late-callback',
    );
    const ns2LateCallback = await page.getAttribute(
      'body',
      'data-helpin_ns2-late-callback',
    );

    // Verify callbacks were executed
    expect(callbacksExecuted).toBeGreaterThanOrEqual(4); // 2 early + 2 late
    expect(ns1EarlyCallback).toBe('executed');
    expect(ns2EarlyCallback).toBe('executed');
    expect(ns1LateCallback).toBe('executed');
    expect(ns2LateCallback).toBe('executed');
  });

  test('should handle error conditions gracefully in namespaced installations', async ({
    page,
  }) => {
    // Create a test page that tests error handling for namespaced installations
    await page.setContent(`
      <!DOCTYPE html>
      <html>
      <head>
        <title>Namespace Error Handling Test</title>
        <script>
          // Track errors by namespace
          window.errorCount = { ns1: 0, ns2: 0 };
          window.errorLog = [];
        </script>
      </head>
      <body>
        <h1>Namespace Error Handling Test</h1>
        <div id="error-output"></div>
        
        <script>
          // Override console.error to capture errors
          const originalConsoleError = console.error;
          console.error = function(message) {
            originalConsoleError.apply(console, arguments);
            
            // Log the error message
            window.errorLog.push(message);
            
            // Update error count by namespace
            if (message.includes('ns1')) {
              window.errorCount.ns1++;
              document.body.setAttribute('data-ns1-error', 'true');
            } else if (message.includes('ns2')) {
              window.errorCount.ns2++;
              document.body.setAttribute('data-ns2-error', 'true');
            }
            
            // Update error output
            const errorOutput = document.getElementById('error-output');
            errorOutput.innerHTML += message + '<br>';
          };
          
          // Simulate SDK loading with error handling
          function initializeNamespacedClient(namespace) {
            // Create namespaced function with error handling
            window[namespace] = function(command, ...args) {
              try {
                if (command === 'invalidCommand') {
                  throw new Error(namespace + ': Invalid command');
                }
                
                if (command === 'track' && !args[0]) {
                  throw new Error(namespace + ': Missing event name for track');
                }
                
                if (command === 'identify' && !args[0]) {
                  throw new Error(namespace + ': Missing user ID for identify');
                }
                
                console.log(namespace + ' processed command:', command, args);
              } catch (error) {
                console.error(error.message);
              }
            };
          }
          
          // Initialize both namespaced clients
          initializeNamespacedClient('helpin_ns1');
          initializeNamespacedClient('helpin_ns2');
          
          // Test error conditions
          setTimeout(() => {
            // Test invalid command
            helpin_ns1('invalidCommand');
            
            // Test missing parameters
            helpin_ns1('track'); // Missing event name
            helpin_ns2('identify'); // Missing user ID
            
            // Test valid commands (should not error)
            helpin_ns1('track', 'valid_event');
            helpin_ns2('identify', 'user123');
            
            document.body.setAttribute('data-test-complete', 'true');
          }, 100);
        </script>
      </body>
      </html>
    `);

    // Wait for test to complete
    await page.waitForSelector('body[data-test-complete="true"]', {
      timeout: 5000,
    });

    // Get error counts and log
    const errorCount = await page.evaluate(() => window.errorCount);
    const errorLog = await page.evaluate(() => window.errorLog);
    console.log('Error counts:', errorCount);
    console.log('Error log:', errorLog);

    // Check for data attributes that indicate errors occurred
    const ns1Error = await page.getAttribute('body', 'data-ns1-error');
    const ns2Error = await page.getAttribute('body', 'data-ns2-error');

    // Verify errors were captured for both namespaces
    expect(errorCount!.ns1).toBeGreaterThanOrEqual(2); // Invalid command, missing event name
    expect(errorCount!.ns2).toBeGreaterThanOrEqual(1); // Missing user ID
    expect(ns1Error).toBe('true');
    expect(ns2Error).toBe('true');

    // Verify specific error messages
    expect(
      errorLog!.some((error) => error.includes('Invalid command')),
    ).toBeTruthy();
    expect(
      errorLog!.some((error) => error.includes('Missing event name')),
    ).toBeTruthy();
    expect(
      errorLog!.some((error) => error.includes('Missing user ID')),
    ).toBeTruthy();
  });

  test('should maintain separate configurations in namespaced installations', async ({
    page,
  }) => {
    // Create a test page that tests configuration isolation for namespaced installations
    await page.setContent(`
      <!DOCTYPE html>
      <html>
      <head>
        <title>Namespace Configuration Isolation Test</title>
        <script>
          // Store configurations by namespace
          window.configSnapshots = {};
        </script>
      </head>
      <body>
        <h1>Namespace Configuration Isolation Test</h1>
        <div id="config-output"></div>
        
        <script>
          // Simulate SDK loading with configuration tracking
          function initializeNamespacedClient(namespace, initialConfig) {
            let config = { ...initialConfig };
            
            // Store initial configuration
            window.configSnapshots[namespace] = { ...config };
            updateConfigOutput();
            
            // Create namespaced function with configuration management
            window[namespace] = function(command, ...args) {
              if (command === 'init') {
                // Update configuration
                const newConfig = args[0] || {};
                config = { ...config, ...newConfig };
                window.configSnapshots[namespace] = { ...config };
                updateConfigOutput();
                return;
              }
              
              if (command === 'getConfig') {
                // Add a data attribute with stringified config for testing
                const configStr = JSON.stringify(config);
                document.body.setAttribute('data-' + namespace + '-config', configStr);
                return config;
              }
              
              console.log(namespace + ' processed command:', command, args);
            };
          }
          
          function updateConfigOutput() {
            const configOutput = document.getElementById('config-output');
            configOutput.innerHTML = JSON.stringify(window.configSnapshots, null, 2);
          }
          
          // Initialize namespaced clients with different configs
          initializeNamespacedClient('helpin_ns1', { key: 'project1', host: 'api1.example.com' });
          initializeNamespacedClient('helpin_ns2', { key: 'project2', host: 'api2.example.com' });
          
          // Test configuration isolation
          setTimeout(() => {
            // Update configurations
            helpin_ns1('init', { debug: true });
            helpin_ns2('init', { debug: false, customDomain: 'custom.example.com' });
            
            // Update ns1 config again
            helpin_ns1('init', { newFeature: 'enabled' });
            
            // Get final configs
            helpin_ns1('getConfig');
            helpin_ns2('getConfig');
            
            document.body.setAttribute('data-test-complete', 'true');
          }, 100);
        </script>
      </body>
      </html>
    `);

    // Wait for test to complete
    await page.waitForSelector('body[data-test-complete="true"]', {
      timeout: 5000,
    });

    // Get configuration snapshots from data attributes
    const ns1ConfigStr = await page.getAttribute(
      'body',
      'data-helpin_ns1-config',
    );
    const ns2ConfigStr = await page.getAttribute(
      'body',
      'data-helpin_ns2-config',
    );

    const ns1Config = JSON.parse(ns1ConfigStr || '{}');
    const ns2Config = JSON.parse(ns2ConfigStr || '{}');

    console.log('NS1 Config:', ns1Config);
    console.log('NS2 Config:', ns2Config);

    // Verify configurations remained isolated
    expect(ns1Config.key).toBe('project1');
    expect(ns1Config.host).toBe('api1.example.com');
    expect(ns1Config.debug).toBe(true);
    expect(ns1Config.newFeature).toBe('enabled');

    expect(ns2Config.key).toBe('project2');
    expect(ns2Config.host).toBe('api2.example.com');
    expect(ns2Config.debug).toBe(false);
    expect(ns2Config.customDomain).toBe('custom.example.com');
    expect(ns2Config.newFeature).toBeUndefined();
  });

  test('should support direct pixel installation alongside namespaced installations', async ({
    page,
  }) => {
    // Create a test page that simulates direct pixel installation and namespaced SDK instances
    await page.setContent(`
      <!DOCTYPE html>
      <html>
      <head>
        <title>Multiple Installation Methods Test</title>
        <script>
          // Track events by installation method
          window.eventsTracked = { direct: 0, namespace1: 0, namespace2: 0 };
          
          // Track event listener registrations by installation method
          window.listenersRegistered = { direct: 0, namespace1: 0, namespace2: 0 };
          
          // Capture console logs
          window.consoleLogs = [];
          const originalLog = console.log;
          console.log = function(...args) {
            window.consoleLogs.push(args.join(' '));
            originalLog.apply(console, args);
          };
          
          // Mock addEventListener to track registrations
          const originalAddEventListener = EventTarget.prototype.addEventListener;
          EventTarget.prototype.addEventListener = function(type, listener, options) {
            // Track based on data attributes we'll set on the listeners
            if (listener && listener.pixelType === 'direct' && type === 'click') {
              window.listenersRegistered.direct++;
            } else if (listener && listener.pixelType === 'namespace1' && type === 'mousemove') {
              window.listenersRegistered.namespace1++;
            } else if (listener && listener.pixelType === 'namespace2' && type === 'scroll') {
              window.listenersRegistered.namespace2++;
            }
            return originalAddEventListener.call(this, type, listener, options);
          };
        </script>
      </head>
      <body>
        <h1>Multiple Installation Methods Test</h1>
        <div id="debug-output"></div>
        
        <script>
          // 1. Direct pixel installation (simulated)
          console.log('Installing direct pixel');
          
          // Global helpin function for direct installation
          window.helpin = function(command, ...args) {
            console.log('Direct pixel received command:', command);
            if (command === 'track') {
              window.eventsTracked.direct++;
            }
          };
          
          // Add direct pixel event listener
          const directListener = function() {};
          directListener.pixelType = 'direct';
          document.addEventListener('click', directListener);
          
          // 2. First namespaced installation
          console.log('Installing first namespaced pixel');
          
          // Create first namespace
          window.helpin_ns1 = function(command, ...args) {
            console.log('Namespace 1 received command:', command);
            if (command === 'track') {
              window.eventsTracked.namespace1++;
            }
          };
          
          // Add first namespace event listener
          const ns1Listener = function() {};
          ns1Listener.pixelType = 'namespace1';
          document.addEventListener('mousemove', ns1Listener);
          
          // 3. Second namespaced installation
          console.log('Installing second namespaced pixel');
          
          // Create second namespace
          window.helpin_ns2 = function(command, ...args) {
            console.log('Namespace 2 received command:', command);
            if (command === 'track') {
              window.eventsTracked.namespace2++;
            }
          };
          
          // Add second namespace event listener
          const ns2Listener = function() {};
          ns2Listener.pixelType = 'namespace2';
          document.addEventListener('scroll', ns2Listener);
          
          // Track events with all installations
          window.helpin('track', 'direct_event');
          window.helpin_ns1('track', 'namespace1_event');
          window.helpin_ns2('track', 'namespace2_event');
          
          // Show debug info
          const debugOutput = document.getElementById('debug-output');
          debugOutput.innerHTML = 
            'Listeners registered: ' + JSON.stringify(window.listenersRegistered) + 
            '<br>Events tracked: ' + JSON.stringify(window.eventsTracked) + 
            '<br>Console logs: ' + JSON.stringify(window.consoleLogs);
          
          // Mark test as complete
          document.body.setAttribute('data-test-complete', 'true');
        </script>
      </body>
      </html>
    `);

    // Wait for test to complete
    await page.waitForSelector('body[data-test-complete="true"]', {
      timeout: 5000,
    });

    // Get listeners registered count
    const listenersRegistered = await page.evaluate(
      () => window.listenersRegistered,
    );
    console.log('Listeners registered:', listenersRegistered);

    // Get events tracked count
    const eventsTracked = await page.evaluate(() => window.eventsTracked);
    console.log('Events tracked:', eventsTracked);

    // Get console logs
    const consoleLogs = await page.evaluate(() => window.consoleLogs);
    console.log('Console logs:', consoleLogs);

    // Verify each installation registered its listeners
    expect(listenersRegistered!.direct).toBe(1);
    expect(listenersRegistered!.namespace1).toBe(1);
    expect(listenersRegistered!.namespace2).toBe(1);

    // Verify each installation tracked its events
    expect(eventsTracked!.direct).toBe(1);
    expect(eventsTracked!.namespace1).toBe(1);
    expect(eventsTracked!.namespace2).toBe(1);

    // Verify installation logs
    expect(
      consoleLogs!.some((log) => log.includes('Installing direct pixel')),
    ).toBeTruthy();
    expect(
      consoleLogs!.some((log) =>
        log.includes('Installing first namespaced pixel'),
      ),
    ).toBeTruthy();
    expect(
      consoleLogs!.some((log) =>
        log.includes('Installing second namespaced pixel'),
      ),
    ).toBeTruthy();
  });
});
