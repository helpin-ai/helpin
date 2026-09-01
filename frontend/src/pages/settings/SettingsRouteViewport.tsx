import type { ReactNode } from 'react';
import { QuietPageViewport } from '@/components/design-system/quiet';

export function SettingsRouteViewport({ children }: { children: ReactNode }) {
  return (
    <QuietPageViewport className="pb-32 md:pb-32">
      {children}
    </QuietPageViewport>
  );
}
