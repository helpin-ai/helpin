import { useState, useEffect, useCallback, useRef } from 'react';
import useHelpin, { HelpinClient } from './useHelpin';
import { EventPayload } from '@helpin-ai/sdk-js';

function getCurrentUrl(): string {
  return typeof window === 'undefined' ? '' : window.location.href;
}

// Custom hook to track URL changes
function useUrlChange() {
  const [url, setUrl] = useState(getCurrentUrl);
  const lastUrlRef = useRef(getCurrentUrl());

  useEffect(() => {
    if (typeof window === 'undefined') {
      return;
    }

    const handleUrlChange = () => {
      const currentUrl = window.location.href;
      if (currentUrl !== lastUrlRef.current) {
        lastUrlRef.current = currentUrl;
        setUrl(currentUrl);
      }
    };

    window.addEventListener('popstate', handleUrlChange);

    // For handling pushState and replaceState
    const originalPushState = window.history.pushState;
    const originalReplaceState = window.history.replaceState;

    window.history.pushState = function () {
      originalPushState.apply(this, arguments as any);
      handleUrlChange();
    };

    window.history.replaceState = function () {
      originalReplaceState.apply(this, arguments as any);
      handleUrlChange();
    };

    return () => {
      window.removeEventListener('popstate', handleUrlChange);
      window.history.pushState = originalPushState;
      window.history.replaceState = originalReplaceState;
    };
  }, []);

  return url;
}

// usePageView hook
function usePageView(
  opts: {
    before?: (helpin: HelpinClient) => void;
    typeName?: string;
    payload?: EventPayload;
  } = {},
): HelpinClient {
  const url = useUrlChange();
  const helpin = useHelpin();
  const lastTrackedUrl = useRef('');

  const trackPageView = useCallback(() => {
    if (typeof window === 'undefined') {
      return;
    }

    if (url !== lastTrackedUrl.current) {
      if (opts.before) {
        opts.before(helpin);
      }
      helpin.track(opts?.typeName || 'pageview', {
        ...opts.payload,
        url: window.location.href,
        path: window.location.pathname,
        referrer: document.referrer,
        title: document.title,
      });
      lastTrackedUrl.current = url;
    }
  }, [helpin, url, opts.before, opts.typeName, opts.payload]);

  useEffect(() => {
    trackPageView();
  }, [url, trackPageView]);

  return helpin;
}

export default usePageView;
