import { useCallback, useEffect, useRef, useState } from 'react';
import { toast } from 'sonner';
import { useQueryClient } from '@tanstack/react-query';
import { CreateTaskModal } from '@/components/pm/CreateTaskModal';
import { TriageSuggestions } from '@/components/pm/TriageSuggestions';
import { getVisibleTriageSuggestions } from '@/components/pm/triageSuggestionRows';
import type { PreparedTaskDraft } from '@/components/pm/preparedTaskDraft';
import { Dialog } from '@/components/ui/dialog';
import { CreateEntityDialogContent } from '@/components/pm/CreateEntityModalLayout';
import { QuietTextAction } from '@/components/design-system/quiet';
import { SupportTaskPreparation } from './SupportTaskPreparation';
import { useConversation, useInboxScopes, useCreateTaskFromConversation } from '@/hooks/queries/useSupport';
import { useWorkspaceAccess, useUpdateSupportTaskPreferences } from '@/hooks/queries/useSession';
import { useAccessibleTeams } from '@/hooks/useAccessibleTeams';
import { pmTriageService, type TriageReview, type TriageView } from '@/lib/services/pmTriageService';
import { unwrapRequired } from '@/lib/queryUtils';
import { queryKeys } from '@/lib/queryKeys';
import type { CreateTaskRequest } from '@/lib/pmTypes';
import { UpgradeRequiredDialog } from '@edition';
import { getUpgradeRequiredReason, type UpgradeRequiredReason } from '@edition/errors';

interface Props {
  workspaceId: string;
  workspaceSlug: string;
  conversationId: string;
  onOpenChange: (open: boolean) => void;
}

export function SupportCreateTaskModal({ workspaceId, workspaceSlug, conversationId, onOpenChange }: Props) {
  const { data: conversation, isError: conversationError } = useConversation(workspaceId, conversationId);
  const { data: scopes, isError: scopesError } = useInboxScopes(workspaceId);
  const { data: access, isError: accessError } = useWorkspaceAccess(workspaceId);
  const { teams, loading: teamsLoading } = useAccessibleTeams(workspaceId);
  const inboxId = conversation?.mailbox_id ?? 'shared';
  const linkedTeam = [scopes?.shared_inbox, ...(scopes?.mailboxes ?? [])].find(
    inbox => inbox && (inbox.id === inboxId || (inboxId === 'shared' && inbox.is_shared)),
  )?.linked_team_id;
  const initialTeam = [
    linkedTeam,
    access?.membership.support_task_team_by_inbox?.[inboxId],
    access?.membership.support_default_team_id,
    teams[0]?.id,
  ].find(id => teams.some(team => team.id === id));
  const settingsLoading = !conversation || !scopes || !access || teamsLoading;
  let settingsError = '';
  if (settingsLoading || !teams.length) {
    if (conversationError || scopesError || accessError) {
      settingsError = 'Couldn’t load conversation settings. Close and try again.';
    } else if (!teamsLoading && access && !teams.length) {
      settingsError = 'No accessible team is available. Ask an admin to add you to a team.';
    }
  } else if (linkedTeam && !teams.some(team => team.id === linkedTeam)) {
    settingsError = 'This inbox’s team is unavailable to you. Ask an admin for access.';
  }
  if (settingsLoading || settingsError) return (
    <Dialog open onOpenChange={onOpenChange}>
      <CreateEntityDialogContent className="w-[480px]">
        <SupportTaskPreparation phase={settingsError ? 'blocked' : 'loading'} error={settingsError}
          onCancel={() => onOpenChange(false)} />
      </CreateEntityDialogContent>
    </Dialog>
  );
  return (
    <SupportCreateTaskForm key={`${workspaceId}:${conversationId}`} workspaceId={workspaceId}
      workspaceSlug={workspaceSlug} conversationId={conversationId} inboxId={inboxId}
      initialTeamId={initialTeam ?? undefined} onOpenChange={onOpenChange} />
  );
}

function SupportCreateTaskForm({ workspaceId, workspaceSlug, conversationId, inboxId, initialTeamId, onOpenChange }: Props & {
  inboxId: string;
  initialTeamId?: string;
}) {
  // Background settings refetches must never reset an open form.
  const [teamDefault] = useState(initialTeamId);
  const [phase, setPhase] = useState<'matching' | 'matches' | 'draft' | 'failed'>('matching');
  const [view, setView] = useState<TriageView | null>(null);
  const [matchRetry, setMatchRetry] = useState(0);
  const [notice, setNotice] = useState('');
  const [reviewing, setReviewing] = useState(false);
  const [taskCreated, setTaskCreated] = useState(false);
  const [prepared, setPrepared] = useState<{ teamId: string; draft: PreparedTaskDraft } | null>(null);
  const [upgradeReason, setUpgradeReason] = useState<UpgradeRequiredReason | null>(null);
  const queryClient = useQueryClient();
  const create = useCreateTaskFromConversation(workspaceId);
  const preferences = useUpdateSupportTaskPreferences(workspaceId);
  const mounted = useRef(true);
  useEffect(() => {
    mounted.current = true;
    return () => { mounted.current = false; };
  }, []);
  useEffect(() => {
    let cancelled = false;
    void pmTriageService.analyze(workspaceId, 'support_conversation', conversationId).then(response => {
      if (cancelled) return;
      const result = unwrapRequired(response, 'Task matching');
      setView(result);
      if (result.status === 'ready' || result.status === 'disabled' || result.status === 'shadow') {
        setPhase(result.status === 'ready' && getVisibleTriageSuggestions(result).length ? 'matches' : 'draft');
      } else {
        setNotice('Couldn’t check existing tasks. You can retry or create a new task.');
        setPhase('failed');
      }
    }).catch(error => {
      if (cancelled) return;
      const reason = getUpgradeRequiredReason(error);
      if (reason) setUpgradeReason(reason);
      setNotice('Couldn’t check existing tasks. You can retry or create a new task.');
      setPhase('failed');
    });
    return () => { cancelled = true; };
  }, [workspaceId, conversationId, matchRetry]);

  const prepareDraft = useCallback(async (teamId: string) => (
    unwrapRequired(await pmTriageService.draft(workspaceId, conversationId, teamId), 'Task draft')
  ), [workspaceId, conversationId]);
  const onPrepared = useCallback((draft: PreparedTaskDraft, teamId: string) => setPrepared({ draft, teamId }), []);
  const onCreate = async (payload: CreateTaskRequest) => {
    if (taskCreated) throw new Error('This task has already been created.');
    if (!prepared?.draft.source_hash || prepared.teamId !== payload.team_id) {
      throw new Error('Wait for the draft for this team to finish.');
    }
    const result = await create.mutateAsync({
      conversationId,
      teamId: payload.team_id!,
      draft: { ...payload, reviewed_draft: true, source_hash: prepared.draft.source_hash },
    });
    setTaskCreated(true);
    // Remember this inbox without holding up the created-task confirmation.
    preferences.mutate({ support_default_team_id: payload.team_id, support_task_inbox_id: inboxId }, {
      onError: () => toast.error('Task created, but the inbox team preference could not be saved.'),
    });
    return {
      id: result.task_id,
      task: { id: result.task_id, name: result.task_name, display_id: result.display_id, task_key: result.task_key },
      agent_run_error: result.agent_run_error,
    };
  };
  const review = async (request: TriageReview) => {
    if (!view || reviewing) return;
    setReviewing(true);
    setNotice('');
    try {
      const result = unwrapRequired(await pmTriageService.review(workspaceId, 'support_conversation', conversationId, request), 'Task suggestion');
      if (!mounted.current) return;
      if (request.dismiss) {
        const next = { ...view, reviewed: { ...view.reviewed, [result.key]: result.status } };
        setView(next);
        if (!getVisibleTriageSuggestions(next).length) setPhase('draft');
      } else {
        void queryClient.invalidateQueries({ queryKey: queryKeys.support.conversation(workspaceId, conversationId) });
        void queryClient.invalidateQueries({ queryKey: queryKeys.support.conversationAssociations(workspaceId, conversationId) });
        toast.success('Conversation linked to existing task');
        onOpenChange(false);
      }
    } catch (error) {
      if (mounted.current) setNotice(error instanceof Error ? error.message : 'Couldn’t save this choice.');
    } finally {
      if (mounted.current) setReviewing(false);
    }
  };
  const hasMatches = !!view && getVisibleTriageSuggestions(view).length > 0;
  const content = phase === 'draft' && !notice && !reviewing && !hasMatches ? null : (
    <div className={prepared ? 'px-6 py-3 text-sm' : 'mt-4 text-sm'}>
      {phase === 'matching' && <p role="status" className="text-muted-foreground">Checking for existing tasks…</p>}
      {notice && (prepared || phase !== 'failed') && <p role="alert" className="text-muted-foreground">{notice}</p>}
      {reviewing && <p role="status" className="text-muted-foreground">Saving your choice…</p>}
      {phase === 'matches' && view && (
        <TriageSuggestions view={view} disabled={reviewing} showComparisonSummary={false}
          taskHref={id => `/w/${encodeURIComponent(workspaceSlug)}/pm/tasks/${encodeURIComponent(id)}`}
          onReview={request => void review(request)} />
      )}
      {(phase === 'matches' || phase === 'failed') && (
        <div className="mt-2 flex gap-4">
          <QuietTextAction disabled={reviewing} onClick={() => { setNotice(''); setPhase('draft'); }}>Create new task</QuietTextAction>
          <QuietTextAction disabled={reviewing} onClick={() => { setNotice(''); setPhase('matching'); setMatchRetry(n => n + 1); }}>Refresh matches</QuietTextAction>
        </div>
      )}
      {phase === 'draft' && hasMatches && (
        <div className="flex items-center gap-4 text-xs text-muted-foreground">
          <QuietTextAction onClick={() => setPhase('matches')}>View matches</QuietTextAction>
        </div>
      )}
    </div>
  );
  return <>
    <CreateTaskModal open workspaceId={workspaceId} initialTeamId={teamDefault}
      onOpenChange={open => { if (!reviewing) onOpenChange(open); }} onCreate={onCreate}
      renderPreparation={prepared ? undefined : ({ error, retry, cancel }) => (
        <SupportTaskPreparation phase={phase}
          error={error} checkedExistingTasks={view?.status === 'ready'} reviewing={reviewing} onRetry={retry} onCancel={cancel}>
          {(phase === 'matches' || phase === 'failed') && content}
        </SupportTaskPreparation>
      )}
      supportSource preparationContent={content} prepareDraft={phase === 'draft' && !taskCreated ? prepareDraft : undefined}
      onDraftPrepared={onPrepared} creationBlocked={phase !== 'draft' || !prepared || taskCreated} />
    <UpgradeRequiredDialog open={upgradeReason !== null} onOpenChange={open => { if (!open) setUpgradeReason(null); }} reason={upgradeReason} onUpgrade={() => onOpenChange(false)} />
  </>;
}
