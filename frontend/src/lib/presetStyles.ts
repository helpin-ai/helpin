import type { AgentPresetKey } from '@/lib/pmTypes';

export const PRESET_STYLES: Record<AgentPresetKey, { label: string; className: string }> = {
  code_builder: {
    label: 'Code Builder',
    className: 'bg-blue-500/10 text-blue-700 dark:text-blue-400 border-blue-500/20',
  },
  epic_planner: {
    label: 'Epic Planner',
    className: 'bg-violet-500/10 text-violet-700 dark:text-violet-400 border-violet-500/20',
  },
  task_planner: {
    label: 'Task Planner',
    className: 'bg-fuchsia-500/10 text-fuchsia-700 dark:text-fuchsia-400 border-fuchsia-500/20',
  },
  /** @deprecated Use task_planner */
  story_planner: {
    label: 'Task Planner',
    className: 'bg-fuchsia-500/10 text-fuchsia-700 dark:text-fuchsia-400 border-fuchsia-500/20',
  },
  review_agent: {
    label: 'Review Agent',
    className: 'bg-amber-500/10 text-amber-700 dark:text-amber-400 border-amber-500/20',
  },
  support_agent: {
    label: 'Support Agent',
    className: 'bg-emerald-500/10 text-emerald-700 dark:text-emerald-400 border-emerald-500/20',
  },
  documentation_agent: {
    label: 'Quill',
    className: 'bg-cyan-500/10 text-cyan-700 dark:text-cyan-400 border-cyan-500/20',
  },
  crm_operator: {
    label: 'Beacon',
    className: 'bg-rose-500/10 text-rose-700 dark:text-rose-400 border-rose-500/20',
  },
  command_agent: {
    label: 'Command Agent',
    className: 'bg-slate-500/10 text-slate-700 dark:text-slate-400 border-slate-500/20',
  },
};
