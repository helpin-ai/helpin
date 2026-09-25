import { useEffect, useState } from 'react';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
import { dockChatService } from '@/lib/services/dockChatService';
import { workspacesService } from '@/lib/services/workspacesService';
import { withDockReadDeadline } from './dock/dockReadDeadline';

type Access = 'checking' | 'allowed' | 'restricted' | 'error';

/** Mount with a key for the viewer and destination so access cannot leak across either. */
export function PublicShareOpenButton({ openPath, signedIn }: { openPath: string; signedIn: boolean }) {
  const [access, setAccess] = useState<Access>('checking');
  useEffect(() => {
    if (!signedIn) return;
    let active = true;
    let controller: AbortController | undefined;
    const check = async () => {
      controller?.abort();
      const request = new AbortController();
      controller = request;
      setAccess('checking');
      try {
        const result = await withDockReadDeadline(async signal => {
          const url = new URL(openPath, window.location.origin);
          const parts = url.pathname.split('/').filter(Boolean);
          if (url.origin !== window.location.origin || parts[0] !== 'w' || !parts[1]) throw new Error('Invalid destination');
          const chatId = url.searchParams.get('ask_chat');
          const runId = parts[2] === 'pm' && parts[3] === 'coding-sessions' ? parts[4] : null;
          if (!chatId && !runId) throw new Error('Missing resource');
          const workspace = await workspacesService.getBySlug(parts[1]);
          if (signal.aborted) throw signal.reason;
          if (!workspace.data || workspace.error) return workspace;
          return chatId
            ? dockChatService.getChat(workspace.data.id, chatId, signal)
            : dockChatService.getRunSnapshot(workspace.data.id, runId!, signal);
        }, request);
        if (!active || request.signal.aborted) return;
        setAccess(result.data && !result.error ? 'allowed' : [403, 404].includes(result.status ?? 0) ? 'restricted' : 'error');
      } catch {
        if (active && controller === request) setAccess('error');
      }
    };
    void check();
    window.addEventListener('focus', check);
    return () => { active = false; controller?.abort(); window.removeEventListener('focus', check); };
  }, [openPath, signedIn]);

  const className = 'shrink-0 whitespace-nowrap rounded-full bg-[#1c1b19] px-3 py-2 text-sm font-medium text-white sm:px-4 dark:bg-[#eeeae1] dark:text-[#1c1b19]';
  if (signedIn && access === 'allowed') {
    return <a href={openPath} target="_blank" rel="noopener noreferrer" className={className}>Open in Helpin</a>;
  }
  const reason = !signedIn ? 'Sign in to check whether you can open this in Helpin.'
    : access === 'checking' ? 'Checking your access…'
    : access === 'restricted' ? 'This is not shared with you inside Helpin. Ask the owner to share it with you in the workspace. You can still read it here.'
    : 'Could not check access. Refresh this page to try again.';
  return <TooltipProvider><Tooltip><TooltipTrigger asChild>
    <button type="button" aria-disabled="true" className={`${className} cursor-not-allowed opacity-50`}>Open in Helpin</button>
  </TooltipTrigger><TooltipContent side="bottom">{reason}</TooltipContent></Tooltip></TooltipProvider>;
}
