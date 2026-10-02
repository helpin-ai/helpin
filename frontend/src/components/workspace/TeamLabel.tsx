import { resolveTeamColor } from '@/lib/teamColor';

type TeamIdentity = { name: string; color?: string | null };

/** Compact team identity for rows and picker options. */
export function TeamColorMark({ team }: { team: TeamIdentity }) {
  return <span aria-hidden="true" data-team-color className="inline-block size-[8px] shrink-0 rounded-[2px]" style={{ backgroundColor: resolveTeamColor(team.name, team.color) }} />;
}

export function TeamLabel({ team }: { team: TeamIdentity }) {
  return <span className="inline-flex min-w-0 items-center gap-1.5"><TeamColorMark team={team} /><span className="truncate">{team.name}</span></span>;
}
