import { BotIcon } from '@/lib/icons';

import type { Agent, AgentPresetKey } from '@/lib/pmTypes';
import { cn } from '@/lib/utils';

export type AgentPersonaKey =
  | 'atlas'
  | 'scribe'
  | 'forge'
  | 'echo'
  | 'lens'
  | 'beacon'
  | 'quill'
  | 'mira'
  | 'generic';

interface AgentPersonaMeta {
  key: AgentPersonaKey;
  label: string;
  role: string;
}

const PRESET_PERSONA_MAP: Partial<Record<AgentPresetKey, AgentPersonaKey>> = {
  epic_planner: 'atlas',
  task_planner: 'scribe',
  story_planner: 'scribe',
  code_builder: 'forge',
  support_agent: 'echo',
  review_agent: 'lens',
  crm_operator: 'beacon',
  documentation_agent: 'quill',
  marketer: 'mira',
};

const PERSONA_META: Record<AgentPersonaKey, AgentPersonaMeta> = {
  atlas: { key: 'atlas', label: 'Atlas', role: 'Epic planner' },
  scribe: { key: 'scribe', label: 'Scribe', role: 'Task planner' },
  forge: { key: 'forge', label: 'Forge', role: 'Coding agent' },
  echo: { key: 'echo', label: 'Echo', role: 'Help chat agent' },
  lens: { key: 'lens', label: 'Lens', role: 'QA reviewer' },
  beacon: { key: 'beacon', label: 'Beacon', role: 'CRM operator' },
  quill: { key: 'quill', label: 'Quill', role: 'Documentation Agent' },
  mira: { key: 'mira', label: 'Mira', role: 'Marketer' },
  generic: { key: 'generic', label: 'Agent', role: 'Automation agent' },
};

function normalizeName(name?: string | null) {
  return name?.trim().toLowerCase() ?? '';
}

function personaFromName(name?: string | null): AgentPersonaKey | null {
  switch (normalizeName(name)) {
    case 'atlas':
      return 'atlas';
    case 'scribe':
      return 'scribe';
    case 'forge':
      return 'forge';
    case 'echo':
      return 'echo';
    case 'lens':
      return 'lens';
    case 'beacon':
      return 'beacon';
    case 'quill':
      return 'quill';
    case 'mira':
      return 'mira';
    default:
      return null;
  }
}

export function resolveAgentPersonaKey(input?: {
  agent?: Pick<Agent, 'name' | 'preset_key'> | null;
  name?: string | null;
  presetKey?: AgentPresetKey;
}): AgentPersonaKey {
  const fromName = personaFromName(input?.agent?.name ?? input?.name);
  if (fromName) return fromName;

  const resolvedPresetKey = input?.agent?.preset_key ?? input?.presetKey;
  const fromPreset = resolvedPresetKey ? PRESET_PERSONA_MAP[resolvedPresetKey] : null;
  if (fromPreset) return fromPreset;

  return 'generic';
}

export function getAgentPersonaMeta(input?: {
  agent?: Pick<Agent, 'name' | 'preset_key'> | null;
  name?: string | null;
  presetKey?: AgentPresetKey;
}): AgentPersonaMeta {
  return PERSONA_META[resolveAgentPersonaKey(input)];
}

interface AgentAvatarProps {
  agent?: Pick<Agent, 'name' | 'preset_key'> | null;
  name?: string | null;
  presetKey?: AgentPresetKey;
  className?: string;
  svgClassName?: string;
  decorative?: boolean;
  genericBare?: boolean;
}

export function AgentAvatar({
  agent,
  name,
  presetKey,
  className,
  svgClassName,
  decorative = true,
  genericBare = false,
}: AgentAvatarProps) {
  const persona = resolveAgentPersonaKey({ agent, name, presetKey });
  const meta = PERSONA_META[persona];

  const accessibilityProps = decorative
    ? { 'aria-hidden': true as const }
    : { role: 'img' as const, 'aria-label': meta.label };

  return (
    <span
      className={cn(
        'inline-flex h-8 w-8 shrink-0 items-center justify-center overflow-hidden rounded-2xl border border-border/70 bg-background/90 shadow-sm',
        genericBare && persona === 'generic' && 'rounded-none border-0 bg-transparent shadow-none',
        className,
      )}
      {...accessibilityProps}
    >
      <PersonaSvg persona={persona} className={cn('h-full w-full', svgClassName)} genericBare={genericBare} />
    </span>
  );
}

function PersonaSvg({ persona, className, genericBare = false }: { persona: AgentPersonaKey; className?: string; genericBare?: boolean }) {
  switch (persona) {
    case 'atlas':
      return (
        <svg className={className} viewBox="0 0 80 80" fill="none">
          <rect x="10" y="14" width="60" height="52" rx="14" fill="#D85A30" opacity=".12" />
          <rect x="14" y="18" width="52" height="44" rx="12" fill="#D85A30" />
          <rect x="22" y="28" width="14" height="10" rx="4" fill="#FAECE7" />
          <rect x="44" y="28" width="14" height="10" rx="4" fill="#FAECE7" />
          <circle cx="28" cy="33" r="3" fill="#4A1B0C" />
          <circle cx="50" cy="33" r="3" fill="#4A1B0C" />
          <rect x="30" y="44" width="20" height="4" rx="2" fill="#FAECE7" />
          <rect x="24" y="8" width="8" height="14" rx="4" fill="#D85A30" />
          <rect x="48" y="8" width="8" height="14" rx="4" fill="#D85A30" />
          <line x1="28" y1="8" x2="28" y2="2" stroke="#993C1D" strokeWidth="2" strokeLinecap="round" />
          <line x1="52" y1="8" x2="52" y2="2" stroke="#993C1D" strokeWidth="2" strokeLinecap="round" />
          <circle cx="28" cy="2" r="2" fill="#EF9F27" />
          <circle cx="52" cy="2" r="2" fill="#EF9F27" />
        </svg>
      );
    case 'scribe':
      return (
        <svg className={className} viewBox="0 0 80 80" fill="none">
          <rect x="10" y="16" width="60" height="48" rx="12" fill="#1D9E75" opacity=".12" />
          <rect x="14" y="20" width="52" height="40" rx="10" fill="#1D9E75" />
          <rect x="22" y="30" width="12" height="8" rx="3" fill="#E1F5EE" />
          <rect x="46" y="30" width="12" height="8" rx="3" fill="#E1F5EE" />
          <circle cx="28" cy="34" r="2.5" fill="#04342C" />
          <circle cx="52" cy="34" r="2.5" fill="#04342C" />
          <path d="M32 46Q40 52 48 46" stroke="#E1F5EE" strokeWidth="2.5" strokeLinecap="round" />
          <rect x="30" y="6" width="20" height="16" rx="6" fill="#1D9E75" />
          <rect x="34" y="2" width="12" height="8" rx="4" fill="#0F6E56" />
          <rect x="36" y="10" width="8" height="4" rx="2" fill="#5DCAA5" />
        </svg>
      );
    case 'forge':
      return (
        <svg className={className} viewBox="0 0 80 80" fill="none">
          <rect x="10" y="16" width="60" height="50" rx="10" fill="#534AB7" opacity=".12" />
          <rect x="14" y="20" width="52" height="42" rx="8" fill="#534AB7" />
          <rect x="20" y="28" width="16" height="12" rx="4" fill="#EEEDFE" />
          <rect x="44" y="28" width="16" height="12" rx="4" fill="#EEEDFE" />
          <rect x="24" y="32" width="4" height="4" rx="1" fill="#26215C" />
          <rect x="28" y="32" width="4" height="4" rx="1" fill="#7F77DD" />
          <rect x="48" y="32" width="4" height="4" rx="1" fill="#26215C" />
          <rect x="52" y="32" width="4" height="4" rx="1" fill="#7F77DD" />
          <rect x="28" y="48" width="24" height="6" rx="3" fill="#AFA9EC" />
          <rect x="30" y="49.5" width="4" height="3" rx="1" fill="#26215C" />
          <rect x="36" y="49.5" width="4" height="3" rx="1" fill="#26215C" />
          <rect x="42" y="49.5" width="4" height="3" rx="1" fill="#26215C" />
          <polygon points="40,4 48,16 32,16" fill="#534AB7" />
          <rect x="37" y="10" width="6" height="3" rx="1" fill="#EEEDFE" />
        </svg>
      );
    case 'echo':
      return (
        <svg className={className} viewBox="0 0 80 80" fill="none">
          <circle cx="40" cy="42" r="28" fill="#D4537E" opacity=".12" />
          <circle cx="40" cy="42" r="24" fill="#D4537E" />
          <circle cx="32" cy="36" r="6" fill="#FBEAF0" />
          <circle cx="48" cy="36" r="6" fill="#FBEAF0" />
          <circle cx="32" cy="36" r="3" fill="#4B1528" />
          <circle cx="48" cy="36" r="3" fill="#4B1528" />
          <circle cx="33.5" cy="35" r="1" fill="#FBEAF0" />
          <circle cx="49.5" cy="35" r="1" fill="#FBEAF0" />
          <path d="M34 50Q40 56 46 50" stroke="#FBEAF0" strokeWidth="2.5" strokeLinecap="round" />
          <ellipse cx="40" cy="10" rx="10" ry="6" fill="#D4537E" />
          <circle cx="40" cy="6" r="3" fill="#ED93B1" />
        </svg>
      );
    case 'lens':
      return (
        <svg className={className} viewBox="0 0 80 80" fill="none">
          <rect x="10" y="16" width="60" height="50" rx="14" fill="#BA7517" opacity=".12" />
          <rect x="14" y="20" width="52" height="42" rx="12" fill="#BA7517" />
          <circle cx="30" cy="36" r="9" fill="#FAEEDA" />
          <circle cx="50" cy="36" r="9" fill="#FAEEDA" />
          <circle cx="30" cy="36" r="5" fill="#412402" />
          <circle cx="50" cy="36" r="5" fill="#412402" />
          <circle cx="32" cy="34" r="2" fill="#FAEEDA" />
          <circle cx="52" cy="34" r="2" fill="#FAEEDA" />
          <rect x="34" y="50" width="12" height="4" rx="2" fill="#FAEEDA" />
          <path d="M10 30Q6 20 14 14L22 20" stroke="#BA7517" strokeWidth="3" strokeLinecap="round" />
          <path d="M70 30Q74 20 66 14L58 20" stroke="#BA7517" strokeWidth="3" strokeLinecap="round" />
        </svg>
      );
    case 'beacon':
      return (
        <svg className={className} viewBox="0 0 80 80" fill="none">
          <circle cx="40" cy="41" r="30" fill="#0F766E" opacity=".12" />
          <rect x="15" y="20" width="50" height="44" rx="18" fill="#0F766E" />
          <path d="M40 6L52 22H28L40 6Z" fill="#14B8A6" />
          <path d="M40 11L46 19H34L40 11Z" fill="#D9FBF5" opacity=".95" />
          <path d="M25 13C20 16 16 21 14 27" stroke="#5EEAD4" strokeWidth="2.5" strokeLinecap="round" />
          <path d="M55 13C60 16 64 21 66 27" stroke="#5EEAD4" strokeWidth="2.5" strokeLinecap="round" />
          <circle cx="40" cy="8" r="2.5" fill="#99F6E4" />
          <circle cx="29" cy="38" r="7" fill="#D9FBF5" />
          <circle cx="51" cy="38" r="7" fill="#D9FBF5" />
          <circle cx="29" cy="38" r="3" fill="#083C39" />
          <circle cx="51" cy="38" r="3" fill="#083C39" />
          <circle cx="31" cy="36" r="1.3" fill="#D9FBF5" />
          <circle cx="53" cy="36" r="1.3" fill="#D9FBF5" />
          <path d="M33 51Q40 57 47 51" stroke="#D9FBF5" strokeWidth="2.6" strokeLinecap="round" />
          <circle cx="58" cy="25" r="3" fill="#99F6E4" />
          <circle cx="62" cy="31" r="2" fill="#5EEAD4" />
          <path d="M28 62H52" stroke="#5EEAD4" strokeWidth="2" strokeLinecap="round" opacity=".75" />
        </svg>
      );
    case 'quill':
      return (
        <svg className={className} viewBox="0 0 80 80" fill="none">
          <circle cx="40" cy="42" r="29" fill="#185FA5" opacity=".12" />
          <rect x="15" y="21" width="50" height="43" rx="16" fill="#185FA5" />
          <rect x="25" y="7" width="30" height="20" rx="7" fill="#185FA5" />
          <rect x="30" y="11" width="18" height="3" rx="1.5" fill="#A7D4FF" />
          <rect x="30" y="16" width="13" height="3" rx="1.5" fill="#A7D4FF" />
          <path d="M56 7C63 8 66 13 64 19C62 25 56 29 48 31C51 23 52 15 56 7Z" fill="#E6F1FB" />
          <path d="M48 31L58 16" stroke="#185FA5" strokeWidth="2" strokeLinecap="round" />
          <circle cx="30" cy="38" r="7" fill="#E6F1FB" />
          <circle cx="50" cy="38" r="7" fill="#E6F1FB" />
          <circle cx="30" cy="38" r="3" fill="#042C53" />
          <circle cx="50" cy="38" r="3" fill="#042C53" />
          <circle cx="32" cy="36" r="1.3" fill="#E6F1FB" />
          <circle cx="52" cy="36" r="1.3" fill="#E6F1FB" />
          <path d="M32 51Q40 57 48 51" stroke="#E6F1FB" strokeWidth="2.6" strokeLinecap="round" />
          <rect x="26" y="58" width="28" height="3" rx="1.5" fill="#85B7EB" opacity=".75" />
        </svg>
      );
    case 'mira':
      return (
        <svg className={className} viewBox="0 0 80 80" fill="none">
          <circle cx="40" cy="42" r="29" fill="#E2554F" opacity=".12" />
          <rect x="14" y="20" width="52" height="43" rx="17" fill="#E2554F" />
          <path d="M24 20C29 12 38 8 48 10C54 11 59 14 62 19C54 18 47 21 42 26C36 20 30 18 24 20Z" fill="#FFB35C" />
          <path d="M27 21C32 16 39 14 47 16C51 17 55 19 58 22C51 22 45 25 41 30C36 24 31 22 27 21Z" fill="#FFF1D5" opacity=".95" />
          <circle cx="30" cy="38" r="7" fill="#FFF4EA" />
          <circle cx="50" cy="38" r="7" fill="#FFF4EA" />
          <circle cx="30" cy="38" r="3" fill="#4B1714" />
          <circle cx="50" cy="38" r="3" fill="#4B1714" />
          <circle cx="32" cy="36" r="1.3" fill="#FFF4EA" />
          <circle cx="52" cy="36" r="1.3" fill="#FFF4EA" />
          <path d="M33 51Q40 56 47 51" stroke="#FFF4EA" strokeWidth="2.6" strokeLinecap="round" />
          <path d="M19 32L10 27" stroke="#FFB35C" strokeWidth="3" strokeLinecap="round" />
          <path d="M61 32L70 27" stroke="#FFB35C" strokeWidth="3" strokeLinecap="round" />
          <circle cx="10" cy="27" r="3" fill="#FFD48C" />
          <circle cx="70" cy="27" r="3" fill="#FFD48C" />
          <path d="M24 62C31 66 49 66 56 62" stroke="#FFB35C" strokeWidth="2.2" strokeLinecap="round" opacity=".8" />
        </svg>
      );
    default:
      if (genericBare) {
        return <BotIcon className={cn(className, '!h-[72%] !w-[72%] text-slate-600 dark:text-slate-300')} />;
      }
      return (
        <div className="flex h-full w-full items-center justify-center bg-slate-100 text-slate-600 dark:bg-slate-900 dark:text-slate-300">
          <BotIcon className={cn(className, '!h-1/2 !w-1/2')} />
        </div>
      );
  }
}
