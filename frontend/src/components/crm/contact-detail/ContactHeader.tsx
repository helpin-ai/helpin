import { useRef, useState, type ReactNode } from 'react';
import { QuietIdentityHeader, QuietMetaLine, QuietStatusText } from '@/components/design-system/quiet';
import { UserAvatar } from '@/components/pm/UserAvatar';
import type { LifecycleStage } from '@/lib/crmTypes';

interface ContactHeaderProps {
  firstName: string;
  lastName: string;
  jobTitle?: string;
  companyName?: string;
  companyHref?: string;
  lifecycleStage: LifecycleStage;
  lifecycleLabel: string;
  displayId?: string;
  avatarColorSeed?: string;
  actions?: ReactNode;
  onNameChange: (first: string, last: string) => void;
}

export const contactHeaderAvatarClassName = 'h-10 w-10 shrink-0';
export const contactHeaderNameClassName = 'block min-w-0 truncate border-b border-transparent text-left text-[26px] font-semibold leading-[1.15] tracking-[-0.02em] text-quiet-text-primary transition-colors hover:border-quiet-field focus-visible:border-quiet-text-primary focus-visible:outline-none md:text-[26px]';
export const contactHeaderNameInputClassName = 'block w-full min-w-0 border-0 border-b-2 border-quiet-text-primary bg-transparent pb-0.5 text-[26px] font-semibold leading-[1.15] tracking-[-0.02em] text-quiet-text-primary outline-none placeholder:text-quiet-muted md:text-[26px]';
export const contactHeaderLifecycleBadgeClassName = 'inline-flex items-center gap-1.5 text-[11.5px] font-semibold uppercase tracking-[0.03em] text-quiet-text-tertiary';

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
  companyHref,
  lifecycleLabel,
  displayId,
  avatarColorSeed,
  actions,
  onNameChange,
}: ContactHeaderProps) {
  const initialName = `${firstName ?? ''} ${lastName ?? ''}`.trim();
  const [draft, setDraft] = useState(initialName);
  const [editing, setEditing] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);

  const commit = () => {
    const [nextFirst, nextLast] = splitFullName(draft);
    if (nextFirst !== (firstName ?? '') || nextLast !== (lastName ?? '')) {
      onNameChange(nextFirst, nextLast);
    }
    setEditing(false);
  };

  const display = initialName || 'Untitled contact';
  const subtitleParts = getContactHeaderSubtitleParts(jobTitle, companyName);

  return (
    <QuietIdentityHeader
      className="lg:px-10"
      avatar={(
        <UserAvatar
          name={initialName || 'Untitled'}
          fallbackColorSeed={avatarColorSeed}
          className={contactHeaderAvatarClassName}
          fallbackClassName="text-sm font-semibold"
        />
      )}
      title={(
        <div className="max-w-[30rem]">
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
            />
          ) : (
            <button
              type="button"
              onClick={() => {
                setDraft(initialName);
                setEditing(true);
              }}
              className={contactHeaderNameClassName}
            >
              {display}
            </button>
          )}
        </div>
      )}
      meta={(
        <QuietMetaLine items={[
          subtitleParts.length > 0 ? (
            <span className="min-w-0 truncate">
              {subtitleParts.map((part, index) => {
                const isCompany = part === companyName?.trim();
                return (
                  <span key={`${part}-${index}`}>
                    {index > 0 ? <span className="mx-1.5 text-quiet-muted">at</span> : null}
                    {isCompany && companyHref ? (
                      <a href={companyHref} className="text-quiet-text-secondary transition-colors hover:text-quiet-text-primary hover:underline">{part}</a>
                    ) : <span>{part}</span>}
                  </span>
                );
              })}
            </span>
          ) : 'No title or company',
          displayId ? <span className="font-mono">{displayId}</span> : null,
        ]} />
      )}
      status={<QuietStatusText tone="lifecycle" className={contactHeaderLifecycleBadgeClassName}>{lifecycleLabel}</QuietStatusText>}
      actions={actions}
    />
  );
}
