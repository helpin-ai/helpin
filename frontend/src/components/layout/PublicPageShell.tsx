import type { ReactNode } from 'react';
import { HelpinLogo } from '@/components/layout/HelpinLogo';

interface PublicPageShellProps {
  children: ReactNode;
}

export function PublicPageShell({ children }: PublicPageShellProps) {
  return (
    <div className="min-h-screen flex items-center justify-center px-4">
      <div className="w-full max-w-md space-y-6">
        <HelpinLogo />
        {children}
      </div>
    </div>
  );
}
