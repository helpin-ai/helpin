import { useState, type FormEvent } from 'react';
import { toast } from 'sonner';
import { QuietPrimaryAction, QuietTextAction, QuietUnderlineInput } from '@/components/design-system/quiet';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { useCreateExternalAgent, usePreviewExternalAgent } from '@/hooks/queries/useExternalAgents';
import type { AgentCardSummary } from '@/lib/externalAgentTypes';
import { Loading01Icon } from '@/lib/icons';
import { isValidExternalAgentURL } from '@/lib/externalAgents';
import {
  ExternalAgentCardSummaryView,
  ExternalAgentDataNotice,
  ExternalAgentField,
  ExternalAgentTeamPicker,
} from './externalAgentParts';

type ExternalAgentAddDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  workspaceId: string;
};

export function ExternalAgentAddDialog(props: ExternalAgentAddDialogProps) {
  return props.open ? <ExternalAgentAddForm {...props} /> : null;
}

function ExternalAgentAddForm({ onOpenChange, workspaceId }: ExternalAgentAddDialogProps) {
  const preview = usePreviewExternalAgent(workspaceId);
  const create = useCreateExternalAgent(workspaceId);
  const [cardURL, setCardURL] = useState('');
  const [token, setToken] = useState('');
  const [card, setCard] = useState<AgentCardSummary | null>(null);
  const [teamIds, setTeamIds] = useState<string[]>([]);
  const [error, setError] = useState<string | null>(null);
  const busy = preview.isPending || create.isPending;
  const urlValid = isValidExternalAgentURL(cardURL);
  const canCheck = urlValid && token.trim().length > 0 && !busy;

  const close = (next: boolean) => {
    if (!busy) onOpenChange(next);
  };

  const checkAgent = async () => {
    if (!canCheck) return;
    setError(null);
    try {
      setCard(await preview.mutateAsync({ card_url: cardURL.trim(), token: token.trim() }));
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Could not reach the agent');
    }
  };

  const addAgent = async () => {
    if (!card || busy) return;
    setError(null);
    try {
      const created = await create.mutateAsync({
        card_url: cardURL.trim(),
        token: token.trim(),
        ...(teamIds.length > 0 ? { allowed_team_ids: teamIds } : {}),
      });
      toast.success(`${created.name || card.name} added`);
      onOpenChange(false);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Could not add the agent');
    }
  };

  const submit = (event: FormEvent) => {
    event.preventDefault();
    void (card ? addAgent() : checkAgent());
  };

  return (
    <Dialog open onOpenChange={close}>
      <DialogContent className="max-h-[90dvh] overflow-y-auto sm:max-w-xl">
        <DialogHeader className="border-b border-quiet-divider-strong pb-3">
          <DialogTitle className="text-[20px] font-semibold tracking-[-0.018em]">
            {card ? `Add ${card.name || 'external agent'}` : 'Add external agent'}
          </DialogTitle>
          <DialogDescription className="text-[12.5px] text-quiet-text-tertiary">
            {card ? 'Step 2 of 2 · Review the agent and choose who can use it' : 'Step 1 of 2 · Connect to the agent'}
          </DialogDescription>
        </DialogHeader>
        <form className="space-y-5" onSubmit={submit} data-external-agent-step={card ? 'review' : 'connect'}>
          {card ? (
            <>
              <ExternalAgentCardSummaryView card={card} />
              <ExternalAgentTeamPicker workspaceId={workspaceId} value={teamIds} onChange={setTeamIds} disabled={busy} />
            </>
          ) : (
            <>
              <ExternalAgentField label="Agent URL" id="external-agent-url">
                <QuietUnderlineInput
                  id="external-agent-url"
                  type="url"
                  inputMode="url"
                  autoComplete="off"
                  autoFocus
                  value={cardURL}
                  onChange={(event) => { setCardURL(event.target.value); setError(null); }}
                  placeholder="https://agent.example.com"
                  aria-describedby="external-agent-url-hint"
                  disabled={busy}
                />
                <p id="external-agent-url-hint" className="text-[12px] text-quiet-muted">
                  The agent's base URL or its Agent Card URL (…/.well-known/agent-card.json).
                </p>
              </ExternalAgentField>
              <ExternalAgentField label="Access token" id="external-agent-token">
                <QuietUnderlineInput
                  id="external-agent-token"
                  type="password"
                  autoComplete="new-password"
                  value={token}
                  onChange={(event) => { setToken(event.target.value); setError(null); }}
                  placeholder="Bearer token"
                  className="font-mono"
                  disabled={busy}
                />
              </ExternalAgentField>
            </>
          )}
          <ExternalAgentDataNotice />
          {error ? <p role="alert" className="text-[12.5px] text-destructive">{error}</p> : null}
          <DialogFooter className="border-t border-quiet-divider-strong pt-3 sm:items-center">
            {card ? (
              <QuietTextAction type="button" className="sm:mr-auto" disabled={busy} onClick={() => { setCard(null); setError(null); }}>
                Back
              </QuietTextAction>
            ) : null}
            <QuietTextAction type="button" disabled={busy} onClick={() => onOpenChange(false)}>Cancel</QuietTextAction>
            {card ? (
              <QuietPrimaryAction type="submit" disabled={busy}>
                {create.isPending ? <Loading01Icon className="h-3.5 w-3.5 animate-spin" /> : null}
                {create.isPending ? 'Adding…' : 'Add agent'}
              </QuietPrimaryAction>
            ) : (
              <QuietPrimaryAction type="submit" disabled={!canCheck}>
                {preview.isPending ? <Loading01Icon className="h-3.5 w-3.5 animate-spin" /> : null}
                {preview.isPending ? 'Checking…' : 'Check agent'}
              </QuietPrimaryAction>
            )}
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
