import { api } from "@/lib/api";
import type {
  SupportConversation,
  SupportConversationMerge,
  SupportMergePreview,
  SupportMergeResult,
  SupportMessagePage,
} from "@/lib/pmTypes";

const path = (workspaceId: string, suffix: string) =>
  `/support/inbox/${suffix}?workspace_id=${encodeURIComponent(workspaceId)}`;
export const supportMergeService = {
  candidates: (
    ws: string,
    id: string,
    search: string,
    allCustomers: boolean,
    page: number,
  ) =>
    api.get<{
      data: SupportConversation[];
      has_more: boolean;
      cross_customer_allowed: boolean;
    }>(
      `${path(ws, `conversations/${id}/merge-candidates`)}&search=${encodeURIComponent(search)}&all_customers=${allCustomers}&page=${page}`,
    ),
  preview: (ws: string, target: string, sources: string[]) =>
    api.post<SupportMergePreview>(
      path(ws, `conversations/${target}/merge-preview`),
      { source_ids: sources },
    ),
  merge: (
    ws: string,
    target: string,
    sources: string[],
    token: string,
    operation: string,
  ) =>
    api.post<SupportMergeResult>(path(ws, `conversations/${target}/merge`), {
      source_ids: sources,
      preview_token: token,
      operation_id: operation,
    }),
  list: (ws: string, id: string) =>
    api.get<SupportConversationMerge[]>(path(ws, `conversations/${id}/merges`)),
  history: (ws: string, id: string, merge: string, cursor?: string) =>
    api.get<SupportMessagePage>(
      `${path(ws, `conversations/${id}/history`)}&merge_id=${encodeURIComponent(merge)}${cursor ? `&cursor=${encodeURIComponent(cursor)}` : ""}`,
    ),
  unmergePreview: (ws: string, id: string, restoreMailbox?: string) =>
    api.post<SupportMergePreview>(
      path(ws, `conversation-merges/${id}/unmerge-preview`),
      { restore_mailbox_id: restoreMailbox },
    ),
  unmerge: (
    ws: string,
    id: string,
    token: string,
    operation: string,
    restoreMailbox?: string,
  ) =>
    api.post<SupportMergeResult>(
      path(ws, `conversation-merges/${id}/unmerge`),
      {
        preview_token: token,
        operation_id: operation,
        restore_mailbox_id: restoreMailbox,
      },
    ),
};
