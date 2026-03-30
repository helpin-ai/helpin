import type { ReactNode } from 'react';

export function PreviewLayout({ children, preview }: { children: ReactNode; preview: ReactNode }) {
  return (
    <div className="grid h-full grid-cols-1 xl:grid-cols-[minmax(0,1fr)_420px]">
      <div className="flex min-w-0 flex-col overflow-hidden">
        {children}
      </div>

      <aside className="hidden self-start xl:sticky xl:top-0 xl:block">
        <div className="h-[calc(100svh-8rem)] min-h-[36rem] overflow-y-auto overscroll-contain">
          <div className="h-full pr-1">{preview}</div>
        </div>
      </aside>
    </div>
  );
}
