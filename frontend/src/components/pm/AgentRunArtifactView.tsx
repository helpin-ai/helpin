import { useEffect, useState } from 'react';
import { File01Icon, FileCodeIcon, CheckmarkCircle02Icon, GitPullRequestIcon, BotIcon, CheckListIcon, SecurityCheckIcon, Image01Icon, PlayCircleIcon } from '@/lib/icons';
import { Badge } from '@/components/ui/badge';
import { cn } from '@/lib/utils';
import type { AgentRunArtifact } from '@/lib/pmTypes';
import { agentRunArtifactLabel } from './agentRunConstants';
import { automationService } from '@/lib/services/automationService';

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
  opencode_prompt: <File01Icon className="h-3.5 w-3.5" />,
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
  review_findings: <SecurityCheckIcon className="h-3.5 w-3.5" />,
  review_decision: <SecurityCheckIcon className="h-3.5 w-3.5" />,
  browser_screenshot: <Image01Icon className="h-3.5 w-3.5" />,
  browser_recording: <PlayCircleIcon className="h-3.5 w-3.5" />,
};

interface Props {
  artifact: AgentRunArtifact;
  reviewDecisionArtifact?: AgentRunArtifact | null;
  maxContentHeight?: string;
}

export function AgentRunArtifactView({ artifact, reviewDecisionArtifact = null, maxContentHeight = 'max-h-32' }: Props) {
  const label = agentRunArtifactLabel(artifact.artifact_type);
  const runPlan = artifact.artifact_type === 'run_plan' ? parseRunPlanArtifact(artifact.inline_content) : null;
  const reviewFindings = artifact.artifact_type === 'review_findings' ? parseReviewFindingsArtifact(artifact.inline_content) : null;
  const reviewDecision = artifact.artifact_type === 'review_decision' ? parseReviewDecisionArtifact(artifact.inline_content) : null;
  const linkedDecision = artifact.artifact_type === 'review_findings' ? parseReviewDecisionArtifact(reviewDecisionArtifact?.inline_content) : null;
  const isBrowserMedia = artifact.artifact_type === 'browser_screenshot' || artifact.artifact_type === 'browser_recording';
  const mediaKey = isBrowserMedia ? `${artifact.workspace_id}/${artifact.id}` : '';
  const [mediaState, setMediaState] = useState<{ key: string; url: string | null; error: boolean }>({ key: '', url: null, error: false });
  const mediaURL = mediaState.key === mediaKey ? mediaState.url : null;
  const mediaError = mediaState.key === mediaKey && mediaState.error;

  useEffect(() => {
    let cancelled = false;
    if (!isBrowserMedia) {
      return () => { cancelled = true; };
    }
    void automationService.getArtifactContentURL(artifact.workspace_id, artifact.id).then((response) => {
      if (cancelled) return;
      if (response.error || !response.data?.url) {
        setMediaState({ key: mediaKey, url: null, error: true });
        return;
      }
      setMediaState({ key: mediaKey, url: response.data.url, error: false });
    });
    return () => { cancelled = true; };
  }, [artifact.id, artifact.workspace_id, isBrowserMedia, mediaKey]);

  return (
    <div className="min-w-0 overflow-hidden rounded border border-border/60 bg-muted/30 p-2">
      <div className="mb-1 flex items-center gap-1.5 text-[11px] font-medium">
        {ARTIFACT_ICONS[artifact.artifact_type] ?? <File01Icon className="h-3.5 w-3.5" />}
        <span className="capitalize">{label}</span>
        <span className="text-muted-foreground">({artifact.format})</span>
      </div>
      {artifact.artifact_type === 'browser_screenshot' ? (
        mediaURL ? (
          <a href={mediaURL} target="_blank" rel="noreferrer" className="block overflow-hidden rounded border border-border/60 bg-background">
            <img
              src={mediaURL}
              alt={typeof artifact.metadata?.file_name === 'string' ? artifact.metadata.file_name : 'Browser screenshot'}
              className="max-h-72 w-full object-contain"
              onError={() => { setMediaState({ key: mediaKey, url: null, error: true }); }}
            />
          </a>
        ) : (
          <div className="rounded border border-border/60 bg-background px-2 py-4 text-center text-[11px] text-muted-foreground">
            {mediaError ? 'Screenshot is unavailable' : 'Loading screenshot…'}
          </div>
        )
      ) : artifact.artifact_type === 'browser_recording' ? (
        mediaURL ? (
          <video
            src={mediaURL}
            controls
            preload="metadata"
            className="max-h-96 w-full rounded border border-border/60 bg-black"
            onError={() => { setMediaState({ key: mediaKey, url: null, error: true }); }}
          >
            Browser recording playback is not supported by this browser.
          </video>
        ) : (
          <div className="rounded border border-border/60 bg-background px-2 py-4 text-center text-[11px] text-muted-foreground">
            {mediaError ? 'Recording is unavailable' : 'Loading recording…'}
          </div>
        )
      ) : runPlan ? (
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
      ) : reviewFindings ? (
        <div className="min-w-0 space-y-2 text-[11px] text-muted-foreground">
          {reviewFindings.title ? <div className="font-medium text-foreground">{reviewFindings.title}</div> : null}
          {reviewFindings.summary ? <p>{reviewFindings.summary}</p> : null}
          <div className="flex flex-wrap items-center gap-1.5">
            {reviewFindings.overallCorrectness ? (
              <Badge variant="outline" className="h-5 rounded-full px-1.5 text-[10px] uppercase tracking-wide">
                {reviewFindings.overallCorrectness.replaceAll('_', ' ')}
              </Badge>
            ) : null}
            {typeof reviewFindings.overallConfidenceScore === 'number' ? (
              <Badge variant="outline" className="h-5 rounded-full px-1.5 text-[10px] uppercase tracking-wide">
                {Math.round(reviewFindings.overallConfidenceScore * 100)}% confidence
              </Badge>
            ) : null}
          </div>
          {reviewFindings.overallExplanation ? <p>{reviewFindings.overallExplanation}</p> : null}
          {linkedDecision ? (
            <div className="flex flex-wrap items-center gap-1.5">
              <Badge variant="outline" className={cn('h-5 rounded-full px-1.5 text-[10px] uppercase tracking-wide', reviewDecisionBadgeClass(linkedDecision.decision))}>
                {linkedDecision.decision.replaceAll('_', ' ')}
              </Badge>
              {linkedDecision.message ? (
                <span className="text-[10px] text-muted-foreground">{linkedDecision.message}</span>
              ) : null}
            </div>
          ) : null}
          {reviewFindings.findings.length > 0 ? (
            <ul className="space-y-1">
              {reviewFindings.findings.map((finding) => (
                <li key={finding.id} className="min-w-0 overflow-hidden rounded border border-border/60 bg-background/70 px-2 py-1.5">
                  <div className="flex flex-wrap items-center gap-1.5">
                    <div className="font-medium text-foreground">{finding.title}</div>
                    {linkedDecision?.findingsById[finding.id]?.status ? (
                      <Badge variant="outline" className={cn('h-5 rounded-full px-1.5 text-[10px] uppercase tracking-wide', reviewFindingStatusBadgeClass(linkedDecision.findingsById[finding.id]!.status))}>
                        {linkedDecision.findingsById[finding.id]!.status.replaceAll('_', ' ')}
                      </Badge>
                    ) : null}
                  </div>
                  {finding.body ? <p className="mt-1">{finding.body}</p> : null}
                  <div className="mt-1 flex min-w-0 flex-wrap gap-2 text-[10px]">
                    <code className="break-all">{finding.id}</code>
                    {finding.priority ? <span>{finding.priority.toUpperCase()}</span> : null}
                    {finding.code_location ? <code className="break-all">{finding.code_location}</code> : null}
                  </div>
                </li>
              ))}
            </ul>
          ) : null}
        </div>
      ) : reviewDecision ? (
        <div className="space-y-2 text-[11px] text-muted-foreground">
          <div className="flex flex-wrap items-center gap-1.5">
            <Badge variant="outline" className={cn('h-5 rounded-full px-1.5 text-[10px] uppercase tracking-wide', reviewDecisionBadgeClass(reviewDecision.decision))}>
              {reviewDecision.decision.replaceAll('_', ' ')}
            </Badge>
            {reviewDecision.message ? <span>{reviewDecision.message}</span> : null}
          </div>
          <ul className="space-y-1">
            {reviewDecision.findings.map((finding) => (
              <li key={finding.id} className="min-w-0 overflow-hidden rounded border border-border/60 bg-background/70 px-2 py-1.5">
                <div className="flex flex-wrap items-center gap-1.5">
                  <div className="font-medium text-foreground">{finding.title || finding.id}</div>
                  <Badge variant="outline" className={cn('h-5 rounded-full px-1.5 text-[10px] uppercase tracking-wide', reviewFindingStatusBadgeClass(finding.status))}>
                    {finding.status.replaceAll('_', ' ')}
                  </Badge>
                </div>
                {finding.code_location ? <div className="mt-1 min-w-0 text-[10px]"><code className="break-all">{finding.code_location}</code></div> : null}
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

interface ReviewFindingsArtifactPayload {
  title?: string;
  summary?: string;
  overallCorrectness?: string;
  overallExplanation?: string;
  overallConfidenceScore?: number;
  findings: Array<{
    id: string;
    title: string;
    body: string;
    priority?: string;
    code_location?: string;
  }>;
}

interface ReviewDecisionArtifactPayload {
  decision: string;
  message?: string;
  findings: Array<{
    id: string;
    title?: string;
    code_location?: string;
    status: string;
  }>;
  findingsById: Record<string, { status: string }>;
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

function parseReviewFindingsArtifact(raw: string | null | undefined): ReviewFindingsArtifactPayload | null {
  if (!raw) return null;
  try {
    const parsed = JSON.parse(raw) as Record<string, unknown>;
    const findings = Array.isArray(parsed.findings) ? parsed.findings : [];
    const normalizedFindings = findings.flatMap((rawFinding, index) => {
      if (!rawFinding || typeof rawFinding !== 'object' || Array.isArray(rawFinding)) return [];
      const finding = rawFinding as Record<string, unknown>;
      const id = typeof finding.id === 'string' && finding.id.trim() ? finding.id.trim() : `finding_${index + 1}`;
      const title = typeof finding.title === 'string' ? finding.title : '';
      if (!id || !title) return [];
      return [{
        id,
        title,
        body: typeof finding.body === 'string' ? finding.body : '',
        priority: typeof finding.priority === 'string' ? finding.priority : '',
        code_location: typeof finding.code_location === 'string' ? finding.code_location : '',
      }];
    });
    const title = typeof parsed.title === 'string' ? parsed.title.trim() : '';
    const summary = typeof parsed.summary === 'string' ? parsed.summary.trim() : '';
    const overallCorrectness = typeof parsed.overall_correctness === 'string' ? parsed.overall_correctness.trim() : '';
    const overallExplanation = typeof parsed.overall_explanation === 'string' ? parsed.overall_explanation.trim() : '';
    const overallConfidenceScore = typeof parsed.overall_confidence_score === 'number' && Number.isFinite(parsed.overall_confidence_score)
      ? parsed.overall_confidence_score
      : undefined;
    if (!title && !summary && !overallCorrectness && !overallExplanation && typeof overallConfidenceScore !== 'number' && normalizedFindings.length === 0) {
      return null;
    }
    return {
      title: title || undefined,
      summary: summary || undefined,
      overallCorrectness: overallCorrectness || undefined,
      overallExplanation: overallExplanation || undefined,
      overallConfidenceScore,
      findings: normalizedFindings,
    };
  } catch {
    return null;
  }
}

function parseReviewDecisionArtifact(raw: string | null | undefined): ReviewDecisionArtifactPayload | null {
  if (!raw) return null;
  try {
    const parsed = JSON.parse(raw) as Record<string, unknown>;
    const decision = typeof parsed.decision === 'string' ? parsed.decision.trim() : '';
    const findings = Array.isArray(parsed.findings) ? parsed.findings : [];
    const normalizedFindings = findings.flatMap((rawFinding) => {
      if (!rawFinding || typeof rawFinding !== 'object' || Array.isArray(rawFinding)) return [];
      const finding = rawFinding as Record<string, unknown>;
      const id = typeof finding.id === 'string' ? finding.id.trim() : '';
      const status = typeof finding.status === 'string' ? finding.status.trim() : '';
      if (!id || !status) return [];
      return [{
        id,
        title: typeof finding.title === 'string' ? finding.title.trim() : '',
        code_location: typeof finding.code_location === 'string' ? finding.code_location.trim() : '',
        status,
      }];
    });
    if (!decision || normalizedFindings.length === 0) return null;
    return {
      decision,
      message: typeof parsed.message === 'string' ? parsed.message.trim() : '',
      findings: normalizedFindings,
      findingsById: Object.fromEntries(normalizedFindings.map((finding) => [finding.id, { status: finding.status }])),
    };
  } catch {
    return null;
  }
}

function reviewDecisionBadgeClass(decision: string) {
  switch (decision.trim()) {
    case 'approve':
      return 'border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-900/60 dark:bg-emerald-950/30 dark:text-emerald-300';
    case 'skip':
      return 'border-muted bg-muted text-muted-foreground';
    default:
      return 'border-orange-200 bg-orange-50 text-orange-700 dark:border-orange-900/60 dark:bg-orange-950/30 dark:text-orange-300';
  }
}

function reviewFindingStatusBadgeClass(status: string) {
  switch (status.trim()) {
    case 'approved':
      return 'border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-900/60 dark:bg-emerald-950/30 dark:text-emerald-300';
    case 'requested_changes':
      return 'border-orange-200 bg-orange-50 text-orange-700 dark:border-orange-900/60 dark:bg-orange-950/30 dark:text-orange-300';
    case 'skipped':
      return 'border-muted bg-muted text-muted-foreground';
    default:
      return 'border-muted bg-muted text-muted-foreground';
  }
}
