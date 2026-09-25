(() => {
  'use strict';

  const CONFIG_KEY = 'helpin_event_test_config_v1';
  const LOG_KEY = 'helpin_event_test_log_v1';
  const RUN_KEY = 'helpin_event_test_run_v1';
  const DEFAULT_HOST = 'https://helpin-dev.localhost';
  const page = document.body.dataset.page || 'control';
  const nativeFetch = window.fetch.bind(window);

  const readJSON = (key, fallback) => {
    try {
      return JSON.parse(localStorage.getItem(key) || '') || fallback;
    } catch {
      return fallback;
    }
  };

  const query = new URLSearchParams(window.location.search);
  const storedConfig = readJSON(CONFIG_KEY, {});
  const config = {
    widgetKey: query.get('key') || query.get('widget_key') || storedConfig.widgetKey || '',
    host: (query.get('host') || storedConfig.host || DEFAULT_HOST).replace(/\/+$/, ''),
  };

  if (config.widgetKey) {
    localStorage.setItem(CONFIG_KEY, JSON.stringify(config));
  }

  const runId = localStorage.getItem(RUN_KEY) || crypto.randomUUID();
  localStorage.setItem(RUN_KEY, runId);

  const logs = readJSON(LOG_KEY, []).slice(-39);
  const now = () => new Date().toLocaleTimeString([], { hour12: false });
  const redactKey = (value) => value ? `${value.slice(0, 5)}…${value.slice(-4)}` : 'not configured';

  const renderLogs = () => {
    const list = document.querySelector('[data-event-log]');
    if (!list) return;
    list.replaceChildren();
    if (!logs.length) {
      const item = document.createElement('li');
      item.className = 'console-empty';
      item.textContent = 'No browser events in this run yet.';
      list.append(item);
      return;
    }
    for (const entry of [...logs].reverse()) {
      const item = document.createElement('li');
      const time = document.createElement('time');
      const message = document.createElement('span');
      time.textContent = entry.time;
      message.textContent = entry.message;
      item.append(time, message);
      list.append(item);
    }
  };

  const log = (message) => {
    logs.push({ time: now(), message: String(message).slice(0, 180) });
    if (logs.length > 40) logs.shift();
    localStorage.setItem(LOG_KEY, JSON.stringify(logs));
    renderLogs();
  };

  const parseEventNames = (body) => {
    if (typeof body !== 'string') return ['event batch'];
    try {
      const parsed = JSON.parse(body);
      const events = Array.isArray(parsed) ? parsed : [parsed];
      return events.map((event) => event.event_type || 'event');
    } catch {
      return ['event batch'];
    }
  };

  window.fetch = async (input, init = {}) => {
    const url = typeof input === 'string' ? input : input.url;
    const isEventRequest = /\/api(?:\/v1\/event|\.)/.test(url);
    const isIdentifyRequest = /\/widget\/identify(?:\?|$)/.test(url);
    const names = isEventRequest ? parseEventNames(init.body) : [];
    if (isEventRequest) log(`Sending ${names.join(', ')}`);
    if (isIdentifyRequest) log('Sending CRM identify');
    try {
      const response = await nativeFetch(input, init);
      if (isEventRequest) log(`${response.ok ? 'Accepted' : 'Rejected'} ${names.join(', ')} · HTTP ${response.status}`);
      if (isIdentifyRequest) log(`${response.ok ? 'Accepted' : 'Rejected'} CRM identify · HTTP ${response.status}`);
      return response;
    } catch (error) {
      if (isEventRequest) log(`Network error sending ${names.join(', ')}`);
      if (isIdentifyRequest) log('Network error sending CRM identify');
      throw error;
    }
  };

  const setText = (selector, value) => {
    const element = document.querySelector(selector);
    if (element) element.textContent = value;
  };

  const setSDKState = (state, label) => {
    const element = document.querySelector('[data-sdk-state]');
    if (!element) return;
    element.className = `state ${state}`;
    element.textContent = label;
  };

  const refreshVisitor = () => {
    try {
      const cookieName = `helpin_aid_${config.widgetKey}`;
      const cookieValue = document.cookie
        .split('; ')
        .find((entry) => entry.startsWith(`${cookieName}=`))
        ?.slice(cookieName.length + 1);
      const visitorId = window.helpin?.('getVisitorId') || (cookieValue ? decodeURIComponent(cookieValue) : '');
      setText('[data-visitor-id]', visitorId || 'pending');
    } catch {
      setText('[data-visitor-id]', 'pending');
    }
  };

  const track = (eventName, properties = {}) => {
    if (!config.widgetKey || typeof window.helpin !== 'function') return;
    window.helpin('track', eventName, { test_run_id: runId, test_page: page, ...properties });
  };

  const initializeSDK = () => {
    if (!config.widgetKey) {
      setSDKState('waiting', 'Add a widget key');
      log('SDK paused · widget key is not configured');
      return;
    }

    setSDKState('waiting', 'Loading local SDK');
    const script = document.createElement('script');
    script.src = `${config.host}/sdk/lib.js`;
    script.dataset.widgetKey = config.widgetKey;
    script.dataset.host = config.host;
    script.dataset.noAutoInit = 'true';
    script.dataset.namespace = 'helpin';
    script.addEventListener('error', () => {
      setSDKState('error', 'SDK load failed');
      log('SDK load failed');
    });
    script.addEventListener('load', () => {
      window.helpin('init', {
        widgetKey: config.widgetKey,
        host: config.host,
        namespace: 'helpin',
        autoBoot: false,
        autoPageview: false,
        forceUseFetch: true,
        cookiePolicy: 'keep',
        ipPolicy: 'keep',
        formCapture: [
          { selector: '#demo-request', formId: 'demo-request', fields: ['email', 'company', 'role'] },
        ],
        interactionCaptureRules: [
          {
            ruleKey: 'versioned_interaction',
            version: 1,
            clickSelectors: ['[data-helpin-intent="pricing"]'],
            scrollMilestones: [75],
          },
        ],
      });
      window.helpin('set', { test_run_id: runId, test_surface: 'event-lab' });
      track('pageview', { route: window.location.pathname });
      if (page === 'security') {
        window.helpin('articleView', 'security-overview', { test_run_id: runId, section: 'trust-center' });
      }
      const startedAt = Date.now();
      const waitForClient = window.setInterval(() => {
        if (window.helpinScriptTagClient?.()) {
          window.clearInterval(waitForClient);
          setSDKState('ready', 'SDK ready');
          log(`SDK ready · ${page} pageview queued`);
          refreshVisitor();
        } else if (Date.now() - startedAt > 15_000) {
          window.clearInterval(waitForClient);
          setSDKState('error', 'SDK initialization timed out');
          log('SDK initialization timed out');
        }
      }, 50);
    });
    document.head.append(script);
  };

  const configure = () => {
    const form = document.querySelector('[data-config-form]');
    if (!form) return;
    const keyInput = form.elements.namedItem('widget_key');
    const hostInput = form.elements.namedItem('host');
    keyInput.value = config.widgetKey;
    hostInput.value = config.host;
    form.addEventListener('submit', (event) => {
      event.preventDefault();
      const widgetKey = keyInput.value.trim();
      const host = hostInput.value.trim().replace(/\/+$/, '');
      if (!widgetKey || !host) {
        setText('[data-config-message]', 'Enter both the public widget key and API host.');
        return;
      }
      localStorage.setItem(CONFIG_KEY, JSON.stringify({ widgetKey, host }));
      setText('[data-config-message]', 'Saved. Reloading the event lab…');
      window.location.reload();
    });
  };

  const wireActions = () => {
    document.querySelector('[data-track-pageview]')?.addEventListener('click', () => {
      track('pageview', { route: window.location.pathname, manual: true });
      log('Manual pageview queued');
    });

    document.querySelector('[data-identify]')?.addEventListener('click', async () => {
      const identity = {
        id: `claimed-${runId}`,
        email: `claimed+${runId.slice(0, 8)}@example.test`,
        first_name: 'Test',
        last_name: 'Buyer',
        company: {
          id: `account-${runId}`,
          name: 'Example Test Account',
          created_at: new Date().toISOString(),
        },
      };
      try {
        await window.helpin('id', identity);
        log('Claimed browser identity queued · intentionally unverified');
        setText('[data-identity-state]', 'browser_claim / untrusted');
      } catch {
        log('Identity command failed');
      }
    });

    document.querySelector('[data-reset]')?.addEventListener('click', async () => {
      try {
        await window.helpin('reset', true);
        log('Identity and anonymous visitor reset');
        refreshVisitor();
        setText('[data-identity-state]', 'anonymous / untrusted');
      } catch {
        log('Reset command failed');
      }
    });

    document.querySelector('[data-open-widget]')?.addEventListener('click', () => {
      window.helpin('open');
      log('Chat widget opened');
    });

    document.querySelector('[data-clear-log]')?.addEventListener('click', () => {
      logs.splice(0);
      localStorage.removeItem(LOG_KEY);
      renderLogs();
    });

    document.querySelector('[data-helpin-intent="pricing"]')?.addEventListener('click', () => {
      track('pricing_cta_clicked', { plan: 'scale' });
      log('Pricing CTA selected · configured interaction listener also active');
    });

    document.querySelector('#demo-request')?.addEventListener('submit', (event) => {
      event.preventDefault();
      const form = event.currentTarget;
      track('demo_requested', { form_id: 'demo-request', role: form.elements.namedItem('role').value });
      setText('[data-form-message]', 'Captured locally. Use the console and ClickHouse query to verify the event.');
      log('Demo form submitted · allowlisted configured capture active');
    });
  };

  setText('[data-configured-key]', redactKey(config.widgetKey));
  setText('[data-configured-host]', config.host);
  setText('[data-run-id]', runId);
  setText('[data-identity-state]', 'anonymous / untrusted');
  renderLogs();
  configure();
  wireActions();
  initializeSDK();
})();
