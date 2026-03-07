import { useEffect, useState } from 'react';
import { workspacesService } from '@/lib/services/workspacesService';
import type { AssignableMember } from '@/lib/types';

let cachedWorkspaceId: string | null = null;
let cachedMembers: AssignableMember[] = [];
let fetchPromise: Promise<void> | null = null;

export function useAssignableWorkspaceMembers(workspaceId: string | undefined) {
  const [fetchedMembers, setFetchedMembers] = useState<AssignableMember[]>([]);
  const [loading, setLoading] = useState(false);
  const members = cachedWorkspaceId === workspaceId && cachedMembers.length > 0
    ? cachedMembers
    : fetchedMembers;

  useEffect(() => {
    if (!workspaceId) return;

    if (cachedWorkspaceId === workspaceId && cachedMembers.length > 0) {
      let cancelled = false;
      queueMicrotask(() => {
        if (!cancelled) {
          setLoading(false);
        }
      });
      return () => {
        cancelled = true;
      };
    }

    let cancelled = false;

    const load = async () => {
      if (!fetchPromise || cachedWorkspaceId !== workspaceId) {
        setLoading(true);
        fetchPromise = (async () => {
          const { data } = await workspacesService.listAssignableMembers(workspaceId);
          if (data) {
            cachedWorkspaceId = workspaceId;
            cachedMembers = data;
          }
        })();
      }

      await fetchPromise;
      fetchPromise = null;
      if (cancelled) return;
      setFetchedMembers(cachedMembers);
      setLoading(false);
    };

    load();
    return () => {
      cancelled = true;
    };
  }, [workspaceId]);

  return { members, loading };
}
