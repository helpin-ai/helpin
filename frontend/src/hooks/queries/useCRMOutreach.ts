import {
  useQuery,
  useQueryClient,
  useInfiniteQuery,
} from "@tanstack/react-query";
import { crmOutreachService } from "@/lib/services/crmOutreachService";
import { unwrap } from "@/lib/queryUtils";
export function useEmailTemplates(ws: string) {
  return useQuery({
    queryKey: ["crm", ws, "emailTemplates"],
    queryFn: async () => unwrap(await crmOutreachService.templates(ws)),
    enabled: !!ws,
  });
}
export function useEmailSequences(ws: string) {
  return useQuery({
    queryKey: ["crm", ws, "emailSequences"],
    queryFn: async () => unwrap(await crmOutreachService.sequences(ws)),
    enabled: !!ws,
  });
}
export function useSequenceEnrollments(
  ws: string,
  filters: {
    sequence_id?: string;
    contact_id?: string;
    deal_id?: string;
    search?: string;
    status?: string;
  } = {},
) {
  const query = useInfiniteQuery({
    queryKey: ["crm", ws, "sequenceEnrollments", filters],
    initialPageParam: 1,
    queryFn: async ({ pageParam, signal }) =>
      unwrap(
        await crmOutreachService.enrollments(
          ws,
          {
            ...filters,
            page: String(pageParam),
          },
          signal,
        ),
      ),
    getNextPageParam: (last, pages) =>
      last.length === 50 ? pages.length + 1 : undefined,
    enabled: !!ws,
    retry: false,
    refetchInterval: (query) =>
      query.state.status === "error" ? false : 15000,
  });
  return { ...query, data: query.data?.pages.flat() };
}
export function useOutreachRefresh(ws: string) {
  const client = useQueryClient();
  return () => client.invalidateQueries({ queryKey: ["crm", ws] });
}
