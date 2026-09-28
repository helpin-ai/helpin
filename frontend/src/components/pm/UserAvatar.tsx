import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar';
import { ContactAvatarImage } from '@/components/ui/contact-avatar-image';
import { resolveTeamMemberAvatarSrc } from '@/lib/teamMemberAvatar';
import { cn, getInitials } from '@/lib/utils';

const AVATAR_COLORS = [
  { bg: 'bg-[#e8ecf7] dark:bg-indigo-950/55', text: 'text-[#4c5a86] dark:text-indigo-200' },
  { bg: 'bg-[#eef0f6] dark:bg-slate-800', text: 'text-[#4c5a86] dark:text-slate-200' },
  { bg: 'bg-[#e6f0ec] dark:bg-emerald-950/50', text: 'text-[#3f6b58] dark:text-emerald-200' },
  { bg: 'bg-[#eeeaf7] dark:bg-violet-950/50', text: 'text-[#5b4c86] dark:text-violet-200' },
  { bg: 'bg-[#f7ece6] dark:bg-orange-950/50', text: 'text-[#8a5433] dark:text-orange-200' },
  { bg: 'bg-[#eef2e6] dark:bg-lime-950/45', text: 'text-[#5c6b3f] dark:text-lime-200' },
];

const UNRESOLVED_AVATAR_COLOR = {
  bg: 'bg-[#f0efec] dark:bg-neutral-800',
  text: 'text-[#78716c] dark:text-neutral-300',
};

function hashName(name: string): number {
  let hash = 0;
  for (let i = 0; i < name.length; i++) {
    hash = ((hash << 5) - hash + name.charCodeAt(i)) | 0;
  }
  return Math.abs(hash);
}

export function getAvatarColor(name?: string | null) {
  if (!name) return UNRESOLVED_AVATAR_COLOR;
  return AVATAR_COLORS[hashName(name) % AVATAR_COLORS.length];
}

function getPresenceIndicatorClass(status?: 'online' | 'away' | 'offline' | null) {
  if (status === 'online') return 'bg-emerald-500';
  if (status === 'away') return 'bg-amber-500';
  return null;
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
  /** Opt contact avatars into Gravatar; team avatars remain unchanged. */
  email?: string | null;
  avatarUrl?: string | null;
  avatarStyle?: string | null;
  avatarSeed?: string | null;
  avatarBackgroundMode?: string | null;
  avatarBackgroundColor?: string | null;
  presenceStatus?: 'online' | 'away' | 'offline' | null;
  className?: string;
  fallbackClassName?: string;
  fallbackColorSeed?: string | null;
}

export function UserAvatar({
  name,
  email,
  avatarUrl,
  avatarStyle,
  avatarSeed,
  avatarBackgroundMode,
  avatarBackgroundColor,
  presenceStatus,
  className,
  fallbackClassName,
  fallbackColorSeed,
}: UserAvatarProps) {
  const color = getAvatarColor(fallbackColorSeed || name);
  const avatarClassName = bumpAvatarDimensions(className);
  const presenceIndicatorClass = getPresenceIndicatorClass(presenceStatus);
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
        {email ? (
          <ContactAvatarImage email={email} src={resolvedAvatarUrl} alt={name ?? ''} />
        ) : (
          <AvatarImage src={resolvedAvatarUrl} alt={name ?? ''} />
        )}
        <AvatarFallback className={cn('text-[9px] font-semibold', color.bg, color.text, fallbackClassName)}>
          {getInitials(name)}
        </AvatarFallback>
      </Avatar>
      {presenceIndicatorClass ? (
        <span
          className={cn('absolute bottom-0 right-0 h-2.5 w-2.5 rounded-full border border-background', presenceIndicatorClass)}
          data-presence-status={presenceStatus}
        />
      ) : null}
    </span>
  );
}
