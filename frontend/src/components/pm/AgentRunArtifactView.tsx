import { FileText, FileCode, CheckCircle2, GitPullRequest, Bot } from 'lucide-react';
import type { AgentRunArtifact } from '@/lib/pmTypes';
import { ARTIFACT_TYPE_LABELS } from './agentRunConstants';

const ARTIFACT_ICONS: Record<string, React.ReactNode> = {
  conversation_log: <FileText className="h-3.5 w-3.5" />,
  tool_log: <FileText className="h-3.5 w-3.5" />,
  diff: <FileCode className="h-3.5 w-3.5" />,
  test_report: <CheckCircle2 className="h-3.5 w-3.5" />,
  pr_metadata: <GitPullRequest className="h-3.5 w-3.5" />,
  agent_summary: <Bot className="h-3.5 w-3.5" />,
  file_bundle: <FileCode className="h-3.5 w-3.5" />,
  handoff_note: <FileText className="h-3.5 w-3.5" />,
  opencode_config: <FileText className="h-3.5 w-3.5" />,
  opencode_stdout: <FileText className="h-3.5 w-3.5" />,
  opencode_stderr: <FileText className="h-3.5 w-3.5" />,
  codex_config: <FileText className="h-3.5 w-3.5" />,
  codex_prompt: <FileText className="h-3.5 w-3.5" />,
  codex_response: <Bot className="h-3.5 w-3.5" />,
  codex_stdout: <FileText className="h-3.5 w-3.5" />,
  codex_stderr: <FileText className="h-3.5 w-3.5" />,
  git_status: <FileCode className="h-3.5 w-3.5" />,
  git_diff_stat: <FileCode className="h-3.5 w-3.5" />,
  git_persistence_result: <FileCode className="h-3.5 w-3.5" />,
};

interface Props {
  artifact: AgentRunArtifact;
  maxContentHeight?: string;
}

export function AgentRunArtifactView({ artifact, maxContentHeight = 'max-h-32' }: Props) {
  const label = ARTIFACT_TYPE_LABELS[artifact.artifact_type] ?? artifact.artifact_type.replace(/_/g, ' ');

  return (
    <div className="rounded border border-border/60 bg-muted/30 p-2">
      <div className="mb-1 flex items-center gap-1.5 text-[11px] font-medium">
        {ARTIFACT_ICONS[artifact.artifact_type] ?? <FileText className="h-3.5 w-3.5" />}
        <span className="capitalize">{label}</span>
        <span className="text-muted-foreground">({artifact.format})</span>
      </div>
      {artifact.inline_content && (
        <pre className={`${maxContentHeight} overflow-auto whitespace-pre-wrap break-all text-[10px] text-muted-foreground`}>
          {artifact.inline_content.slice(0, 2000)}
          {artifact.inline_content.length > 2000 && '...'}
        </pre>
      )}
    </div>
  );
}
