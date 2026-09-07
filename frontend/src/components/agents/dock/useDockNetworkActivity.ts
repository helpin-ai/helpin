import { useSyncExternalStore } from 'react';

export function isDockNetworkAvailable() {
  return document.visibilityState !== 'hidden' && navigator.onLine !== false;
}

function subscribe(onChange: () => void) {
  document.addEventListener('visibilitychange', onChange);
  window.addEventListener('online', onChange);
  window.addEventListener('offline', onChange);
  return () => {
    document.removeEventListener('visibilitychange', onChange);
    window.removeEventListener('online', onChange);
    window.removeEventListener('offline', onChange);
  };
}

/** Pause automatic reads in hidden or offline browsers; resume with a fresh read. */
export function useDockNetworkActivity() {
  return useSyncExternalStore(subscribe, isDockNetworkAvailable, () => false);
}
