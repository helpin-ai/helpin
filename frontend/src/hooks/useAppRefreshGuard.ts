import { useLayoutEffect, useRef } from 'react';
import { registerAppRefreshGuard } from '@/lib/appLoadRecovery';

export function useAppRefreshGuard(guard: () => string | null) {
  const latest = useRef(guard);
  latest.current = guard;
  useLayoutEffect(() => registerAppRefreshGuard(() => latest.current()), []);
}
