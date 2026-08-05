import { Copy01Icon, Tick01Icon } from '@/lib/icons';
import { UserAvatar } from '@/components/pm/UserAvatar';
import { MarkdownContent } from '@/components/pm/CodingSession/MarkdownContent';
import { formatCodingSessionRelative } from '@/components/pm/CodingSession/codingSessionUtils';
import { useCopyToClipboard } from '@/hooks/useCopyToClipboard';
import { useAuthStore } from '@/stores/authStore';

interface DockUserMessageProps {
  content: string;
  timestamp?: string;
  pending?: boolean;
}

/** Right-aligned user message used by both persisted turns and optimistic echoes. */
export function DockUserMessage({ content, timestamp, pending = false }: DockUserMessageProps) {
  const user = useAuthStore((state) => state.user);
  const { copied, copy, error } = useCopyToClipboard();
  const userName = user?.full_name || user?.email || 'You';

  return (
    <div className="group flex items-end justify-end gap-2">
      <div data-dock-user-bubble className="flex min-w-0 max-w-[85%] flex-col items-end">
        <div className="max-w-full rounded-2xl rounded-br-sm bg-blue-50 px-3.5 py-2.5 text-sm leading-relaxed text-foreground/85 shadow-sm dark:bg-blue-950/40 dark:text-foreground">
          <MarkdownContent content={content} className="text-inherit" />
        </div>
        <div
          data-dock-message-actions
          className="flex min-h-5 items-center justify-end gap-1.5 pr-0.5 pt-1 text-[10px] text-muted-foreground opacity-0 transition-opacity group-hover:opacity-100 group-focus-within:opacity-100"
        >
          {timestamp ? (
            <time dateTime={timestamp} title={new Date(timestamp).toLocaleString()}>
              {formatCodingSessionRelative(timestamp)}
            </time>
          ) : pending ? (
            <span>Sending…</span>
          ) : null}
          <button
            type="button"
            aria-label={copied ? 'Message copied' : 'Copy message'}
            title={copied ? 'Copied' : 'Copy message'}
            onClick={() => copy(content)}
            className="inline-flex h-5 w-5 items-center justify-center rounded text-muted-foreground transition-colors hover:bg-muted hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          >
            {copied ? <Tick01Icon className="h-3 w-3" /> : <Copy01Icon className="h-3 w-3" />}
          </button>
          <span role="status" aria-live="polite" className="sr-only">
            {copied ? 'Copied' : error ? 'Copy failed' : ''}
          </span>
        </div>
      </div>
      <span data-dock-user-avatar aria-label={userName} className="mb-5">
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
      </span>
    </div>
  );
}
