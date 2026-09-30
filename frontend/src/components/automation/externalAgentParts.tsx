import type { ReactNode } from 'react';
import { QuietMetaLine, quietUnderlineControlClassName } from '@/components/design-system/quiet';
import { QuietDropdown } from '@/components/design-system/quiet-dropdown';
import { Label } from '@/components/ui/label';
import { useWorkspaceTeams } from '@/hooks/useWorkspaceTeams';
import type { AgentCardSummary } from '@/lib/externalAgentTypes';
import {
  externalAgentCapabilityLabel,
  externalAgentHost,
  externalAgentProtocolLabel,
  externalAgentTeamSummary,
} from '@/lib/externalAgents';
import { ArrowDown01Icon } from '@/lib/icons';
import { cn } from '@/lib/utils';

const ALL_TEAMS = '__all_teams__';
const VISIBLE_SKILLS = 6;

export function ExternalAgentField({ label, id, children }: { label: string; id?: string; children: ReactNode }) {
  return (
    <div className="min-w-0 space-y-1">
      <Label htmlFor={id} className="text-sm font-medium text-quiet-text-secondary">{label}</Label>
      {children}
    </div>
  );
}

export function ExternalAgentDataNotice({ className }: { className?: string }) {
  return (
    <p className={cn('text-[12.5px] leading-5 text-quiet-text-tertiary [text-wrap:pretty]', className)} data-external-agent-data-notice>
      Tasks sent to this agent leave Helpin: it receives the task title, description, acceptance criteria, and a link back to the task.
      Files it uploads are attached to the task.
    </p>
  );
}

/** Read-only summary of the agent's A2A Agent Card, shown before adding it. */
export function ExternalAgentCardSummaryView({ card }: { card: AgentCardSummary }) {
  const skills = card.skills ?? [];
  const hidden = Math.max(0, skills.length - VISIBLE_SKILLS);
  return (
    <div className="space-y-3" data-external-agent-card>
      <div className="min-w-0">
        <p className="text-[14px] font-semibold tracking-[-0.008em] text-quiet-text-primary">{card.name || 'Unnamed agent'}</p>
        {card.description ? (
          <p className="mt-1 text-sm leading-[1.6] text-quiet-text-secondary [text-wrap:pretty]">{card.description}</p>
        ) : null}
        <QuietMetaLine
          className="mt-2"
          items={[
            card.provider_name || null,
            card.version ? `v${card.version}` : null,
            externalAgentProtocolLabel(card),
            externalAgentCapabilityLabel(card),
          ]}
        />
        {card.interface_url ? (
          <p className="mt-1 truncate text-[11.5px] text-quiet-muted" title={card.interface_url}>{externalAgentHost(card.interface_url)}</p>
        ) : null}
      </div>
      <div>
        <p className="text-[12px] font-semibold uppercase tracking-[0.06em] text-quiet-muted">
          Skills <span className="font-normal tabular-nums">{skills.length}</span>
        </p>
        {skills.length === 0 ? (
          <p className="mt-1 text-[12.5px] text-quiet-text-tertiary">The Agent Card lists no skills.</p>
        ) : (
          <ul className="mt-1">
            {skills.slice(0, VISIBLE_SKILLS).map((skill) => (
              <li key={skill.id || skill.name} className="min-w-0 border-b border-quiet-divider-light py-1.5 last:border-b-0">
                <p className="truncate text-sm font-medium text-quiet-text-primary">{skill.name || skill.id}</p>
                {skill.description ? <p className="line-clamp-2 text-[12.5px] leading-5 text-quiet-text-tertiary">{skill.description}</p> : null}
              </li>
            ))}
            {hidden > 0 ? <li className="pt-1.5 text-[12.5px] text-quiet-text-tertiary">{hidden} more</li> : null}
          </ul>
        )}
      </div>
    </div>
  );
}

/** Optional restriction on which teams may assign tasks to the external agent. Empty means all teams. */
export function ExternalAgentTeamPicker({
  workspaceId,
  value,
  onChange,
  disabled,
  id = 'external-agent-teams',
}: {
  workspaceId: string;
  value: string[];
  onChange: (teamIds: string[]) => void;
  disabled?: boolean;
  id?: string;
}) {
  const { teams, loading, findTeamName } = useWorkspaceTeams(workspaceId);
  const summary = loading && value.length > 0 ? 'Loading teams…' : externalAgentTeamSummary(value, (teamId) => findTeamName(teamId));
  const toggle = (teamId: string) => {
    if (teamId === ALL_TEAMS) {
      onChange([]);
      return;
    }
    onChange(value.includes(teamId) ? value.filter((idValue) => idValue !== teamId) : [...value, teamId]);
  };
  return (
    <ExternalAgentField label="Teams that can assign tasks" id={id}>
      <QuietDropdown
        label="Teams"
        multiple
        disabled={disabled}
        loading={loading}
        selected={value.length === 0 ? [ALL_TEAMS] : value}
        onSelect={toggle}
        options={[
          { value: ALL_TEAMS, label: 'All teams' },
          ...teams.map((team) => ({ value: team.id, label: team.name })),
        ]}
        trigger={(
          <button
            id={id}
            type="button"
            disabled={disabled}
            className={cn(quietUnderlineControlClassName, 'flex w-full items-center gap-2 text-left disabled:opacity-60')}
          >
            <span className="min-w-0 flex-1 truncate">{summary}</span>
            <ArrowDown01Icon className="h-3.5 w-3.5 shrink-0 text-quiet-muted" />
          </button>
        )}
      />
    </ExternalAgentField>
  );
}
