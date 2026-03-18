import { Hash } from 'lucide-react';

export function StorySidebarIdRow({ displayId }: { displayId: string | number }) {
  return (
    <div className="mb-3 grid grid-cols-[16px_72px_1fr] items-center gap-x-2 gap-y-2.5">
      <Hash className="h-4 w-4 text-muted-foreground" />
      <span className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
        ID
      </span>
      <span className="font-mono text-sm text-foreground">{displayId}</span>
    </div>
  );
}
