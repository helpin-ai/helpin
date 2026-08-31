import type { CRMMeetingPlatform } from '@/lib/crmMeetingTypes';
import { getMeetingPlatformLabel } from '@/lib/meetingPresentation';
import { cn } from '@/lib/utils';

export function MeetingPlatformIcon({
  platform,
  className,
  size = 'md',
  presentation = 'default',
}: {
  platform: CRMMeetingPlatform;
  className?: string;
  size?: 'sm' | 'md';
  presentation?: 'default' | 'quiet';
}) {
  const iconClassName = size === 'sm' ? 'h-3.5 w-3.5' : 'h-4 w-4';

  return (
    <span
      aria-label={getMeetingPlatformLabel(platform)}
      className={cn(
        'inline-flex shrink-0 items-center justify-center',
        presentation === 'default' && 'rounded-md border bg-background shadow-sm',
        presentation === 'quiet' && 'bg-transparent',
        size === 'sm' ? 'h-6 w-6' : 'h-8 w-8',
        className,
      )}
    >
      {platform === 'google_meet' && (
        <svg viewBox="0 0 24 24" className={iconClassName} aria-hidden="true">
          <path fill="#00832d" d="M3 6.5A2.5 2.5 0 0 1 5.5 4H14l3 3v10l-3 3H5.5A2.5 2.5 0 0 1 3 17.5z" />
          <path fill="#00ac47" d="M3 9h8v11H5.5A2.5 2.5 0 0 1 3 17.5z" />
          <path fill="#ffba00" d="M11 4h3l3 3-3 3h-3z" />
          <path fill="#0066da" d="M14 10l5.1-3.7c.8-.6 1.9 0 1.9 1v9.4c0 1-1.1 1.6-1.9 1L14 14z" />
        </svg>
      )}
      {platform === 'zoom' && (
        <svg viewBox="0 0 24 24" className={iconClassName} aria-hidden="true">
          <rect width="24" height="24" rx="6" fill="#2D8CFF" />
          <path fill="white" d="M5.5 8.2c0-.7.6-1.2 1.2-1.2h6.1c.7 0 1.2.5 1.2 1.2v7.6c0 .7-.5 1.2-1.2 1.2H6.7c-.6 0-1.2-.5-1.2-1.2zm9.4 2.5 2.7-2c.4-.3.9 0 .9.5v5.6c0 .5-.5.8-.9.5l-2.7-2z" />
        </svg>
      )}
      {platform === 'teams' && (
        <svg viewBox="0 0 24 24" className={iconClassName} aria-hidden="true">
          <rect x="5" y="6" width="13" height="13" rx="3" fill="#6264A7" />
          <circle cx="17.5" cy="5.5" r="2.5" fill="#7B83EB" />
          <rect x="2" y="8" width="10" height="10" rx="2" fill="#5059C9" />
          <path fill="white" d="M4.5 10h5v1.5H7.8V16H6.2v-4.5H4.5z" />
        </svg>
      )}
      {platform === 'webex' && (
        <svg viewBox="0 0 24 24" className={iconClassName} aria-hidden="true">
          <path d="M4 12a8 8 0 0 1 13.6-5.7" fill="none" stroke="#00BCEB" strokeWidth="3.2" strokeLinecap="round" />
          <path d="M20 12A8 8 0 0 1 6.4 17.7" fill="none" stroke="#6EBE44" strokeWidth="3.2" strokeLinecap="round" />
          <circle cx="12" cy="12" r="2.4" fill="#1F8ACB" />
        </svg>
      )}
    </span>
  );
}

export function MeetingPlatformLabel({
  platform,
  compact = false,
  presentation = 'default',
}: {
  platform: CRMMeetingPlatform;
  compact?: boolean;
  presentation?: 'default' | 'quiet';
}) {
  return (
    <span className="inline-flex min-w-0 items-center gap-2">
      <MeetingPlatformIcon platform={platform} size={compact ? 'sm' : 'md'} presentation={presentation} />
      <span className="truncate">{getMeetingPlatformLabel(platform)}</span>
    </span>
  );
}
