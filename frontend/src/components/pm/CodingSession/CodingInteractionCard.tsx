import { useEffect, useState, type ReactNode } from 'react';
import { CheckmarkCircle02Icon, GitCommitIcon, SecurityCheckIcon } from '@/lib/icons';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';
import type { CodingSessionInteraction } from '@/lib/pmTypes';
import { cn } from '@/lib/utils';

interface Props {
  interaction: CodingSessionInteraction;
  acting: string | null;
  onResolve: (interactionId: string, responsePayload: Record<string, unknown>, followupMessage?: string) => void;
  compact?: boolean;
}

interface QuestionAnswerState {
  value?: string;
  freetext?: string;
}

export function CodingInteractionCard({ interaction, acting, onResolve, compact = false }: Props) {
  const [questionAnswers, setQuestionAnswers] = useState<Record<string, QuestionAnswerState>>({});
  const [followupMessage, setFollowupMessage] = useState('');
  const [currentQuestionIndex, setCurrentQuestionIndex] = useState(0);

  useEffect(() => {
    setQuestionAnswers({});
    setFollowupMessage('');
    setCurrentQuestionIndex(0);
  }, [interaction.interaction_id]);

  const isBusy = acting !== null;
  const requestPayload = interaction.request_payload ?? {};

  if (interaction.interaction_kind === 'request_user_input') {
    const codexQuestions = parseCodexUserInputQuestions(requestPayload);
    if (interaction.request_schema_version === 'codex.v2' && codexQuestions.length > 0) {
      const allAnswered = codexQuestions.every((question) => isCodexQuestionAnswered(question, questionAnswers[question.id]));
      const showStepper = compact && codexQuestions.length > 1;
      const visibleQuestions = showStepper ? [codexQuestions[Math.min(currentQuestionIndex, codexQuestions.length - 1)]] : codexQuestions;
      const currentQuestion = codexQuestions[Math.min(currentQuestionIndex, codexQuestions.length - 1)];
      const currentAnswered = currentQuestion ? isCodexQuestionAnswered(currentQuestion, questionAnswers[currentQuestion.id]) : false;

      return (
        <InteractionShell compact={compact}
          icon={<CheckmarkCircle02Icon className="h-4 w-4" />}
          eyebrow="User input required"
          title={interaction.title ?? 'Answer the pending questions'}
          summary={interaction.summary}
        >
          {showStepper ? (
            <div className="mb-3 flex items-center justify-between text-[11px] text-muted-foreground">
              <span>Question {currentQuestionIndex + 1} of {codexQuestions.length}</span>
              <span>{Object.values(questionAnswers).filter((answer) => Boolean(answer?.value || answer?.freetext?.trim())).length} answered</span>
            </div>
          ) : null}
          <div className="space-y-4">
            {visibleQuestions.map((question) => {
              const answer = questionAnswers[question.id];
              const isSecret = question.isSecret;
              return (
                <div key={question.id} className="space-y-3 rounded-lg border border-border bg-muted/25 p-3">
                  <div>
                    {question.header ? (
                      <div className="text-[11px] font-medium uppercase tracking-wide text-muted-foreground">{question.header}</div>
                    ) : null}
                    <div className={cn('mt-1 font-medium text-foreground', compact ? 'text-sm leading-5' : 'text-sm')}>{question.question}</div>
                  </div>
                  {question.options.length > 0 ? (
                    <div className="space-y-2">
                      {question.options.map((option) => {
                        const selected = answer?.value === option.label;
                        return (
                          <button
                            key={option.label}
                            type="button"
                            onClick={() => setQuestionAnswers((current) => ({
                              ...current,
                              [question.id]: { value: option.label },
                            }))}
                            className={cn(
                              'w-full rounded-lg border px-3 py-2 text-left transition-colors',
                              selected
                                ? 'border-primary/50 bg-primary/5 dark:bg-primary/10'
                                : 'border-border/60 bg-background hover:border-border hover:bg-accent/40',
                            )}
                            disabled={isBusy}
                          >
                            <div className={cn('font-medium text-foreground', compact ? 'text-sm leading-5' : 'text-sm')}>{option.label}</div>
                            {option.description ? (
                              <div className="mt-1 text-xs leading-5 text-muted-foreground">{option.description}</div>
                            ) : null}
                          </button>
                        );
                      })}
                      {question.isOther ? (
                        <button
                          type="button"
                          onClick={() => setQuestionAnswers((current) => ({
                            ...current,
                            [question.id]: { ...current[question.id], value: '__other__' },
                          }))}
                          className={cn(
                            'w-full rounded-lg border px-3 py-2 text-left transition-colors',
                            answer?.value === '__other__'
                              ? 'border-primary/50 bg-primary/5 dark:bg-primary/10'
                              : 'border-border/60 bg-background hover:border-border hover:bg-accent/40',
                          )}
                          disabled={isBusy}
                        >
                          <div className={cn('font-medium text-foreground', compact ? 'text-sm leading-5' : 'text-sm')}>Other</div>
                          <div className="mt-1 text-xs leading-5 text-muted-foreground">Provide a custom reply.</div>
                        </button>
                      ) : null}
                    </div>
                  ) : null}

                  {(!question.options.length || answer?.value === '__other__') ? (
                    <Input
                      type={isSecret ? 'password' : 'text'}
                      value={answer?.freetext ?? ''}
                      onChange={(event) => setQuestionAnswers((current) => ({
                        ...current,
                        [question.id]: { ...current[question.id], freetext: event.target.value },
                      }))}
                      placeholder="Type your answer"
                      className={compact ? 'text-sm' : undefined}
                      disabled={isBusy}
                    />
                  ) : null}
                </div>
              );
            })}
          </div>
          <div className="mt-4 flex flex-wrap items-center gap-2">
            {showStepper ? (
              <Button
                size="sm"
                variant="outline"
                disabled={currentQuestionIndex === 0 || isBusy}
                onClick={() => setCurrentQuestionIndex((current) => Math.max(0, current - 1))}
              >
                Back
              </Button>
            ) : null}
            {showStepper && currentQuestionIndex < codexQuestions.length - 1 ? (
              <Button
                size="sm"
                disabled={!currentAnswered || isBusy}
                onClick={() => setCurrentQuestionIndex((current) => Math.min(codexQuestions.length - 1, current + 1))}
              >
                Next
              </Button>
            ) : (
              <Button
                size="sm"
                disabled={!allAnswered || isBusy}
                onClick={() => onResolve(interaction.interaction_id, buildCodexUserInputResponsePayload(codexQuestions, questionAnswers))}
              >
                Submit answers
              </Button>
            )}
          </div>
        </InteractionShell>
      );
    }

    const helpinQuestions = parseHelpinQuestions(requestPayload);
    if (helpinQuestions.length > 0) {
      const allAnswered = helpinQuestions.every((question) => isHelpinQuestionAnswered(question, questionAnswers[question.id]));
      const showStepper = compact && helpinQuestions.length > 1;
      const visibleQuestions = showStepper ? [helpinQuestions[Math.min(currentQuestionIndex, helpinQuestions.length - 1)]] : helpinQuestions;
      const currentQuestion = helpinQuestions[Math.min(currentQuestionIndex, helpinQuestions.length - 1)];
      const currentAnswered = currentQuestion ? isHelpinQuestionAnswered(currentQuestion, questionAnswers[currentQuestion.id]) : false;

      return (
        <InteractionShell compact={compact}
          icon={<CheckmarkCircle02Icon className="h-4 w-4" />}
          eyebrow="User input required"
          title={interaction.title ?? 'Answer the pending questions'}
          summary={interaction.summary}
        >
          {showStepper ? (
            <div className="mb-3 flex items-center justify-between text-[11px] text-muted-foreground">
              <span>Question {currentQuestionIndex + 1} of {helpinQuestions.length}</span>
              <span>{Object.values(questionAnswers).filter((answer) => Boolean(answer?.value || answer?.freetext?.trim())).length} answered</span>
            </div>
          ) : null}
          <div className="space-y-4">
            {visibleQuestions.map((question) => {
              const answer = questionAnswers[question.id];
              const selectedOption = question.options.find((option) => option.value === answer?.value);
              return (
                <div key={question.id} className="space-y-3 rounded-lg border border-border bg-muted/25 p-3">
                  <div className={cn('font-medium text-foreground', compact ? 'text-sm leading-5' : 'text-sm')}>{question.text}</div>
                  <div className="space-y-2">
                    {question.options.map((option) => {
                      const selected = answer?.value === option.value;
                      return (
                        <button
                          key={option.value}
                          type="button"
                          onClick={() => setQuestionAnswers((current) => ({
                            ...current,
                            [question.id]: { value: option.value, freetext: option.freetext ? current[question.id]?.freetext : undefined },
                          }))}
                          className={cn(
                            'w-full rounded-lg border px-3 py-2 text-left transition-colors',
                            selected
                              ? 'border-primary/50 bg-primary/5 dark:bg-primary/10'
                              : 'border-border/60 bg-background hover:border-border hover:bg-accent/40',
                          )}
                          disabled={isBusy}
                        >
                          <div className={cn('font-medium text-foreground', compact ? 'text-sm leading-5' : 'text-sm')}>{option.label}</div>
                        </button>
                      );
                    })}
                  </div>
                  {selectedOption?.freetext ? (
                    <Input
                      value={answer?.freetext ?? ''}
                      onChange={(event) => setQuestionAnswers((current) => ({
                        ...current,
                        [question.id]: { ...current[question.id], freetext: event.target.value },
                      }))}
                      placeholder="Please specify"
                      className={compact ? 'text-sm' : undefined}
                      disabled={isBusy}
                    />
                  ) : null}
                </div>
              );
            })}
          </div>
          <div className="mt-4 flex flex-wrap items-center gap-2">
            {showStepper ? (
              <Button
                size="sm"
                variant="outline"
                disabled={currentQuestionIndex === 0 || isBusy}
                onClick={() => setCurrentQuestionIndex((current) => Math.max(0, current - 1))}
              >
                Back
              </Button>
            ) : null}
            {showStepper && currentQuestionIndex < helpinQuestions.length - 1 ? (
              <Button
                size="sm"
                disabled={!currentAnswered || isBusy}
                onClick={() => setCurrentQuestionIndex((current) => Math.min(helpinQuestions.length - 1, current + 1))}
              >
                Next
              </Button>
            ) : (
              <Button
                size="sm"
                disabled={!allAnswered || isBusy}
                onClick={() => onResolve(interaction.interaction_id, buildHelpinUserInputResponsePayload(helpinQuestions, questionAnswers))}
              >
                Submit answers
              </Button>
            )}
          </div>
        </InteractionShell>
      );
    }
  }

  if (interaction.interaction_kind === 'review_checkpoint') {
    const checkpoint = parseReviewCheckpointRequest(requestPayload);
    return (
      <InteractionShell compact={compact}
        icon={<SecurityCheckIcon className="h-4 w-4" />}
        eyebrow={checkpoint?.phase ? `${checkpoint.phase} review checkpoint` : 'Review checkpoint'}
        title={interaction.title ?? checkpoint?.title ?? 'Review required'}
        summary={interaction.summary ?? checkpoint?.summary}
      >
        <Textarea
          value={followupMessage}
          onChange={(event) => setFollowupMessage(event.target.value)}
          placeholder="Optional note for the agent"
          className="min-h-[76px]"
          disabled={isBusy}
        />
        <div className="mt-4 flex flex-wrap gap-2">
          <Button
            size="sm"
            disabled={isBusy}
            onClick={() => onResolve(interaction.interaction_id, { decision: 'approve' }, followupMessage.trim() || undefined)}
          >
            Approve
          </Button>
          <Button
            variant="outline"
            size="sm"
            disabled={isBusy}
            onClick={() => onResolve(
              interaction.interaction_id,
              {
                decision: 'request_changes',
                ...(followupMessage.trim() ? { message: followupMessage.trim() } : {}),
              },
              followupMessage.trim() || undefined,
            )}
          >
            Request changes
          </Button>
        </div>
      </InteractionShell>
    );
  }

  if (interaction.interaction_kind === 'permissions_approval') {
    const permissions = parsePermissionsRequest(requestPayload);
    const requestedPermissions = permissions?.permissions ?? {};
    return (
      <InteractionShell compact={compact}
        icon={<SecurityCheckIcon className="h-4 w-4" />}
        eyebrow="Permissions approval"
        title={interaction.title ?? 'Approve additional permissions'}
        summary={interaction.summary}
      >
        <div className={cn('space-y-2 rounded-lg border border-border bg-muted/25 p-3', compact ? 'text-xs' : 'text-sm')}>
          <div><span className="font-medium text-foreground">Reason:</span> <span className="text-muted-foreground">{permissions?.reason ?? 'No reason provided.'}</span></div>
          <pre className="overflow-auto whitespace-pre-wrap rounded-lg border border-border bg-slate-950 px-3 py-2 text-[11px] leading-5 text-slate-100">
            {JSON.stringify(requestedPermissions, null, 2)}
          </pre>
        </div>
        <div className="mt-4 flex flex-wrap gap-2">
          <Button
            size="sm"
            disabled={isBusy}
            onClick={() => onResolve(interaction.interaction_id, { permissions: requestedPermissions, scope: 'turn' })}
          >
            Allow for turn
          </Button>
          <Button
            variant="outline"
            size="sm"
            disabled={isBusy}
            onClick={() => onResolve(interaction.interaction_id, { permissions: requestedPermissions, scope: 'session' })}
          >
            Allow for session
          </Button>
          <Button
            variant="outline"
            size="sm"
            disabled={isBusy}
            onClick={() => onResolve(interaction.interaction_id, { permissions: {}, scope: 'turn' })}
          >
            Deny
          </Button>
        </div>
      </InteractionShell>
    );
  }

  if (interaction.interaction_kind === 'command_execution_approval' || interaction.interaction_kind === 'file_change_approval') {
    const runtimeApproval = parseRuntimeApprovalRequest(interaction);
    const decisions = runtimeApproval.decisions.length > 0
      ? runtimeApproval.decisions
      : interaction.interaction_kind === 'command_execution_approval'
        ? ['accept', 'acceptForSession', 'decline', 'cancel']
        : ['accept', 'acceptForSession', 'decline', 'cancel'];

    return (
      <InteractionShell compact={compact}
        icon={<GitCommitIcon className="h-4 w-4" />}
        eyebrow={interaction.interaction_kind === 'command_execution_approval' ? 'Command approval' : 'File-change approval'}
        title={interaction.title ?? runtimeApproval.title}
      >
        <CommandApprovalDetails command={runtimeApproval.command} reason={runtimeApproval.reason} grantRoot={runtimeApproval.grantRoot} cwd={runtimeApproval.cwd} compact={compact} />
        <Textarea
          value={followupMessage}
          onChange={(event) => setFollowupMessage(event.target.value)}
          placeholder="Optional follow-up message if you want the agent to revise after denying"
          className="mt-4 min-h-[76px]"
          disabled={isBusy}
        />
        <div className="mt-4 flex flex-wrap gap-2">
          {decisions.map((decision) => (
            <Button
              key={decision}
              size="sm"
              variant={decision.startsWith('accept') ? 'default' : 'outline'}
              disabled={isBusy}
              onClick={() => onResolve(interaction.interaction_id, { decision }, followupMessage.trim() || undefined)}
            >
              {labelForDecision(decision)}
            </Button>
          ))}
        </div>
      </InteractionShell>
    );
  }

  return (
    <InteractionShell compact={compact}
      icon={<SecurityCheckIcon className="h-4 w-4" />}
      eyebrow="Interaction"
      title={interaction.title ?? interaction.interaction_kind.replaceAll('_', ' ')}
      summary={interaction.summary}
    >
      <pre className="overflow-auto whitespace-pre-wrap rounded-lg border border-border bg-slate-950 px-3 py-2 text-[11px] leading-5 text-slate-100">
        {JSON.stringify(interaction.request_payload, null, 2)}
      </pre>
    </InteractionShell>
  );
}

function InteractionShell({
  icon,
  eyebrow,
  title,
  summary,
  children,
  compact = false,
}: {
  icon: ReactNode;
  eyebrow: string;
  title: string;
  summary?: string;
  children: ReactNode;
  compact?: boolean;
}) {
  return (
    <div
      className={cn(
        'rounded-xl border bg-card',
        compact
          ? 'border-amber-200/80 bg-[color:color-mix(in_oklch,var(--card)_82%,oklch(0.94_0.03_72)_18%)] p-3 dark:border-border dark:bg-card'
          : 'border-border p-4',
      )}
    >
      <div className={cn('mb-1.5 flex items-center gap-2 font-medium uppercase tracking-wide text-muted-foreground', compact ? 'text-[10px]' : 'text-[11px]')}>
        {icon}
        {eyebrow}
      </div>
      <div className={cn('font-semibold', compact ? 'text-sm' : 'text-base')}>{title}</div>
      {summary ? (
        <p className={cn('mt-1 text-muted-foreground', compact ? 'text-sm leading-5' : 'text-sm')}>{summary}</p>
      ) : null}
      <div className={compact ? 'mt-3' : 'mt-4'}>{children}</div>
    </div>
  );
}

function isCodexQuestionAnswered(
  question: ReturnType<typeof parseCodexUserInputQuestions>[number],
  answer?: QuestionAnswerState,
) {
  if (!answer) return false;
  if (!question.options.length || answer.value === '__other__') {
    return Boolean(answer.freetext?.trim());
  }
  return Boolean(answer.value?.trim());
}

function parseCodexUserInputQuestions(payload: Record<string, unknown>) {
  const questions = Array.isArray(payload.questions) ? payload.questions : [];
  return questions.flatMap((rawQuestion) => {
    if (!rawQuestion || typeof rawQuestion !== 'object' || Array.isArray(rawQuestion)) return [];
    const question = rawQuestion as Record<string, unknown>;
    const id = typeof question.id === 'string' ? question.id.trim() : '';
    const prompt = typeof question.question === 'string' ? question.question.trim() : '';
    if (!id || !prompt) return [];
    const options = Array.isArray(question.options) ? question.options : [];
    return [{
      id,
      header: typeof question.header === 'string' ? question.header.trim() : '',
      question: prompt,
      isOther: question.isOther === true,
      isSecret: question.isSecret === true,
      options: options.flatMap((rawOption) => {
        if (!rawOption || typeof rawOption !== 'object' || Array.isArray(rawOption)) return [];
        const option = rawOption as Record<string, unknown>;
        const label = typeof option.label === 'string' ? option.label.trim() : '';
        if (!label) return [];
        return [{
          label,
          description: typeof option.description === 'string' ? option.description.trim() : '',
        }];
      }),
    }];
  });
}

function buildCodexUserInputResponsePayload(
  questions: ReturnType<typeof parseCodexUserInputQuestions>,
  answers: Record<string, QuestionAnswerState>,
) {
  const payload: Record<string, { answers: string[] }> = {};
  for (const question of questions) {
    const answer = answers[question.id];
    const value = answer?.value === '__other__' || !question.options.length
      ? answer?.freetext?.trim()
      : answer?.value?.trim();
    if (!value) continue;
    payload[question.id] = { answers: [value] };
  }
  return { answers: payload };
}

function isHelpinQuestionAnswered(
  question: ReturnType<typeof parseHelpinQuestions>[number],
  answer?: QuestionAnswerState,
) {
  if (!answer?.value) return false;
  const selectedOption = question.options.find((option) => option.value === answer.value);
  if (selectedOption?.freetext) {
    return Boolean(answer.freetext?.trim());
  }
  return true;
}

function parseHelpinQuestions(payload: Record<string, unknown>) {
  const questions = Array.isArray(payload.questions) ? payload.questions : [];
  return questions.flatMap((rawQuestion) => {
    if (!rawQuestion || typeof rawQuestion !== 'object' || Array.isArray(rawQuestion)) return [];
    const question = rawQuestion as Record<string, unknown>;
    const id = typeof question.id === 'string' ? question.id.trim() : '';
    const text = typeof question.text === 'string' ? question.text.trim() : '';
    if (!id || !text) return [];
    const options = Array.isArray(question.options) ? question.options : [];
    return [{
      id,
      text,
      options: options.flatMap((rawOption) => {
        if (!rawOption || typeof rawOption !== 'object' || Array.isArray(rawOption)) return [];
        const option = rawOption as Record<string, unknown>;
        const value = typeof option.value === 'string' ? option.value.trim() : '';
        const label = typeof option.label === 'string' ? option.label.trim() : '';
        if (!value || !label) return [];
        return [{
          value,
          label,
          freetext: option.freetext === true,
        }];
      }),
    }];
  });
}

function buildHelpinUserInputResponsePayload(
  questions: ReturnType<typeof parseHelpinQuestions>,
  answers: Record<string, QuestionAnswerState>,
) {
  const entries = questions.flatMap((question) => {
    const answer = answers[question.id];
    if (!answer?.value) return [];
    const option = question.options.find((candidate) => candidate.value === answer.value);
    if (!option) return [];
    return [{
      question_id: question.id,
      question: question.text,
      selected_value: option.value,
      selected_label: option.label,
      freetext: option.freetext ? answer.freetext?.trim() : undefined,
    }];
  });
  const lines = entries.map((entry) => (
    entry.freetext
      ? `- ${entry.question_id}: ${entry.question} -> ${entry.selected_label} (${entry.freetext})`
      : `- ${entry.question_id}: ${entry.question} -> ${entry.selected_label}`
  ));
  return {
    content: lines.join('\n'),
    answers: entries,
  };
}

function parseReviewCheckpointRequest(payload: Record<string, unknown>) {
  return {
    phase: typeof payload.phase === 'string' ? payload.phase.trim() : '',
    title: typeof payload.title === 'string' ? payload.title.trim() : '',
    summary: typeof payload.summary === 'string' ? payload.summary.trim() : '',
  };
}

function parsePermissionsRequest(payload: Record<string, unknown>) {
  return {
    reason: typeof payload.reason === 'string' ? payload.reason.trim() : '',
    permissions: payload.permissions && typeof payload.permissions === 'object' && !Array.isArray(payload.permissions)
      ? payload.permissions as Record<string, unknown>
      : {},
  };
}

const COMMAND_COLLAPSED_LINES = 6;

function extractFilePaths(command: string): string[] {
  const matches = command.match(/(?:^|\s)((?:\/|\.\.?\/)?[\w./-]+\.(?:ts|tsx|js|jsx|go|py|css|html|json|sql|md|yaml|yml|toml|sh))\b/g);
  if (!matches) return [];
  const unique = [...new Set(matches.map((m) => m.trim()))];
  return unique.slice(0, 8);
}

function CommandApprovalDetails({ command, reason, grantRoot, cwd, compact }: { command: string; reason: string; grantRoot: string; cwd: string; compact?: boolean }) {
  const [expanded, setExpanded] = useState(false);
  const lines = command.split('\n');
  const isLong = lines.length > COMMAND_COLLAPSED_LINES;
  const filePaths = command ? extractFilePaths(command) : [];

  return (
    <div className={cn('space-y-2', compact ? 'text-xs' : 'text-sm')}>
      {filePaths.length > 0 && (
        <div className="flex flex-wrap gap-1.5">
          {filePaths.map((fp) => (
            <span key={fp} className="inline-flex items-center rounded-md bg-primary/10 px-2 py-0.5 text-xs font-medium text-primary">
              {fp}
            </span>
          ))}
        </div>
      )}
      {command ? (
        <div className="relative">
          <pre className={cn(
            'overflow-auto whitespace-pre-wrap break-all rounded-lg border border-border bg-slate-950 px-3 py-2 text-[11px] leading-5 text-slate-100',
            !expanded && isLong && 'max-h-[156px]',
            expanded && 'max-h-80',
          )}>
            {expanded || !isLong ? command : lines.slice(0, COMMAND_COLLAPSED_LINES).join('\n')}
          </pre>
          {isLong && !expanded && (
            <div className="pointer-events-none absolute inset-x-0 bottom-0 h-10 rounded-b-lg bg-gradient-to-t from-slate-950 to-transparent" />
          )}
          {isLong && (
            <button
              type="button"
              className="mt-1 text-xs font-medium text-primary hover:underline"
              onClick={() => setExpanded((prev) => !prev)}
            >
              {expanded ? 'Show less' : `Show more (${lines.length} lines)`}
            </button>
          )}
        </div>
      ) : null}
      {(reason || grantRoot || cwd) ? (
        <div className={cn('space-y-1 text-muted-foreground', compact ? 'text-xs' : 'text-sm')}>
          {cwd ? (
            <div><span className="font-medium text-foreground">Directory:</span> <code className="text-xs">{cwd}</code></div>
          ) : null}
          {reason ? (
            <div><span className="font-medium text-foreground">Reason:</span> {reason}</div>
          ) : null}
          {grantRoot ? (
            <div><span className="font-medium text-foreground">Grant root:</span> <code className="text-xs">{grantRoot}</code></div>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}

function parseRuntimeApprovalRequest(interaction: CodingSessionInteraction) {
  const payload = interaction.request_payload ?? {};
  const availableDecisions = Array.isArray(payload.availableDecisions) ? payload.availableDecisions : [];
  return {
    title: interaction.interaction_kind === 'command_execution_approval' ? 'Approve command execution' : 'Approve file changes',
    summary: interaction.summary ?? '',
    command: typeof payload.command === 'string' ? payload.command.trim() : '',
    cwd: typeof payload.cwd === 'string' ? payload.cwd.trim() : '',
    reason: typeof payload.reason === 'string' ? payload.reason.trim() : '',
    grantRoot: typeof payload.grantRoot === 'string' ? payload.grantRoot.trim() : '',
    decisions: availableDecisions.flatMap((decision) => {
      if (typeof decision === 'string' && decision.trim()) return [decision.trim()];
      return [];
    }),
  };
}

function labelForDecision(decision: string) {
  switch (decision) {
    case 'accept':
      return 'Approve';
    case 'acceptForSession':
      return 'Approve for session';
    case 'acceptWithExecpolicyAmendment':
      return 'Approve with rule';
    case 'applyNetworkPolicyAmendment':
      return 'Apply network rule';
    case 'decline':
      return 'Decline';
    case 'cancel':
      return 'Cancel turn';
    default:
      return decision;
  }
}
