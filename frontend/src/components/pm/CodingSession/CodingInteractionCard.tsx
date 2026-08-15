import { useMemo, useState, type ReactNode } from 'react';
import { ArrowDown02Icon, ArrowUp02Icon, CheckmarkCircle02Icon, ExpandIcon, File01Icon, GitCommitIcon, SecurityCheckIcon } from '@/lib/icons';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';
import type { PublishedPreview } from '@/components/pm/runPreviews';
import type {
  CodingSessionApprovalRequestPayload,
  CodingSessionApprovalResponsePayload,
  CodingSessionInteraction,
  CodingSessionReviewCheckpointRequestPayload,
  CodingSessionReviewCheckpointResponsePayload,
  CodingSessionReviewFinding,
} from '@/lib/pmTypes';
import { cn } from '@/lib/utils';
import { MarkdownContent } from './MarkdownContent';

interface Props {
  interaction: CodingSessionInteraction;
  acting: string | null;
  onResolve: (interactionId: string, responsePayload: Record<string, unknown>, followupMessage?: string) => void;
  compact?: boolean;
  availablePreviewPanelKey?: string | null;
  attachedPreview?: PublishedPreview | null;
  onViewPreview?: (panelKey: string) => void;
}

interface QuestionAnswerState {
  value?: string;
  freetext?: string;
}

const APPROVAL_PREVIEW_COLLAPSED_LENGTH = 480;
const approveButtonClassName = 'border-emerald-600 bg-emerald-600 text-white hover:bg-emerald-700 hover:text-white dark:border-emerald-500 dark:bg-emerald-600 dark:hover:bg-emerald-500';

export function CodingInteractionCard(props: Props) {
  return <CodingInteractionCardContent key={props.interaction.interaction_id} {...props} />;
}

function CodingInteractionCardContent({ interaction, acting, onResolve, compact = false, availablePreviewPanelKey, attachedPreview, onViewPreview }: Props) {
  const [questionAnswers, setQuestionAnswers] = useState<Record<string, QuestionAnswerState>>({});
  const [followupMessage, setFollowupMessage] = useState('');
  const [currentQuestionIndex, setCurrentQuestionIndex] = useState(0);
  const [selectedFindingIDs, setSelectedFindingIDs] = useState<string[]>([]);
  const [approvalPreviewExpanded, setApprovalPreviewExpanded] = useState(false);

  const isBusy = acting !== null;
  const requestPayload = interaction.request_payload ?? {};

  if (interaction.interaction_kind === 'request_user_input') {
    const sharedQuestions = parseSharedUserInputQuestions(requestPayload);
    if (sharedQuestions.length > 0) {
      const allAnswered = sharedQuestions.every((question) => isSharedUserInputQuestionAnswered(question, questionAnswers[question.id]));
      const showStepper = compact && sharedQuestions.length > 1;
      const visibleQuestions = showStepper ? [sharedQuestions[Math.min(currentQuestionIndex, sharedQuestions.length - 1)]] : sharedQuestions;
      const currentQuestion = sharedQuestions[Math.min(currentQuestionIndex, sharedQuestions.length - 1)];
      const currentAnswered = currentQuestion ? isSharedUserInputQuestionAnswered(currentQuestion, questionAnswers[currentQuestion.id]) : false;
      const summary = dedupePromptSummary(interaction.summary, sharedQuestions.map((question) => question.question));

      return (
        <InteractionShell compact={compact}
          icon={<CheckmarkCircle02Icon className="h-4 w-4" />}
          eyebrow="Needs your input"
          title={interaction.title && interaction.title.toLowerCase() !== 'user input required' ? interaction.title : 'Answer to continue'}
          summary={summary}
        >
          {showStepper ? (
            <div className="mb-3 flex items-center justify-between text-[11px] text-muted-foreground">
              <span>Question {currentQuestionIndex + 1} of {sharedQuestions.length}</span>
              <span>{Object.values(questionAnswers).filter((answer) => Boolean(answer?.value || answer?.freetext?.trim())).length} answered</span>
            </div>
          ) : null}
          <div className="divide-y divide-border/60">
            {visibleQuestions.map((question) => {
              const answer = questionAnswers[question.id];
              const isSecret = question.isSecret;
              return (
                <div key={question.id} className="space-y-3 py-3 first:pt-0 last:pb-0" data-coding-session-question>
                  <div>
                    {question.header ? (
                      <div className="text-[11px] font-medium uppercase tracking-wide text-muted-foreground">{question.header}</div>
                    ) : null}
                    <MarkdownContent
                      content={question.question}
                      className={cn('mt-1 text-foreground', compact ? 'text-sm leading-5' : 'text-sm leading-6')}
                    />
                  </div>
                  {question.options.length > 0 ? (
                    <div className="space-y-1">
                      {question.options.map((option) => {
                        const selected = answer?.value === option.label;
                        return (
                          <InteractionOption
                            key={option.label}
                            selected={selected}
                            label={option.label}
                            description={option.description}
                            onClick={() => setQuestionAnswers((current) => ({
                              ...current,
                              [question.id]: { value: option.label },
                            }))}
                            disabled={isBusy}
                            compact={compact}
                          />
                        );
                      })}
                      {question.isOther ? (
                        <InteractionOption
                          selected={answer?.value === '__other__'}
                          label="Other"
                          description="Provide a custom reply."
                          onClick={() => setQuestionAnswers((current) => ({
                            ...current,
                            [question.id]: { ...current[question.id], value: '__other__' },
                          }))}
                          disabled={isBusy}
                          compact={compact}
                        />
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
            {showStepper && currentQuestionIndex < sharedQuestions.length - 1 ? (
              <Button
                size="sm"
                disabled={!currentAnswered || isBusy}
                onClick={() => setCurrentQuestionIndex((current) => Math.min(sharedQuestions.length - 1, current + 1))}
              >
                Next
              </Button>
            ) : (
              <Button
                size="sm"
                disabled={!allAnswered || isBusy}
                onClick={() => onResolve(interaction.interaction_id, buildSharedUserInputResponsePayload(sharedQuestions, questionAnswers))}
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
      const summary = dedupePromptSummary(interaction.summary, helpinQuestions.map((question) => question.text));

      return (
        <InteractionShell compact={compact}
          icon={<CheckmarkCircle02Icon className="h-4 w-4" />}
          eyebrow="Needs your input"
          title={interaction.title && interaction.title.toLowerCase() !== 'user input required' ? interaction.title : 'Answer to continue'}
          summary={summary}
        >
          {showStepper ? (
            <div className="mb-3 flex items-center justify-between text-[11px] text-muted-foreground">
              <span>Question {currentQuestionIndex + 1} of {helpinQuestions.length}</span>
              <span>{Object.values(questionAnswers).filter((answer) => Boolean(answer?.value || answer?.freetext?.trim())).length} answered</span>
            </div>
          ) : null}
          <div className="divide-y divide-border/60">
            {visibleQuestions.map((question) => {
              const answer = questionAnswers[question.id];
              const selectedOption = question.options.find((option) => option.value === answer?.value);
              return (
                <div key={question.id} className="space-y-3 py-3 first:pt-0 last:pb-0" data-coding-session-question>
                  <MarkdownContent
                    content={question.text}
                    className={cn('text-foreground', compact ? 'text-sm leading-5' : 'text-sm leading-6')}
                  />
                  <div className="space-y-1">
                    {question.options.map((option) => {
                      const selected = answer?.value === option.value;
                      return (
                        <InteractionOption
                          key={option.value}
                          selected={selected}
                          label={option.label}
                          onClick={() => setQuestionAnswers((current) => ({
                            ...current,
                            [question.id]: { value: option.value, freetext: option.freetext ? current[question.id]?.freetext : undefined },
                          }))}
                          disabled={isBusy}
                          compact={compact}
                        />
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
    const findings = checkpoint?.findings ?? [];
    const hasFindings = findings.length > 0;
    const selectedFindingSet = new Set(selectedFindingIDs);
    const allSelected = hasFindings && findings.every((finding) => selectedFindingSet.has(finding.id));
    const selectedCount = selectedFindingIDs.length;
    const buildReviewResponse = (
      decision: CodingSessionReviewCheckpointResponsePayload['decision'],
      selectionMode?: CodingSessionReviewCheckpointResponsePayload['selection_mode'],
    ): CodingSessionReviewCheckpointResponsePayload => ({
      decision,
      ...(followupMessage.trim() ? { message: followupMessage.trim() } : {}),
      ...(selectionMode ? { selection_mode: selectionMode } : {}),
      ...(selectionMode === 'selected' ? { selected_finding_ids: selectedFindingIDs } : {}),
      ...(selectionMode === 'none' ? { selected_finding_ids: [] } : {}),
    });

    return (
      <InteractionShell compact={compact}
        icon={<SecurityCheckIcon className="h-4 w-4" />}
        eyebrow={checkpoint?.phase ? `${checkpoint.phase} · needs your review` : 'Needs your review'}
        title={interaction.title && interaction.title.toLowerCase() !== 'review required'
          ? interaction.title
          : checkpoint?.title || 'Review findings to continue'}
        summary={interaction.summary ?? checkpoint?.summary}
      >
        {hasFindings ? (
          <div className="space-y-3">
            <ReviewCheckpointOverview checkpoint={checkpoint} compact={compact} />
            <div>
              <div className="flex items-center justify-between gap-3 border-b border-border/60 pb-2.5">
                <div>
                  <div className={cn('font-medium text-foreground', compact ? 'text-sm leading-5' : 'text-sm')}>Findings</div>
                  <div className="text-xs text-muted-foreground">
                    Select the findings that should move into the next step.
                  </div>
                </div>
                <label className="flex items-center gap-2 text-xs text-muted-foreground">
                  <Checkbox
                    checked={allSelected}
                    onCheckedChange={(checked) => {
                      setSelectedFindingIDs(checked ? findings.map((finding) => finding.id) : []);
                    }}
                    disabled={isBusy}
                  />
                  Select all
                </label>
              </div>
              <div className="divide-y divide-border/60">
                {findings.map((finding) => {
                  const selected = selectedFindingSet.has(finding.id);
                  return (
                    <label
                      key={finding.id}
                      className={cn(
                        '-mx-2 flex gap-3 px-2 py-3 transition-colors',
                        selected
                          ? 'bg-primary/[0.05] dark:bg-primary/[0.08]'
                          : 'hover:bg-muted/45',
                      )}
                      data-coding-session-review-finding
                    >
                      <Checkbox
                        checked={selected}
                        onCheckedChange={(checked) => {
                          setSelectedFindingIDs((current) => (
                            checked
                              ? [...new Set([...current, finding.id])]
                              : current.filter((id) => id !== finding.id)
                          ));
                        }}
                        disabled={isBusy}
                        className="mt-0.5"
                      />
                      <div className="min-w-0 flex-1 space-y-2">
                        <div className="flex flex-wrap items-center gap-2">
                          <div className={cn('font-medium text-foreground', compact ? 'text-sm leading-5' : 'text-sm')}>
                            {finding.title}
                          </div>
                          {finding.priority ? (
                            <Badge variant="outline" className={priorityBadgeClassName(finding.priority)}>
                              {finding.priority.toUpperCase()}
                            </Badge>
                          ) : null}
                          {finding.confidence ? (
                            <Badge variant="outline" className="text-[10px] uppercase tracking-wide">
                              {finding.confidence}
                            </Badge>
                          ) : null}
                        </div>
                        {finding.body ? (
                          <p className="text-sm leading-6 text-muted-foreground">{finding.body}</p>
                        ) : null}
                        <div className="flex flex-wrap items-center gap-2 text-[11px] text-muted-foreground">
                          <code>{finding.id}</code>
                          {finding.code_location ? (
                            <code>{finding.code_location}</code>
                          ) : null}
                        </div>
                      </div>
                    </label>
                  );
                })}
              </div>
            </div>
          </div>
        ) : null}
        <Textarea
          value={followupMessage}
          onChange={(event) => setFollowupMessage(event.target.value)}
          placeholder={hasFindings ? 'Optional note about the selected or skipped findings' : 'Optional note for the agent'}
          className={cn('min-h-[76px]', hasFindings && 'mt-4')}
          disabled={isBusy}
        />
        <div className="mt-4 flex flex-wrap gap-2">
          <Button
            size="sm"
            className={approveButtonClassName}
            disabled={isBusy || (hasFindings && selectedCount === 0)}
            onClick={() => onResolve(
              interaction.interaction_id,
              buildReviewResponse('approve', hasFindings ? 'selected' : undefined),
              followupMessage.trim() || undefined,
            )}
          >
            Approve
          </Button>
          {hasFindings ? (
            <Button
              variant="outline"
              size="sm"
              disabled={isBusy}
              onClick={() => onResolve(
                interaction.interaction_id,
                buildReviewResponse('skip', 'none'),
                followupMessage.trim() || undefined,
              )}
            >
              Skip
            </Button>
          ) : null}
          <Button
            variant="outline"
            size="sm"
            disabled={isBusy}
            onClick={() => onResolve(
              interaction.interaction_id,
              buildReviewResponse('request_changes', hasFindings ? (selectedCount > 0 ? 'selected' : 'all') : undefined),
              followupMessage.trim() || undefined,
            )}
          >
            Request changes
          </Button>
        </div>
      </InteractionShell>
    );
  }

  if (interaction.interaction_kind === 'approval_request') {
    const approval = parseApprovalRequest(requestPayload);
    // Runtimes (e.g. Codex) describe what they want to do in the interaction
    // summary — "Command: …", file/permission details, reason. Surface it so
    // the reviewer knows what they are approving instead of a bare title.
    // Only when there is no richer document/preview already representing the
    // request (task-plan/doc approvals carry a preview panel and intentionally
    // suppress the raw summary in favour of the rendered preview).
    const approvalContext = (interaction.summary ?? approval?.summary)?.trim() || undefined;
    const showApprovalContext = Boolean(approvalContext) && !approval?.preview_panel_key && !attachedPreview;
    const buildApprovalResponse = (
      decision: CodingSessionApprovalResponsePayload['decision'],
    ): CodingSessionApprovalResponsePayload => ({
      decision,
      ...(followupMessage.trim() ? { message: followupMessage.trim() } : {}),
    });

    const canViewPreview = Boolean(availablePreviewPanelKey && onViewPreview);
    const noteIsPresent = followupMessage.trim().length > 0;
    const fullPreviewAction = canViewPreview ? (
      <Button
        type="button"
        variant="ghost"
        size="sm"
        className="-mr-1 h-7 gap-1.5 px-2 text-xs text-muted-foreground hover:text-foreground"
        onClick={() => onViewPreview!(availablePreviewPanelKey!)}
      >
        <ExpandIcon className="h-3.5 w-3.5" />
        Open full preview
      </Button>
    ) : null;

    return (
      <InteractionShell compact={compact}
        icon={<SecurityCheckIcon className="h-4 w-4" />}
        eyebrow="Needs your approval"
        title={interaction.title ?? approval?.title ?? 'Approval required'}
        action={fullPreviewAction}
      >
        {showApprovalContext ? (
          <div className="mb-3 border-y border-border/60 py-2.5" data-coding-session-approval-context>
            <div className="mb-1 text-[11px] font-semibold uppercase tracking-wide text-muted-foreground">
              What the agent wants to do
            </div>
            <pre className="whitespace-pre-wrap break-words font-mono text-xs leading-5 text-foreground">
              {approvalContext}
            </pre>
          </div>
        ) : null}
        <ApprovalInlinePreview
          preview={attachedPreview}
          expanded={approvalPreviewExpanded}
          onExpandedChange={setApprovalPreviewExpanded}
        />
        <Textarea
          value={followupMessage}
          onChange={(event) => setFollowupMessage(event.target.value)}
          placeholder="Optional note sent with your decision"
          className={cn(
            'min-h-[76px] focus-visible:border-ring/70 focus-visible:ring-2 focus-visible:ring-ring/15',
            attachedPreview && 'mt-3',
          )}
          disabled={isBusy}
        />
        <div
          className="mt-5 flex flex-wrap gap-2 border-t border-border/60 pt-4"
          data-coding-session-approval-actions
        >
          <Button
            size="sm"
            className={approveButtonClassName}
            disabled={isBusy}
            onClick={() => onResolve(
              interaction.interaction_id,
              buildApprovalResponse('approve'),
              followupMessage.trim() || undefined,
            )}
          >
            {noteIsPresent ? 'Approve with note' : 'Approve'}
          </Button>
          <Button
            variant="outline"
            size="sm"
            disabled={isBusy}
            onClick={() => onResolve(
              interaction.interaction_id,
              buildApprovalResponse('request_changes'),
              followupMessage.trim() || undefined,
            )}
          >
            {noteIsPresent ? 'Request changes with note' : 'Request changes'}
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
        <PermissionsApprovalDetails
          reason={permissions?.reason ?? ''}
          permissions={requestedPermissions}
          compact={compact}
        />
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
          className="mt-4 min-h-[76px] focus-visible:border-ring/70 focus-visible:ring-2 focus-visible:ring-ring/15"
          disabled={isBusy}
        />
        <div className="mt-4 flex flex-wrap gap-2">
          {decisions.map((decision) => (
            <Button
              key={decision}
              size="sm"
              variant={decision.startsWith('accept') ? 'default' : 'outline'}
              className={runtimeDecisionButtonClassName(decision)}
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

function InteractionOption({
  selected,
  label,
  description,
  onClick,
  disabled,
  compact,
}: {
  selected: boolean;
  label: string;
  description?: string;
  onClick: () => void;
  disabled: boolean;
  compact?: boolean;
}) {
  return (
    <button
      type="button"
      aria-pressed={selected}
      onClick={onClick}
      disabled={disabled}
      className={cn(
        'group/option flex min-h-11 w-full items-start gap-2.5 rounded-md px-2.5 py-2 text-left outline-none transition-colors',
        'hover:bg-muted/60 focus-visible:ring-2 focus-visible:ring-ring/25 disabled:opacity-60',
        selected && 'bg-primary/[0.06] hover:bg-primary/[0.08] dark:bg-primary/[0.1]',
      )}
      data-coding-session-interaction-option
    >
      <span
        aria-hidden
        className={cn(
          'mt-1 grid h-3.5 w-3.5 shrink-0 place-items-center rounded-full border transition-colors',
          selected ? 'border-primary' : 'border-muted-foreground/45 group-hover/option:border-muted-foreground/70',
        )}
      >
        {selected ? <span className="h-1.5 w-1.5 rounded-full bg-primary" /> : null}
      </span>
      <span className="min-w-0 flex-1">
        <span className={cn('block font-medium text-foreground', compact ? 'text-sm leading-5' : 'text-sm')}>
          {label}
        </span>
        {description ? (
          <span className="mt-0.5 block text-xs leading-5 text-muted-foreground">{description}</span>
        ) : null}
      </span>
    </button>
  );
}

function InteractionShell({
  icon,
  eyebrow,
  title,
  summary,
  children,
  action,
  compact = false,
}: {
  icon?: ReactNode;
  eyebrow?: string;
  title: string;
  summary?: string;
  children: ReactNode;
  action?: ReactNode;
  compact?: boolean;
}) {
  return (
    <section
      aria-label={title}
      data-coding-session-interaction-shell
      className={cn(
        'border-y bg-transparent',
        compact
          ? 'border-amber-400/40 py-3 dark:border-amber-500/35'
          : 'border-border/70 py-4',
      )}
    >
      {eyebrow ? (
        <div className={cn('mb-1.5 flex items-center gap-2 font-medium uppercase tracking-wide text-amber-700 dark:text-amber-400', compact ? 'text-[10px]' : 'text-[11px]')}>
          {icon}
          {eyebrow}
        </div>
      ) : null}
      <div className="flex items-start justify-between gap-3">
        <div className={cn('min-w-0 font-semibold', compact ? 'text-sm' : 'text-base')}>{title}</div>
        {action ? <div className="shrink-0">{action}</div> : null}
      </div>
      {summary ? (
        <MarkdownContent
          content={summary}
          className={cn('mt-1 text-muted-foreground', compact ? 'text-sm leading-5' : 'text-sm leading-6')}
        />
      ) : null}
      <div className={compact ? 'mt-3' : 'mt-4'}>{children}</div>
    </section>
  );
}

function ApprovalInlinePreview({
  preview,
  expanded,
  onExpandedChange,
}: {
  preview?: PublishedPreview | null;
  expanded: boolean;
  onExpandedChange: (expanded: boolean) => void;
}) {
  const taskPlanPreview = parseApprovalTaskPlanPreview(preview);
  if (taskPlanPreview) {
    return (
      <div className="mb-3 rounded-lg border border-border/70 bg-muted/25">
        <div className="flex items-center justify-between gap-2 border-b border-border/60 px-3 py-2">
          <div className="flex min-w-0 items-center gap-2">
            <File01Icon className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
            <span className="truncate text-[11px] font-semibold uppercase tracking-wide text-muted-foreground">
              Document preview
            </span>
          </div>
        </div>
        <div className="space-y-2 px-3 py-2">
          {taskPlanPreview.summary ? (
            <MarkdownContent content={taskPlanPreview.summary} className="text-[12px] leading-5 text-foreground" />
          ) : null}
          <div className="space-y-1">
            {taskPlanPreview.tasks.map((task, index) => (
              <div key={`${task.ref ?? task.title}-${index}`} className="rounded-md bg-background/70 px-2.5 py-2">
                <div className="flex flex-wrap items-center gap-1.5">
                  {task.ref ? (
                    <Badge variant="outline" className="h-5 rounded-full px-1.5 text-[10px] text-muted-foreground">
                      {task.ref}
                    </Badge>
                  ) : null}
                  <span className="text-xs font-medium text-foreground">{task.title}</span>
                </div>
                {task.description ? (
                  <p className="mt-1 text-xs leading-5 text-muted-foreground">{task.description}</p>
                ) : null}
              </div>
            ))}
          </div>
        </div>
      </div>
    );
  }

  const previewContent = approvalPreviewContent(preview);
  if (!previewContent) return null;

  const isTruncated = previewContent.length > APPROVAL_PREVIEW_COLLAPSED_LENGTH;
  const visibleContent = !expanded && isTruncated
    ? `${previewContent.slice(0, APPROVAL_PREVIEW_COLLAPSED_LENGTH).trimEnd()}...`
    : previewContent;

  return (
    <div className="mb-3 rounded-lg border border-border/70 bg-muted/25">
      <div className="flex items-center justify-between gap-2 border-b border-border/60 px-3 py-2">
        <div className="flex min-w-0 items-center gap-2">
          <File01Icon className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
          <span className="truncate text-[11px] font-semibold uppercase tracking-wide text-muted-foreground">
            Document preview
          </span>
        </div>
      </div>
      <div className="px-3 py-2">
        <div
          className={cn(
            'relative',
            isTruncated && !expanded && 'overflow-hidden pb-4',
          )}
        >
          <MarkdownContent content={visibleContent} className="text-[12px] leading-5 text-foreground" />
          {isTruncated && !expanded ? (
            <div
              className="pointer-events-none absolute inset-x-0 bottom-0 h-10 bg-gradient-to-t from-card/95 via-card/70 to-transparent"
              data-coding-session-approval-preview-fade
            />
          ) : null}
        </div>
        {isTruncated ? (
          <div className="mt-3 border-t border-border/60 pt-2">
            <button
              type="button"
              className="inline-flex h-7 items-center gap-1.5 rounded-md border border-border bg-background px-2.5 text-xs font-medium text-foreground shadow-sm transition-colors hover:bg-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/30"
              onClick={() => onExpandedChange(!expanded)}
            >
              {expanded ? (
                <>
                  <ArrowUp02Icon className="h-3.5 w-3.5" />
                  Collapse preview
                </>
              ) : (
                <>
                  <ArrowDown02Icon className="h-3.5 w-3.5" />
                  Show full preview
                </>
              )}
            </button>
          </div>
        ) : null}
      </div>
    </div>
  );
}

function asRecord(value: unknown): Record<string, unknown> | null {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return null;
  return value as Record<string, unknown>;
}

function stringValue(value: unknown) {
  return typeof value === 'string' ? value.trim() : '';
}

function parseApprovalTaskPlanPreview(preview?: PublishedPreview | null) {
  if (!preview || preview.format !== 'json') return null;
  const record = asRecord(preview.content);
  if (!record) return null;
  const proposedTasks = Array.isArray(record.proposed_tasks)
    ? record.proposed_tasks
    : Array.isArray(record.proposed_stories)
      ? record.proposed_stories
      : [];
  const tasks = proposedTasks.flatMap((entry) => {
    const task = asRecord(entry);
    if (!task) return [];
    const title = stringValue(task.title) || stringValue(task.name);
    if (!title) return [];
    return [{
      ref: stringValue(task.ref) || undefined,
      title,
      description: stringValue(task.description) || undefined,
    }];
  });
  if (tasks.length === 0) return null;
  return {
    summary: stringValue(record.summary) || undefined,
    tasks,
  };
}

function approvalPreviewContent(preview?: PublishedPreview | null): string | null {
  if (!preview) return null;
  if (preview.format === 'markdown' && typeof preview.content === 'string') {
    return preview.content.trim() || null;
  }
  if (preview.format === 'json') {
    try {
      return JSON.stringify(preview.content, null, 2);
    } catch {
      return null;
    }
  }
  return null;
}

function normalizePromptText(value: string) {
  return value.replace(/\s+/g, ' ').trim();
}

function dedupePromptSummary(summary: string | undefined, prompts: string[]) {
  const normalizedSummary = normalizePromptText(summary ?? '');
  if (!normalizedSummary) return undefined;
  const duplicatesPrompt = prompts.some((prompt) => normalizePromptText(prompt) === normalizedSummary);
  return duplicatesPrompt ? undefined : summary;
}

function isSharedUserInputQuestionAnswered(
  question: ReturnType<typeof parseSharedUserInputQuestions>[number],
  answer?: QuestionAnswerState,
) {
  if (!answer) return false;
  if (!question.options.length || answer.value === '__other__') {
    return Boolean(answer.freetext?.trim());
  }
  return Boolean(answer.value?.trim());
}

function parseSharedUserInputQuestions(payload: Record<string, unknown>) {
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

function buildSharedUserInputResponsePayload(
  questions: ReturnType<typeof parseSharedUserInputQuestions>,
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
  const findings = Array.isArray(payload.findings) ? payload.findings : [];
  return {
    phase: typeof payload.phase === 'string' ? payload.phase.trim() : '',
    title: typeof payload.title === 'string' ? payload.title.trim() : '',
    summary: typeof payload.summary === 'string' ? payload.summary.trim() : '',
    findings: findings.flatMap((finding, index) => parseReviewFinding(finding, index)),
    overallCorrectness: typeof payload.overall_correctness === 'string' ? payload.overall_correctness.trim() : '',
    overallExplanation: typeof payload.overall_explanation === 'string' ? payload.overall_explanation.trim() : '',
    overallConfidenceScore: typeof payload.overall_confidence_score === 'number' && Number.isFinite(payload.overall_confidence_score)
      ? payload.overall_confidence_score
      : undefined,
  } satisfies CodingSessionReviewCheckpointRequestPayload & {
    findings: CodingSessionReviewFinding[];
    overallCorrectness?: string;
    overallExplanation?: string;
    overallConfidenceScore?: number;
  };
}

function parseApprovalRequest(payload: Record<string, unknown>) {
  if (!payload || typeof payload !== 'object') return null;
  return {
    phase: typeof payload.phase === 'string' ? payload.phase : undefined,
    preview_panel_key: typeof payload.preview_panel_key === 'string' ? payload.preview_panel_key : undefined,
    title: typeof payload.title === 'string' ? payload.title : undefined,
    summary: typeof payload.summary === 'string' ? payload.summary : undefined,
  } satisfies CodingSessionApprovalRequestPayload;
}

function parseReviewFinding(rawFinding: unknown, index: number): CodingSessionReviewFinding[] {
  if (!rawFinding || typeof rawFinding !== 'object' || Array.isArray(rawFinding)) return [];
  const finding = rawFinding as Record<string, unknown>;
  const id = typeof finding.id === 'string' ? finding.id.trim() : `finding_${index + 1}`;
  const title = typeof finding.title === 'string' ? finding.title.trim() : '';
  if (!id || !title) return [];
  return [{
    id,
    title,
    body: typeof finding.body === 'string' ? finding.body.trim() : '',
    priority: typeof finding.priority === 'string' ? finding.priority.trim() : '',
    confidence: formatConfidenceLabel(finding.confidence),
    code_location: typeof finding.code_location === 'string' ? finding.code_location.trim() : '',
  }];
}

function formatConfidenceLabel(value: unknown) {
  if (typeof value === 'string') return value.trim();
  if (typeof value === 'number' && Number.isFinite(value)) {
    if (value >= 0 && value <= 1) return `${Math.round(value * 100)}%`;
    return String(value);
  }
  return '';
}

function ReviewCheckpointOverview({
  checkpoint,
  compact,
}: {
  checkpoint: ReturnType<typeof parseReviewCheckpointRequest>;
  compact?: boolean;
}) {
  const badges = useMemo(() => {
    const items: ReactNode[] = [];
    if (checkpoint.overallCorrectness) {
      items.push(
        <Badge key="correctness" variant="outline" className="text-[10px] uppercase tracking-wide">
          {checkpoint.overallCorrectness.replaceAll('_', ' ')}
        </Badge>,
      );
    }
    if (typeof checkpoint.overallConfidenceScore === 'number') {
      items.push(
        <Badge key="confidence" variant="outline" className="text-[10px] uppercase tracking-wide">
          {Math.round(checkpoint.overallConfidenceScore * 100)}% confidence
        </Badge>,
      );
    }
    return items;
  }, [checkpoint.overallConfidenceScore, checkpoint.overallCorrectness]);

  if (!checkpoint.overallExplanation && badges.length === 0) return null;

  return (
    <div className="border-b border-border/60 pb-3" data-coding-session-review-overview>
      {badges.length > 0 ? (
        <div className="mb-2 flex flex-wrap gap-2">{badges}</div>
      ) : null}
      {checkpoint.overallExplanation ? (
        <p className={cn('text-muted-foreground', compact ? 'text-sm leading-5' : 'text-sm leading-6')}>
          {checkpoint.overallExplanation}
        </p>
      ) : null}
    </div>
  );
}

function priorityBadgeClassName(priority: string) {
  switch (priority.trim().toUpperCase()) {
    case 'P0':
      return 'border-red-200 bg-red-50 text-red-700 dark:border-red-900/60 dark:bg-red-950/30 dark:text-red-300';
    case 'P1':
      return 'border-orange-200 bg-orange-50 text-orange-700 dark:border-orange-900/60 dark:bg-orange-950/30 dark:text-orange-300';
    case 'P2':
      return 'border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-900/60 dark:bg-amber-950/30 dark:text-amber-300';
    default:
      return 'text-[10px] uppercase tracking-wide';
  }
}

function parsePermissionsRequest(payload: Record<string, unknown>) {
  return {
    reason: typeof payload.reason === 'string' ? payload.reason.trim() : '',
    permissions: payload.permissions && typeof payload.permissions === 'object' && !Array.isArray(payload.permissions)
      ? payload.permissions as Record<string, unknown>
      : {},
  };
}

function formatPermissionValue(value: unknown): string {
  if (value === true) return 'Allowed';
  if (value === false) return 'Not allowed';
  if (typeof value === 'string' && value.trim()) return value.trim();
  if (typeof value === 'number' && Number.isFinite(value)) return String(value);
  if (Array.isArray(value)) {
    return value.map((entry) => formatPermissionValue(entry)).join(', ');
  }
  if (value && typeof value === 'object') return 'Custom';
  return 'Not specified';
}

function PermissionsApprovalDetails({
  reason,
  permissions,
  compact,
}: {
  reason: string;
  permissions: Record<string, unknown>;
  compact?: boolean;
}) {
  const entries = Object.entries(permissions);
  return (
    <div className={cn('space-y-3 rounded-lg border border-border bg-muted/25 p-3', compact ? 'text-xs' : 'text-sm')}>
      {reason ? (
        <p className="leading-5 text-muted-foreground">{reason}</p>
      ) : null}
      {entries.length ? (
        <div className="space-y-1.5">
          {entries.map(([name, value]) => (
            <div key={name} className="flex items-center justify-between gap-3 rounded-md bg-background/70 px-2.5 py-2">
              <span className="min-w-0 truncate font-medium text-foreground">{name.replaceAll('_', ' ')}</span>
              <span className="shrink-0 text-xs text-muted-foreground">{formatPermissionValue(value)}</span>
            </div>
          ))}
        </div>
      ) : (
        <p className="text-muted-foreground">No additional permissions were listed.</p>
      )}
      <details>
        <summary className="cursor-pointer list-none text-xs font-medium text-muted-foreground hover:text-foreground">
          Raw details
        </summary>
        <pre className="mt-2 max-h-48 overflow-auto whitespace-pre-wrap rounded-lg border border-border bg-background px-3 py-2 text-[11px] leading-5 text-muted-foreground">
          {JSON.stringify(permissions, null, 2)}
        </pre>
      </details>
    </div>
  );
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
            'overflow-auto whitespace-pre-wrap break-all rounded-lg border border-border/60 bg-muted/50 px-3 py-2 font-mono text-[11px] leading-5 text-foreground/80',
            !expanded && isLong && 'max-h-[156px]',
            expanded && 'max-h-80',
          )}>
            {expanded || !isLong ? command : lines.slice(0, COMMAND_COLLAPSED_LINES).join('\n')}
          </pre>
          {isLong && !expanded && (
            <div className="pointer-events-none absolute inset-x-0 bottom-0 h-10 rounded-b-lg bg-gradient-to-t from-muted/80 to-transparent" />
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

function runtimeDecisionButtonClassName(decision: string) {
  if (decision.startsWith('accept')) {
    return 'bg-emerald-600 text-white hover:bg-emerald-700 hover:text-white';
  }
  if (decision === 'decline' || decision === 'cancel') {
    return 'border-destructive/30 text-destructive hover:bg-destructive/10 hover:text-destructive';
  }
  return undefined;
}
