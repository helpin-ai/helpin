import { useState } from 'react';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';
import { dockChatService } from '@/lib/services/dockChatService';
import type { DockChat } from '@/lib/dockTypes';

interface ChatListViewProps {
  workspaceId: string;
  chats: DockChat[];
  activeChatId: string | null;
  onSelect: (chatId: string) => void;
  onChanged: () => void;
}

/** The user's dock chats: switch, rename, archive. */
export function ChatListView({ workspaceId, chats, activeChatId, onSelect, onChanged }: ChatListViewProps) {
  const [renamingId, setRenamingId] = useState<string | null>(null);
  const [renameValue, setRenameValue] = useState('');

  const rename = async (chatId: string) => {
    const title = renameValue.trim();
    setRenamingId(null);
    if (!title) return;
    const res = await dockChatService.updateChat(workspaceId, chatId, { title });
    if (res.error) toast.error(res.error);
    onChanged();
  };

  const archive = async (chatId: string) => {
    const res = await dockChatService.updateChat(workspaceId, chatId, { archived: true });
    if (res.error) toast.error(res.error);
    onChanged();
  };

  if (chats.length === 0) {
    return (
      <p className="px-4 py-6 text-center text-sm text-muted-foreground">
        No chats yet — start one with “New chat”.
      </p>
    );
  }

  return (
    <ul className="max-h-[60vh] overflow-y-auto py-1">
      {chats.map((chat) => (
        <li key={chat.id} className="group flex items-center gap-2 px-2">
          {renamingId === chat.id ? (
            <input
              autoFocus
              value={renameValue}
              onChange={(e) => setRenameValue(e.target.value)}
              onBlur={() => void rename(chat.id)}
              onKeyDown={(e) => {
                if (e.key === 'Enter') void rename(chat.id);
                if (e.key === 'Escape') setRenamingId(null);
              }}
              className="my-1 w-full rounded-md border border-border bg-background px-2 py-1.5 text-sm"
            />
          ) : (
            <button
              type="button"
              onClick={() => onSelect(chat.id)}
              className={cn(
                'my-0.5 flex-1 truncate rounded-md px-2 py-1.5 text-left text-sm hover:bg-muted',
                chat.id === activeChatId && 'bg-muted font-medium',
              )}
            >
              {chat.title.trim() || 'Untitled chat'}
            </button>
          )}
          <div className="hidden shrink-0 items-center gap-1 group-hover:flex">
            <Button
              size="sm"
              variant="ghost"
              className="h-6 px-1.5 text-xs"
              onClick={() => {
                setRenamingId(chat.id);
                setRenameValue(chat.title);
              }}
            >
              Rename
            </Button>
            <Button
              size="sm"
              variant="ghost"
              className="h-6 px-1.5 text-xs text-muted-foreground"
              onClick={() => void archive(chat.id)}
            >
              Archive
            </Button>
          </div>
        </li>
      ))}
    </ul>
  );
}
