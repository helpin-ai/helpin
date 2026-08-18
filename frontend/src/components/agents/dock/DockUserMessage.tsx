import { UserAvatar } from '@/components/pm/UserAvatar';
import { MarkdownContent } from '@/components/pm/CodingSession/MarkdownContent';
import { formatCodingSessionRelative } from '@/components/pm/CodingSession/codingSessionUtils';
import { useAuthStore } from '@/stores/authStore';

interface DockUserMessageProps {
  content: string;
  timestamp?: string;
  pending?: boolean;
}

/** Right-aligned user message used by both persisted turns and optimistic echoes. */
export function DockUserMessage({ content, timestamp, pending = false }: DockUserMessageProps) {
  const user = useAuthStore((state) => state.user);
  const userName = user?.full_name || user?.email || 'You';

  return (
    <div className="flex flex-col items-end gap-2">
      <div data-dock-user-message-header className="flex items-center justify-end gap-2 px-1 text-[11px] text-muted-foreground">
        {timestamp ? (
          <time dateTime={timestamp} title={new Date(timestamp).toLocaleString()}>
            {formatCodingSessionRelative(timestamp)}
          </time>
        ) : pending ? <span>Sending…</span> : null}
        {pending && timestamp ? <span className="sr-only">Sending…</span> : null}
        <span className="font-medium">{userName}</span>
        <UserAvatar
          name={userName}
          avatarUrl={user?.avatar_url}
          avatarStyle={user?.avatar_style}
          avatarSeed={user?.avatar_seed}
          avatarBackgroundMode={user?.avatar_background_mode}
          avatarBackgroundColor={user?.avatar_background_color}
          className="h-6 w-6"
          fallbackClassName="text-[10px]"
        />
      </div>
      <div data-dock-user-bubble className="max-w-[85%] rounded-2xl rounded-br-sm bg-blue-50 px-3.5 py-2.5 text-sm leading-relaxed text-foreground/85 shadow-sm dark:bg-blue-950/40 dark:text-foreground">
        <MarkdownContent content={content} className="text-inherit" />
      </div>
    </div>
  );
}
