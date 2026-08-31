import type { ReactNode } from 'react';
import { QuietPageViewport } from '@/components/design-system/quiet';

export function AutomationRouteViewport({ children }: { children: ReactNode }) {
  return <QuietPageViewport>{children}</QuietPageViewport>;
}
