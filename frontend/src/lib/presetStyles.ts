import type { AgentPresetKey } from '@/lib/pmTypes';

export const PRESET_STYLES: Record<AgentPresetKey, { label: string; className: string }> = {
  code_builder: {
    label: 'Forge',
    className: 'bg-blue-500/10 text-blue-700 dark:text-blue-400 border-blue-500/20',
  },
  epic_planner: {
    label: 'Atlas',
    className: 'bg-violet-500/10 text-violet-700 dark:text-violet-400 border-violet-500/20',
  },
  task_planner: {
    label: 'Scribe',
    className: 'bg-fuchsia-500/10 text-fuchsia-700 dark:text-fuchsia-400 border-fuchsia-500/20',
  },
  review_agent: {
    label: 'Lens',
    className: 'bg-amber-500/10 text-amber-700 dark:text-amber-400 border-amber-500/20',
  },
  support_agent: {
    label: 'Echo',
    className: 'bg-emerald-500/10 text-emerald-700 dark:text-emerald-400 border-emerald-500/20',
  },
  documentation_agent: {
    label: 'Quill',
    className: 'bg-cyan-500/10 text-cyan-700 dark:text-cyan-400 border-cyan-500/20',
  },
  marketer: {
    label: 'Mira',
    className: 'bg-teal-500/10 text-teal-700 dark:text-teal-400 border-teal-500/20',
  },
  crm_operator: {
    label: 'Beacon',
    className: 'bg-rose-500/10 text-rose-700 dark:text-rose-400 border-rose-500/20',
  },
  command_agent: {
    label: 'Sub-agent',
    className: 'bg-slate-500/10 text-slate-700 dark:text-slate-400 border-slate-500/20',
  },
};
