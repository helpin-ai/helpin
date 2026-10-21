import type { SupportMessagePage } from "@/lib/pmTypes";
import type { SupportMessagePages } from "@/lib/supportMessagePages";
import {
  type QueryKey,
  useInfiniteQuery,
  useMutation,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import { supportMergeService } from "@/lib/services/supportMergeService";
import { unwrapRequired } from "@/lib/queryUtils";
import { queryKeys } from "@/lib/queryKeys";

export const mergeKeys = {
  list: (ws: string, id: string) =>
    [...queryKeys.support.conversation(ws, id), "merges"] as const,
  history: (ws: string, id: string, merge: string) =>
    [
      ...queryKeys.support.conversation(ws, id),
      "shared-history",
      merge,
    ] as const,
};
export function useConversationMerges(ws: string, id: string | null) {
  return useQuery({
    queryKey: mergeKeys.list(ws, id ?? ""),
    queryFn: async () =>
      unwrapRequired(
        await supportMergeService.list(ws, id!),
        "Conversation merges",
      ),
    enabled: !!ws && !!id,
  });
}
export function useSharedConversationHistory(
  ws: string,
  id: string,
  merge: string | undefined,
) {
  return useInfiniteQuery<
    SupportMessagePage,
    Error,
    SupportMessagePages,
    QueryKey,
    string | undefined
  >({
    queryKey: mergeKeys.history(ws, id, merge ?? ""),
    enabled: !!merge,
    initialPageParam: undefined as string | undefined,
    queryFn: async ({ pageParam }) =>
      unwrapRequired(
        await supportMergeService.history(ws, id, merge!, pageParam),
        "Shared history",
      ),
    getNextPageParam: (page) => page.next_cursor ?? undefined,
  });
}
export function useMergeConversation(ws: string) {
  const client = useQueryClient();
  return useMutation({
    mutationFn: async ({
      target,
      sources,
      token,
      operation,
    }: {
      target: string;
      sources: string[];
      token: string;
      operation: string;
    }) =>
      unwrapRequired(
        await supportMergeService.merge(ws, target, sources, token, operation),
        "Merge",
      ),
    onSuccess: async () => {
      await Promise.all([
        client.invalidateQueries({ queryKey: ["support", ws] }),
        client.invalidateQueries({
          queryKey: queryKeys.support.workspaceUnread(),
        }),
      ]);
    },
  });
}
export function useUnmergeConversation(ws: string) {
  const client = useQueryClient();
  return useMutation({
    mutationFn: async ({
      id,
      token,
      operation,
      restoreMailbox,
    }: {
      id: string;
      token: string;
      operation: string;
      restoreMailbox?: string;
    }) =>
      unwrapRequired(
        await supportMergeService.unmerge(
          ws,
          id,
          token,
          operation,
          restoreMailbox,
        ),
        "Unmerge",
      ),
    onSuccess: async () => {
      await Promise.all([
        client.invalidateQueries({ queryKey: ["support", ws] }),
        client.invalidateQueries({
          queryKey: queryKeys.support.workspaceUnread(),
        }),
      ]);
    },
  });
}
