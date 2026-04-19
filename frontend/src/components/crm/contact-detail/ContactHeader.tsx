import { useEffect, useRef, useState } from 'react';
import { UserAvatar } from '@/components/pm/UserAvatar';
import { cn } from '@/lib/utils';
import type { LifecycleStage } from '@/lib/crmTypes';

interface ContactHeaderProps {
  firstName: string;
  lastName: string;
  jobTitle?: string;
  companyName?: string;
  companyHref?: string;
  lifecycleStage: LifecycleStage;
  lifecycleLabel: string;
  onNameChange: (first: string, last: string) => void;
}

const STAGE_DOT: Record<LifecycleStage, string> = {
  subscriber: 'bg-sky-500',
  lead: 'bg-indigo-500',
  marketing_qualified: 'bg-violet-500',
  sales_qualified: 'bg-amber-500',
  opportunity: 'bg-orange-500',
  customer: 'bg-emerald-500',
  evangelist: 'bg-pink-500',
};

function splitFullName(value: string): [string, string] {
  const trimmed = value.trim().replace(/\s+/g, ' ');
  if (!trimmed) return ['', ''];
  const idx = trimmed.indexOf(' ');
  if (idx === -1) return [trimmed, ''];
  return [trimmed.slice(0, idx), trimmed.slice(idx + 1)];
}

export function ContactHeader({
  firstName,
  lastName,
  jobTitle,
  companyName,
  companyHref: _companyHref,
  lifecycleStage,
  lifecycleLabel,
  onNameChange,
}: ContactHeaderProps) {
  const initialName = `${firstName ?? ''} ${lastName ?? ''}`.trim();
  const [draft, setDraft] = useState(initialName);
  const [editing, setEditing] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (!editing) setDraft(initialName);
  }, [editing, initialName]);

  const commit = () => {
    const [nextFirst, nextLast] = splitFullName(draft);
    if (nextFirst !== (firstName ?? '') || nextLast !== (lastName ?? '')) {
      onNameChange(nextFirst, nextLast);
    }
    setEditing(false);
  };

  const display = initialName || 'Untitled contact';
  const stageDot = STAGE_DOT[lifecycleStage] ?? 'bg-muted-foreground';

  return (
    <div className="px-6 pb-4 pt-5">
      <div className="flex items-center gap-4">
        <UserAvatar
          name={initialName || 'Untitled'}
          className="h-11 w-11 shrink-0"
          fallbackClassName="text-sm font-semibold"
        />

        <div className="min-w-0 flex-1">
          {editing ? (
            <input
              ref={inputRef}
              autoFocus
              value={draft}
              onChange={(e) => setDraft(e.target.value)}
              onBlur={commit}
              onKeyDown={(e) => {
                if (e.key === 'Enter') {
                  e.preventDefault();
                  commit();
                } else if (e.key === 'Escape') {
                  e.preventDefault();
                  setDraft(initialName);
                  setEditing(false);
                }
              }}
              placeholder="Full name"
              className="-ml-1 block w-full min-w-0 rounded-md bg-transparent px-1 text-[22px] font-semibold leading-tight tracking-[-0.01em] text-foreground outline-none ring-1 ring-inset ring-transparent focus:ring-border"
              style={{ maxWidth: '24rem' }}
            />
          ) : (
            <button
              type="button"
              onClick={() => setEditing(true)}
              className="-ml-1 block min-w-0 truncate rounded-md px-1 text-left text-[22px] font-semibold leading-tight tracking-[-0.01em] text-foreground transition-colors hover:bg-muted/50"
            >
              {display}
            </button>
          )}

          <div className="mt-1.5 flex flex-wrap items-center gap-x-2 gap-y-0.5 text-[13px] text-muted-foreground">
            {jobTitle && <span className="truncate">{jobTitle}</span>}
            {jobTitle && companyName && <Bullet />}
            {companyName && (
              <span className="truncate text-foreground/75">{companyName}</span>
            )}
            {(jobTitle || companyName) && <Bullet />}
            <span className="inline-flex items-center gap-1.5">
              <span className={cn('size-1.5 rounded-full', stageDot)} />
              {lifecycleLabel}
            </span>
          </div>
        </div>

      </div>
    </div>
  );
}

function Bullet() {
  return <span className="size-0.5 rounded-full bg-muted-foreground/60" />;
}
