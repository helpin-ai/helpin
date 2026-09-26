import { useSyncExternalStore } from 'react';

function subscribe(onChange: () => void) {
  window.addEventListener('online', onChange);
  window.addEventListener('offline', onChange);
  return () => {
    window.removeEventListener('online', onChange);
    window.removeEventListener('offline', onChange);
  };
}

/** Browser connectivity only; does not infer server or run health. */
export function useBrowserOnline() {
  return useSyncExternalStore(subscribe, () => navigator.onLine !== false, () => true);
}
