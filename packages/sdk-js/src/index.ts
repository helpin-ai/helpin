import { HelpinClient } from './core/client';
import { WidgetManager, type WidgetSettings } from './core/widget';
import { defaultConfig } from './core/config';
import type { Config } from './core/types';
import { LogLevel } from './utils/logger';
import type { UserProps, EventPayload, ClientProperties } from './core/types';
import { parseLogLevel } from './utils/helpers';
import { convertKeysToCamelCase, isWindowAvailable } from './utils/common';
import { isAMDEnvironment, getAMDDefine } from './utils/amd-detector';

const widgetManager = new WidgetManager();

function helpinClient(config: Partial<Config>): HelpinClient {
  const cleanConfig = JSON.parse(JSON.stringify(config));
  const camelCaseConfig = convertKeysToCamelCase(cleanConfig);
  const mergedConfig: Config = {
    ...defaultConfig,
    ...camelCaseConfig,
  } as Config;

  if (!mergedConfig.key) {
    throw new Error('API key is required!');
  }

  if (!mergedConfig.trackingHost) {
    throw new Error('Tracking host is required!');
  }

  return new HelpinClient(mergedConfig);
}

function initFromScript(script: HTMLScriptElement): HelpinClient {
  const config: Partial<Config> = {
    key: script.getAttribute('data-key') || undefined,
    trackingHost:
      script.getAttribute('data-tracking-host') ||
      'https://events.helpin.ai',
    logLevel: parseLogLevel(script.getAttribute('data-log-level')),
    autoPageview:
      script.getAttribute('data-auto-pageview') === 'false'
        ? false
        : script.getAttribute('data-auto-pageview') === 'true'
          ? true
          : undefined, // Let default config handle it
    useBeaconApi: script.getAttribute('data-use-beacon-api') === 'true',
    forceUseFetch: script.getAttribute('data-force-use-fetch') === 'true',
    gaHook: script.getAttribute('data-ga-hook') === 'true',
    segmentHook: script.getAttribute('data-segment-hook') === 'true',
    randomizeUrl: script.getAttribute('data-randomize-url') === 'true',
    capture3rdPartyCookies:
      script.getAttribute('data-capture-3rd-party-cookies') === 'false'
        ? false
        : undefined,
    idMethod:
      (script.getAttribute('data-id-method') as 'cookie' | 'localStorage') ||
      undefined,
    privacyPolicy:
      script.getAttribute('data-privacy-policy') === 'strict'
        ? 'strict'
        : undefined,
    ipPolicy:
      (script.getAttribute('data-ip-policy') as Config['ipPolicy']) ||
      undefined,
    cookiePolicy:
      (script.getAttribute('data-cookie-policy') as Config['cookiePolicy']) ||
      undefined,
    minSendTimeout:
      parseInt(script.getAttribute('data-min-send-timeout') || '', 10) ||
      undefined,
    maxSendTimeout:
      parseInt(script.getAttribute('data-max-send-timeout') || '', 10) ||
      undefined,
    maxSendAttempts:
      parseInt(script.getAttribute('data-max-send-attempts') || '', 10) ||
      undefined,
    propertiesStringMaxLength:
      parseInt(
        script.getAttribute('data-properties-string-max-length') || '',
        10,
      ) || null,
    propertyBlacklist:
      script.getAttribute('data-property-blacklist')?.split(',') || undefined,
    exclude: script.getAttribute('data-exclude') || undefined,
    namespace: script.getAttribute('data-namespace') || undefined,
    crossDomainLinking:
      script.getAttribute('data-cross-domain-linking') !== 'false',
    domains: script.getAttribute('data-domains') || undefined,
  };

  // Additional config for strict mode
  if (config.privacyPolicy === 'strict') {
    config.cookiePolicy = 'strict';
    config.ipPolicy = 'strict';
  }

  if (config.cookiePolicy === 'comply' && config.useBeaconApi) {
    config.cookiePolicy = 'strict';
  }

  const client = helpinClient(config);
  const namespace = config.namespace || 'helpin';

  // Only send pageview if auto-pageview is enabled (default behavior for script tag)
  if (isWindowAvailable()) {
    client.pageview();
  }

  initializeNamespacedClient(namespace, client);

  return client;
}

function initializeNamespacedClient(
  namespace: string,
  client: HelpinClient,
) {
  let isReady = false;
  let resolveReady: () => void;
  const readyPromise = new Promise<void>((resolve) => { resolveReady = resolve; });
  const queue: any[][] = [];
  const onLoadCallbacks: (() => void)[] = [];

  function processQueue() {
    while (queue.length > 0) {
      const item = queue.shift();
      if (item) {
        // Check if last item is a promise handler
        const lastItem = item[item.length - 1];
        const hasPromiseHandler =
          lastItem && typeof lastItem.resolve === 'function';

        if (hasPromiseHandler) {
          const promiseHandler = item.pop();
          try {
            const result = namespacedFunction.apply(null, item);
            if (result && typeof result.then === 'function') {
              result.then(promiseHandler.resolve).catch(promiseHandler.reject);
            } else {
              promiseHandler.resolve(result);
            }
          } catch (error) {
            promiseHandler.reject(error);
          }
        } else {
          try {
            namespacedFunction.apply(null, item);
          } catch (error) {
            console.error(`Helpin: Error processing queued command:`, error);
          }
        }
      }
    }
  }
  function executeOnLoadCallbacks() {
    onLoadCallbacks.forEach((callback) => callback());
    onLoadCallbacks.length = 0;
  }

  // Create the main function that handles command-style calls
  function namespacedFunction(...args: any[]) {
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
    }

    // Widget methods
    const widgetMethods: Record<string, Function> = {
      boot: (settings: WidgetSettings) => widgetManager.boot(settings),
      shutdown: () => widgetManager.shutdown(),
      show: () => widgetManager.show(),
      hide: () => widgetManager.hide(),
      showMessages: () => widgetManager.showMessages(),
      showNewMessage: (content?: string) => widgetManager.showNewMessage(content),
      showConversation: (id: string) => widgetManager.showConversation(id),
      showArticle: (id: string) => widgetManager.showArticle(id),
      onShow: (cb: (...args: any[]) => void) => widgetManager.onShow(cb),
      onHide: (cb: (...args: any[]) => void) => widgetManager.onHide(cb),
      onUnreadCountChange: (cb: (...args: any[]) => void) => widgetManager.onUnreadCountChange(cb),
      onUserEmailSupplied: (cb: (...args: any[]) => void) => widgetManager.onUserEmailSupplied(cb),
      getVisitorId: () => widgetManager.getVisitorId(),
    };

    if (widgetMethods[method]) {
      return widgetMethods[method].apply(widgetManager, args.slice(1));
    } else {
      console.error(`Method ${method} not found on HelpinClient`);
    }
  }

  const asyncMethods = ['id', 'group', 'reset']; // These typically return Promise<void>
  const syncMethods = [
    'track',
    'lead',
    'pageview',
    'set',
    'unset',
    'rawTrack',
    'setUserId',
  ]; // These typically return void
  const methods = [...asyncMethods, ...syncMethods];
  methods.forEach((method) => {
    namespacedFunction[method] = function (...args: any[]) {
      if (!isReady) {
        if (asyncMethods.includes(method)) {
          return new Promise((resolve, reject) => {
            queue.push([method, ...args, { resolve, reject }]);
          });
        } else {
          queue.push([method, ...args]);
          return; // Returns undefined for sync methods
        }
      }

      if (typeof client[method] === 'function') {
        return client[method].apply(client, args);
      }
    };
  });
  // Add getConfig as a special case since it's a getter
  namespacedFunction.getConfig = function () {
    if (!isReady) {
      console.warn('Helpin client not ready yet');
      return null;
    }
    return client.getConfig();
  };

  // Set the function on the window
  (window as any)[namespace] = namespacedFunction;

  // Initialize queue processing
  const queueName = `${namespace}Q`;
  const existingQueue = (window as any)[queueName] || [];
  (window as any)[queueName] = existingQueue;

  existingQueue.push = function (...args: any[]) {
    namespacedFunction.apply(null, args);
    return Array.prototype.push.apply(this, args);
  };

  // Set client as ready and process any queued items
  Promise.resolve().then(() => {
    isReady = true;
    resolveReady();
    processQueue();
    executeOnLoadCallbacks();
    console.log(`Helpin client for namespace ${namespace} is ready`);
  });

  // Process any existing queue items
  while (existingQueue.length > 0) {
    const item = existingQueue.shift();
    if (item) {
      queue.push(item);
    }
  }
}

// Track initialization state
let isInitialized = false;
let scriptTagClient: HelpinClient | null = null;

// AMD Support
if (isWindowAvailable()) {
  const amdDefine = getAMDDefine();

  if (amdDefine) {
    // Define as AMD module - only exports, no initialization
    amdDefine('helpin', [], function () {
      return {
        helpinClient,
        HelpinClient,
        LogLevel,
        // Expose the script tag client if it exists
        getScriptTagClient: () => scriptTagClient,
      };
    });
  }

  // Always expose to global scope for script tag usage
  if (typeof window !== 'undefined') {
    (window as any).helpinClient = helpinClient;
    (window as any).HelpinClient = HelpinClient;
    (window as any).helpinScriptTagClient = () => scriptTagClient;
  }

  // Browser-specific initialization for script tag
  // Only initialize if loaded via script tag AND not within an AMD context
  (function (document, window) {
    // Capture the current script
    const currentScript = document.currentScript as HTMLScriptElement;

    function shouldAutoInitialize() {
      // Don't auto-initialize if:
      // 1. Already initialized
      // 2. Script doesn't have data attributes (likely loaded for AMD use)
      // 3. Explicitly disabled via data attribute

      if (isInitialized) return false;
      if (!currentScript) return false;
      if (!currentScript.hasAttribute('data-key')) return false;
      if (currentScript.getAttribute('data-no-auto-init') === 'true')
        return false;

      return currentScript.src.includes('lib.js');
    }

    function initialize() {
      if (shouldAutoInitialize()) {
        console.log('[Helpin] Auto-initializing from script tag');
        scriptTagClient = initFromScript(currentScript!);
        isInitialized = true;
      }
    }

    // Check if we're in a browser environment
    if (typeof window !== 'undefined' && currentScript) {
      // If the scripts is loaded with defer or async, we need to wait for the DOM to be ready
      if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', initialize);
      } else {
        initialize();
      }
    }
  })(document, window);
}

// For CommonJS/Node.js environments
if (typeof module !== 'undefined' && module.exports && !isAMDEnvironment()) {
  module.exports = {
    helpinClient,
    HelpinClient,
    Config: undefined as any,
    UserProps: undefined as any,
    EventPayload: undefined as any,
    LogLevel,
    ClientProperties: undefined as any,
  };
}

// For ES modules
export {
  helpinClient,
  HelpinClient,
  Config as HelpinOptions,
  UserProps,
  EventPayload,
  LogLevel,
  ClientProperties,
};
