import type { ReactNode } from 'react';
import { formatDistanceToNowStrict } from 'date-fns';

import { AgentAvatar } from '@/components/agents/AgentAvatar';
import { UserAvatar } from '@/components/pm/UserAvatar';
import type { AgentPresetKey } from '@/lib/pmTypes';

export interface UpdateActivityActor {
  email: string;
  full_name: string;
  avatar_url?: string | null;
  avatar_style?: string | null;
  avatar_seed?: string | null;
  avatar_background_mode?: string | null;
  avatar_background_color?: string | null;
}

interface UpdateActivityAgent {
  name: string;
  presetKey?: AgentPresetKey;
}

interface UpdateActivityRowProps {
  label: string;
  occurredAt: string;
  emphasizedValues?: string[];
  detail?: string;
  humanActor?: UpdateActivityActor;
  agent?: UpdateActivityAgent;
  fallbackIcon?: ReactNode;
  actionLabel?: string;
  onClick?: () => void;
}

function compactUpdateTime(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  const diffMs = Date.now() - date.getTime();
  if (diffMs < 5_000) return 'now';
  const relative = formatDistanceToNowStrict(date, { addSuffix: false })
    .replace(/ seconds?/, 's')
    .replace(/ minutes?/, 'm')
    .replace(/ hours?/, 'h')
    .replace(/ days?/, 'd')
    .replace(/ weeks?/, 'w')
    .replace(/ months?/, 'mo')
    .replace(/ years?/, 'y');
  return `${relative} ago`;
}

function escapeRegExp(value: string) {
  return value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

export function EmphasizedActivityLabel({ label, values = [] }: { label: string; values?: string[] }) {
  const emphasizedValues = [...new Set(values.map((value) => value.trim()).filter((value) => value && value !== 'Agent'))]
    .sort((left, right) => right.length - left.length);
  if (emphasizedValues.length === 0) return label;

  const valueSet = new Set(emphasizedValues);
  const parts = label.split(new RegExp(`(${emphasizedValues.map(escapeRegExp).join('|')})`, 'g'));
  return parts.map((part, index) => valueSet.has(part)
    ? <span key={`${part}-${index}`} className="font-semibold text-foreground/90">{part}</span>
    : part);
}

export function UpdateActivityRow({
  label,
  occurredAt,
  emphasizedValues = [],
  detail,
  humanActor,
  agent,
  fallbackIcon,
  actionLabel,
  onClick,
}: UpdateActivityRowProps) {
  const humanActorName = humanActor?.full_name?.trim() || humanActor?.email?.trim() || '';
  const rowClassName = 'flex w-full items-center gap-3 px-2 py-3 text-left text-sm transition-colors';
  const content = (
    <>
      <span className="flex h-6 w-6 shrink-0 items-center justify-center">
        {humanActor ? (
          <UserAvatar
            name={humanActorName}
            avatarUrl={humanActor.avatar_url}
            avatarStyle={humanActor.avatar_style}
            avatarSeed={humanActor.avatar_seed}
            avatarBackgroundMode={humanActor.avatar_background_mode}
            avatarBackgroundColor={humanActor.avatar_background_color}
            className="h-4 w-4"
            fallbackClassName="text-[7px]"
          />
        ) : agent ? (
          <AgentAvatar
            name={agent.name}
            presetKey={agent.presetKey}
            className="h-5 w-5 rounded-none border-0 bg-transparent shadow-none"
            genericBare
          />
        ) : (
          <span className="flex h-6 w-6 items-center justify-center rounded-full bg-muted text-muted-foreground">
            {fallbackIcon ?? <span className="h-1.5 w-1.5 rounded-full bg-current" />}
          </span>
        )}
      </span>
      <span className="min-w-0 flex-1 truncate text-foreground/70">
        <EmphasizedActivityLabel label={label} values={emphasizedValues} />
        {detail ? <span className="text-muted-foreground"> — {detail}</span> : null}
      </span>
      {actionLabel ? <span className="shrink-0 text-xs text-muted-foreground">{actionLabel}</span> : null}
      <time className="w-16 shrink-0 whitespace-nowrap text-right text-xs text-muted-foreground">
        {compactUpdateTime(occurredAt)}
      </time>
    </>
  );

  if (onClick) {
    return (
      <button type="button" className={`${rowClassName} hover:bg-muted/30`} onClick={onClick}>
        {content}
      </button>
    );
  }

  return <div className={rowClassName}>{content}</div>;
}
