import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar';
import { resolveTeamMemberAvatarSrc } from '@/lib/teamMemberAvatar';
import { cn, getInitials } from '@/lib/utils';

const AVATAR_COLORS = [
  { bg: 'bg-rose-100 dark:bg-rose-900/40', text: 'text-rose-700 dark:text-rose-300' },
  { bg: 'bg-pink-100 dark:bg-pink-900/40', text: 'text-pink-700 dark:text-pink-300' },
  { bg: 'bg-fuchsia-100 dark:bg-fuchsia-900/40', text: 'text-fuchsia-700 dark:text-fuchsia-300' },
  { bg: 'bg-purple-100 dark:bg-purple-900/40', text: 'text-purple-700 dark:text-purple-300' },
  { bg: 'bg-indigo-100 dark:bg-indigo-900/40', text: 'text-indigo-700 dark:text-indigo-300' },
  { bg: 'bg-blue-100 dark:bg-blue-900/40', text: 'text-blue-700 dark:text-blue-300' },
  { bg: 'bg-teal-100 dark:bg-teal-900/40', text: 'text-teal-700 dark:text-teal-300' },
  { bg: 'bg-emerald-100 dark:bg-emerald-900/40', text: 'text-emerald-700 dark:text-emerald-300' },
  { bg: 'bg-lime-100 dark:bg-lime-900/40', text: 'text-lime-700 dark:text-lime-300' },
  { bg: 'bg-amber-100 dark:bg-amber-900/40', text: 'text-amber-700 dark:text-amber-300' },
  { bg: 'bg-orange-100 dark:bg-orange-900/40', text: 'text-orange-700 dark:text-orange-300' },
  { bg: 'bg-red-100 dark:bg-red-900/40', text: 'text-red-700 dark:text-red-300' },
];

function hashName(name: string): number {
  let hash = 0;
  for (let i = 0; i < name.length; i++) {
    hash = ((hash << 5) - hash + name.charCodeAt(i)) | 0;
  }
  return Math.abs(hash);
}

export function getAvatarColor(name?: string | null) {
  if (!name) return AVATAR_COLORS[0];
  return AVATAR_COLORS[hashName(name) % AVATAR_COLORS.length];
}

function bumpAvatarDimensions(className?: string) {
  if (!className) return className;

  const sizeMap: Record<string, string> = {
    'h-4': 'h-5',
    'w-4': 'w-5',
    'h-5': 'h-6',
    'w-5': 'w-6',
    'h-6': 'h-7',
    'w-6': 'w-7',
    'h-7': 'h-8',
    'w-7': 'w-8',
    'h-8': 'h-9',
    'w-8': 'w-9',
  };

  return className
    .split(/\s+/)
    .map((token) => sizeMap[token] ?? token)
    .join(' ');
}

interface UserAvatarProps {
  name?: string | null;
  avatarUrl?: string | null;
  avatarStyle?: string | null;
  avatarSeed?: string | null;
  avatarBackgroundMode?: string | null;
  avatarBackgroundColor?: string | null;
  presenceStatus?: 'online' | 'away' | 'offline' | null;
  className?: string;
  fallbackClassName?: string;
}

export function UserAvatar({
  name,
  avatarUrl,
  avatarStyle,
  avatarSeed,
  avatarBackgroundMode,
  avatarBackgroundColor,
  presenceStatus,
  className,
  fallbackClassName,
}: UserAvatarProps) {
  const color = getAvatarColor(name);
  const avatarClassName = bumpAvatarDimensions(className);
  const resolvedAvatarUrl = resolveTeamMemberAvatarSrc({
    avatarUrl,
    avatarStyle,
    avatarSeed,
    avatarBackgroundMode,
    avatarBackgroundColor,
    fallbackSeed: name,
  });

  return (
    <span className="relative inline-flex shrink-0">
      <Avatar className={cn('h-7 w-7 border border-border/80', avatarClassName)}>
        <AvatarImage src={resolvedAvatarUrl} alt={name ?? ''} />
        <AvatarFallback className={cn('text-[9px] font-semibold', color.bg, color.text, fallbackClassName)}>
          {getInitials(name)}
        </AvatarFallback>
      </Avatar>
      {presenceStatus === 'online' ? (
        <span className="absolute bottom-0 right-0 h-2.5 w-2.5 rounded-full border border-background bg-emerald-500" />
      ) : null}
    </span>
  );
}
