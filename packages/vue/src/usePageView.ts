import { inject, onMounted, onUnmounted } from 'vue';
import type { EventPayload, HelpinClient } from '@helpin-ai/sdk-js';
import type { Router } from 'vue-router';
import { HelpinKey } from './injection';

export interface UsePageViewOptions {
  router?: Router;
  before?: (helpin: HelpinClient) => void;
  typeName?: string;
  payload?: EventPayload;
}

export default function usePageView(
  options: UsePageViewOptions = {},
): HelpinClient | null {
  const client = inject(HelpinKey, null);
  let removeRouterHook: (() => void) | undefined;
  let lastTrackedUrl = '';

  const trackPageView = (): void => {
    if (!client || typeof window === 'undefined') {
      return;
    }

    const currentUrl = window.location.href;
    if (currentUrl === lastTrackedUrl) {
      return;
    }
    lastTrackedUrl = currentUrl;

    options.before?.(client);
    client.track(options.typeName || 'pageview', {
      ...options.payload,
      url: currentUrl,
      path: window.location.pathname,
      referrer: document.referrer || '',
      title: document.title,
      timestamp: new Date().toISOString(),
    });
  };

  onMounted(() => {
    trackPageView();
    if (options.router) {
      removeRouterHook = options.router.afterEach(() => trackPageView());
    }
  });

  onUnmounted(() => removeRouterHook?.());

  return client;
}
