import { useQuery } from "@tanstack/react-query";
import { aiProfileService } from "@/lib/services/aiProfileService";

export function AIProfileLabel({ workspaceId, profileId }: { workspaceId?: string; profileId?: string | null }) {
  if (!profileId) return <>Workspace default</>;
  return <NamedProfile workspaceId={workspaceId} profileId={profileId} />;
}

function NamedProfile({ workspaceId, profileId }: { workspaceId?: string; profileId: string }) {
  const query = useQuery({
    queryKey: ["ai-profiles", workspaceId],
    enabled: !!workspaceId && !!profileId,
    queryFn: async () => {
      const response = await aiProfileService.list(workspaceId!);
      if (response.error || !response.data) throw new Error(response.error || "Unable to load AI profiles");
      return response.data;
    },
    staleTime: 30_000,
  });
  if (query.isPending) return <>Loading profile…</>;
  if (query.isError) return <>Profile unavailable</>;
  return <>{query.data.find((profile) => profile.id === profileId)?.name ?? "Saved profile unavailable"}</>;
}
