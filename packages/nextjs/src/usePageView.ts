import { useState, useEffect, useCallback, useRef } from 'react';
import { EventPayload, HelpinClient } from '@helpin-ai/sdk-js';

// Type for the hook options
interface UsePageViewOptions {
  before?: (helpin: HelpinClient) => void;
  typeName?: string;
  payload?: EventPayload;
}

// Custom hook to track URL changes safely
function useUrlChange() {
  const [url, setUrl] = useState<string>('');
  const isClient = typeof window !== 'undefined';

  useEffect(() => {
    if (!isClient) return;

    // Initialize with current URL
    setUrl(window.location.href);

    const handleUrlChange = () => {
      setUrl(window.location.href);
    };

    // Store original history methods
    const history = window.history;
    const originalPushState = history.pushState.bind(history);
    const originalReplaceState = history.replaceState.bind(history);

    // Wrap history methods
    const wrapHistoryMethod = (original: Function) => {
      return function (this: History, ...args: any[]) {
        const result = original.apply(this, args);
        handleUrlChange();
        return result;
      };
    };

    // Replace history methods
    history.pushState = wrapHistoryMethod(originalPushState);
    history.replaceState = wrapHistoryMethod(originalReplaceState);

    // Add popstate listener
    window.addEventListener('popstate', handleUrlChange);

    // Cleanup
    return () => {
      window.removeEventListener('popstate', handleUrlChange);
      history.pushState = originalPushState;
      history.replaceState = originalReplaceState;
    };
  }, [isClient]);

  return url;
}

// usePageView hook
function usePageView(
  helpin: HelpinClient | null,
  opts: UsePageViewOptions = {},
): HelpinClient | null {
  const url = useUrlChange();
  const isClient = typeof window !== 'undefined';
  const lastTrackedUrl = useRef<string>('');

  const trackPageView = useCallback(() => {
    if (!isClient || !helpin) return;

    // Get current URL
    const currentUrl = window.location.href;

    // Prevent duplicate tracking of the same URL
    if (lastTrackedUrl.current === currentUrl) return;
    lastTrackedUrl.current = currentUrl;

    // Execute before callback if provided
    if (opts.before) {
      opts.before(helpin);
    }

    // Track the page view
    try {
      helpin.track(opts?.typeName || 'pageview', {
        ...opts.payload,
        url: currentUrl,
        path: window.location.pathname,
        referrer: document.referrer || '',
        title: document.title,
        timestamp: new Date().toISOString(),
      });
    } catch (error) {
      // Silently handle errors in production
      console.warn('Helpin pageview tracking error:', error);
    }
  }, [helpin, opts.before, opts.typeName, opts.payload, isClient]);

  useEffect(() => {
    if (url) {
      trackPageView();
    }
  }, [url, trackPageView]);

  return helpin;
}

export default usePageView;
