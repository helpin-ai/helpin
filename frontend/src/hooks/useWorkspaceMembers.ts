import { useEffect, useState } from 'react';
import { workspacesService } from '@/lib/services/workspacesService';
import type { MemberWithUser } from '@/lib/types';

let cachedWorkspaceId: string | null = null;
let cachedMembers: MemberWithUser[] = [];
let fetchPromise: Promise<void> | null = null;

export function useWorkspaceMembers(workspaceId: string | undefined) {
  const [members, setMembers] = useState<MemberWithUser[]>(
    cachedWorkspaceId === workspaceId ? cachedMembers : [],
  );
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (!workspaceId) return;

    if (cachedWorkspaceId === workspaceId && cachedMembers.length > 0) {
      setMembers(cachedMembers);
      return;
    }

    const load = async () => {
      if (!fetchPromise || cachedWorkspaceId !== workspaceId) {
        setLoading(true);
        fetchPromise = (async () => {
          const { data } = await workspacesService.listMembers(workspaceId);
          if (data) {
            cachedWorkspaceId = workspaceId;
            cachedMembers = data;
          }
        })();
      }

      await fetchPromise;
      fetchPromise = null;
      setMembers(cachedMembers);
      setLoading(false);
    };

    load();
  }, [workspaceId]);

  return { members, loading };
}
