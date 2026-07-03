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

export const contactHeaderAvatarClassName = 'h-14 w-14 shrink-0';
export const contactHeaderNameClassName = '-ml-1 block min-w-0 truncate rounded-md px-1 text-left text-[24px] font-semibold leading-[1.08] text-foreground transition-colors hover:bg-muted/50';
export const contactHeaderNameInputClassName = '-ml-1 block w-full min-w-0 rounded-md bg-transparent px-1 text-[24px] font-semibold leading-[1.08] text-foreground outline-none ring-1 ring-inset ring-transparent focus:ring-border';
export const contactHeaderLifecycleBadgeClassName = 'inline-flex h-5 items-center gap-1.5 rounded-full border border-border/60 px-2 text-[11px] font-medium text-muted-foreground';

function splitFullName(value: string): [string, string] {
  const trimmed = value.trim().replace(/\s+/g, ' ');
  if (!trimmed) return ['', ''];
  const idx = trimmed.indexOf(' ');
  if (idx === -1) return [trimmed, ''];
  return [trimmed.slice(0, idx), trimmed.slice(idx + 1)];
}

export function getContactHeaderSubtitleParts(jobTitle?: string, companyName?: string): string[] {
  return [jobTitle, companyName].map((value) => value?.trim()).filter((value): value is string => Boolean(value));
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
  const subtitleParts = getContactHeaderSubtitleParts(jobTitle, companyName);

  return (
    <div className="px-8 pb-5 pt-6">
      <div className="flex items-center gap-4">
        <UserAvatar
          name={initialName || 'Untitled'}
          className={contactHeaderAvatarClassName}
          fallbackClassName="text-lg font-semibold"
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
              className={contactHeaderNameInputClassName}
              style={{ maxWidth: '30rem' }}
            />
          ) : (
            <button
              type="button"
              onClick={() => setEditing(true)}
              className={contactHeaderNameClassName}
            >
              {display}
            </button>
          )}

          <div className="mt-2 flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1 text-[13px] text-muted-foreground">
            {subtitleParts.length > 0 ? (
              <span className="min-w-0 truncate">
                {subtitleParts.map((part, index) => (
                  <span key={`${part}-${index}`}>
                    {index > 0 && <span className="mx-1.5 text-muted-foreground/60">at</span>}
                    <span className={index > 0 ? 'text-foreground/75' : undefined}>{part}</span>
                  </span>
                ))}
              </span>
            ) : (
              <span className="text-muted-foreground/70">No title or company</span>
            )}
            <span className={contactHeaderLifecycleBadgeClassName}>
              <span className={cn('size-1.5 rounded-full', stageDot)} />
              {lifecycleLabel}
            </span>
          </div>
        </div>

      </div>
    </div>
  );
}
