import { useAssignableMembers } from '@/hooks/queries/useWorkspaces';
import type { AssignableMember } from '@/lib/types';

const EMPTY: AssignableMember[] = [];

/**
 * Assignable members for a workspace, backed by TanStack Query.
 *
 * Kept as a thin wrapper so the ~20 existing call sites keep their
 * `{ members, loading }` shape. Prefer `useAssignableMembers` from
 * `@/hooks/queries` in new code.
 */
export function useAssignableWorkspaceMembers(workspaceId: string | undefined) {
  const { data, isLoading } = useAssignableMembers(workspaceId ?? '');
  return { members: data ?? EMPTY, loading: isLoading };
}
