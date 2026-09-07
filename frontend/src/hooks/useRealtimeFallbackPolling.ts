import { useSyncExternalStore } from 'react';
import { useSupportPresenceStore } from '@/stores/supportPresenceStore';

export function isForegroundNetworkAvailable() {
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

/** Realtime handles updates; visible, online queries periodically recover missed events. */
export function useRealtimeFallbackPolling(enabled = true) {
  const available = useSyncExternalStore(subscribe, isForegroundNetworkAvailable, () => false);
  const connected = useSupportPresenceStore((state) => state.wsConnected);
  const active = enabled && available;
  return {
    enabled: active,
    refetchInterval: active ? (connected ? 120_000 : 30_000) : false,
    refetchIntervalInBackground: false,
  } as const;
}
