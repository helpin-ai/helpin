import { useAIProfiles } from "@/hooks/queries/useAIProfiles";

export function AIProfileLabel({ workspaceId, profileId }: { workspaceId?: string; profileId?: string | null }) {
  if (!profileId) return <>Workspace default</>;
  return <NamedProfile workspaceId={workspaceId} profileId={profileId} />;
}

function NamedProfile({ workspaceId, profileId }: { workspaceId?: string; profileId: string }) {
  const query = useAIProfiles(workspaceId ?? "", { enabled: !!profileId, staleTime: 30_000 });
  if (query.isPending) return <>Loading profile…</>;
  if (query.isError) return <>Profile unavailable</>;
  return <>{query.data.find((profile) => profile.id === profileId)?.name ?? "Saved profile unavailable"}</>;
}
