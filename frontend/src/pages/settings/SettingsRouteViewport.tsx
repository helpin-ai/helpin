import type { ReactNode } from 'react';

export function SettingsRouteViewport({ children }: { children: ReactNode }) {
  return (
    <div className="h-full overflow-auto p-4 pb-32 md:p-6 md:pb-32">
      {children}
    </div>
  );
}
