import { File01Icon, FileCodeIcon, CheckmarkCircle02Icon, GitPullRequestIcon, BotIcon, CheckListIcon } from '@/lib/icons';
import type { AgentRunArtifact } from '@/lib/pmTypes';
import { ARTIFACT_TYPE_LABELS } from './agentRunConstants';

const ARTIFACT_ICONS: Record<string, React.ReactNode> = {
  conversation_log: <File01Icon className="h-3.5 w-3.5" />,
  tool_log: <File01Icon className="h-3.5 w-3.5" />,
  diff: <FileCodeIcon className="h-3.5 w-3.5" />,
  test_report: <CheckmarkCircle02Icon className="h-3.5 w-3.5" />,
  pr_metadata: <GitPullRequestIcon className="h-3.5 w-3.5" />,
  agent_summary: <BotIcon className="h-3.5 w-3.5" />,
  file_bundle: <FileCodeIcon className="h-3.5 w-3.5" />,
  handoff_note: <File01Icon className="h-3.5 w-3.5" />,
  opencode_config: <File01Icon className="h-3.5 w-3.5" />,
  opencode_stdout: <File01Icon className="h-3.5 w-3.5" />,
  opencode_stderr: <File01Icon className="h-3.5 w-3.5" />,
  codex_config: <File01Icon className="h-3.5 w-3.5" />,
  codex_prompt: <File01Icon className="h-3.5 w-3.5" />,
  codex_response: <BotIcon className="h-3.5 w-3.5" />,
  codex_stdout: <File01Icon className="h-3.5 w-3.5" />,
  codex_stderr: <File01Icon className="h-3.5 w-3.5" />,
  git_status: <FileCodeIcon className="h-3.5 w-3.5" />,
  git_diff_stat: <FileCodeIcon className="h-3.5 w-3.5" />,
  git_persistence_result: <FileCodeIcon className="h-3.5 w-3.5" />,
  run_plan: <CheckListIcon className="h-3.5 w-3.5" />,
};

interface Props {
  artifact: AgentRunArtifact;
  maxContentHeight?: string;
}

export function AgentRunArtifactView({ artifact, maxContentHeight = 'max-h-32' }: Props) {
  const label = ARTIFACT_TYPE_LABELS[artifact.artifact_type] ?? artifact.artifact_type.replace(/_/g, ' ');
  const runPlan = artifact.artifact_type === 'run_plan' ? parseRunPlanArtifact(artifact.inline_content) : null;

  return (
    <div className="rounded border border-border/60 bg-muted/30 p-2">
      <div className="mb-1 flex items-center gap-1.5 text-[11px] font-medium">
        {ARTIFACT_ICONS[artifact.artifact_type] ?? <File01Icon className="h-3.5 w-3.5" />}
        <span className="capitalize">{label}</span>
        <span className="text-muted-foreground">({artifact.format})</span>
      </div>
      {runPlan ? (
        <div className="space-y-1 text-[11px] text-muted-foreground">
          {runPlan.note ? <p>{runPlan.note}</p> : null}
          <ul className="space-y-1">
            {runPlan.plan.map((step, index) => (
              <li key={`${index}-${step.step}`} className="flex items-start gap-1.5">
                <span className="mt-0.5 inline-block min-w-4 text-[10px] font-medium text-foreground/70">
                  {step.status === 'completed' ? '[x]' : step.status === 'in_progress' ? '[>]' : '[ ]'}
                </span>
                <span>{step.step}</span>
              </li>
            ))}
          </ul>
        </div>
      ) : artifact.inline_content && (
        <pre className={`${maxContentHeight} overflow-auto whitespace-pre-wrap break-all text-[10px] text-muted-foreground`}>
          {artifact.inline_content.slice(0, 2000)}
          {artifact.inline_content.length > 2000 && '...'}
        </pre>
      )}
    </div>
  );
}

interface RunPlanArtifactPayload {
  note?: string;
  plan: Array<{ step: string; status: 'pending' | 'in_progress' | 'completed' }>;
}

function parseRunPlanArtifact(raw: string | null | undefined): RunPlanArtifactPayload | null {
  if (!raw) return null;
  try {
    const parsed = JSON.parse(raw) as Partial<RunPlanArtifactPayload>;
    if (!Array.isArray(parsed.plan) || parsed.plan.length === 0) return null;
    return {
      note: typeof parsed.note === 'string' ? parsed.note : undefined,
      plan: parsed.plan.filter((step): step is RunPlanArtifactPayload['plan'][number] => {
        return !!step && typeof step.step === 'string' && typeof step.status === 'string';
      }),
    };
  } catch {
    return null;
  }
}
