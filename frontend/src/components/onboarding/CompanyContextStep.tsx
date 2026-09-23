import { useId, useRef, useState, type FormEvent } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Textarea } from '@/components/ui/textarea';
import { AiMagicIcon, Loading01Icon } from '@/lib/icons';
import { queryKeys } from '@/lib/queryKeys';
import { workspacesService } from '@/lib/services/workspacesService';
import type { Workspace } from '@/lib/types';
import { describeContextGenerationError, websiteDisplayName } from '@/lib/workspaceOnboardingFlow';
import { OnboardingActions, OnboardingTextButton } from './OnboardingShell';

type CompanyContextStepProps = {
  workspace: Workspace;
  /** Suggested website, for example from a company email domain. */
  defaultWebsiteUrl?: string;
  /** Whether the workspace has AI that can read the website. */
  canGenerate: boolean;
  onContinue: () => void;
};

type Generation = { state: 'idle' } | { state: 'running'; host: string } | { state: 'failed'; message: string };

/**
 * Optional company/product context for agents. It can be drafted from the
 * company website with the workspace's AI, written by hand, or skipped.
 */
export function CompanyContextStep({ workspace, defaultWebsiteUrl = '', canGenerate, onContinue }: CompanyContextStepProps) {
  const id = useId();
  const queryClient = useQueryClient();
  const [websiteUrl, setWebsiteUrl] = useState(workspace.website_url || defaultWebsiteUrl);
  const [context, setContext] = useState(workspace.company_product_context ?? '');
  const [generation, setGeneration] = useState<Generation>({ state: 'idle' });
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);
  const requestRef = useRef(0);
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const generating = generation.state === 'running';

  const generate = async () => {
    const url = websiteUrl.trim();
    if (!url) {
      setGeneration({ state: 'failed', message: 'Enter your website first.' });
      return;
    }
    const request = ++requestRef.current;
    setGeneration({ state: 'running', host: websiteDisplayName(url) });
    const response = await workspacesService.generateCompanyProductDescription({
      website_url: url,
      workspace_id: workspace.id,
      workspace_name: workspace.name,
    });
    // A newer request, Cancel, or "Write it myself" makes this answer stale.
    if (request !== requestRef.current) return;
    const text = response.data?.company_product_context || response.data?.description;
    if (text) {
      setContext(text);
      setGeneration({ state: 'idle' });
      return;
    }
    setGeneration({ state: 'failed', message: describeContextGenerationError(response) });
  };

  const stopGenerating = ({ focusTextarea }: { focusTextarea: boolean }) => {
    requestRef.current += 1;
    setGeneration({ state: 'idle' });
    if (focusTextarea) {
      window.setTimeout(() => textareaRef.current?.focus(), 0);
    }
  };

  const save = async (event: FormEvent) => {
    event.preventDefault();
    const trimmedContext = context.trim();
    const trimmedUrl = websiteUrl.trim();
    if (!trimmedContext && !trimmedUrl) {
      onContinue();
      return;
    }
    setSaving(true);
    setSaveError(null);
    const { data, error } = await workspacesService.update(workspace.id, {
      ...(trimmedContext ? { company_product_context: trimmedContext } : {}),
      ...(trimmedUrl ? { website_url: trimmedUrl } : {}),
    });
    setSaving(false);
    if (error || !data) {
      setSaveError(error ?? 'The context couldn’t be saved. Try again, or skip for now.');
      return;
    }
    queryClient.setQueryData(queryKeys.workspaces.bySlug(workspace.slug), data);
    void queryClient.invalidateQueries({ queryKey: ['workspaces'] });
    onContinue();
  };

  return (
    <form onSubmit={(event) => void save(event)} className="space-y-7" aria-busy={generating}>
      <div className="space-y-2">
        <Label htmlFor={`${id}-website`}>Company website <span className="font-normal text-muted-foreground">(optional)</span></Label>
        <div className="flex flex-col gap-3 sm:flex-row sm:items-end">
          <Input
            id={`${id}-website`}
            type="url"
            inputMode="url"
            placeholder="https://acme.com"
            value={websiteUrl}
            disabled={generating}
            onChange={(event) => setWebsiteUrl(event.target.value)}
            className="min-w-0 flex-1"
          />
          <Button
            type="button"
            variant="outline"
            className="shrink-0"
            disabled={!canGenerate || generating || !websiteUrl.trim()}
            onClick={() => void generate()}
          >
            <AiMagicIcon className="mr-2 h-4 w-4" aria-hidden="true" />
            Generate from website
          </Button>
        </div>
        {!canGenerate && (
          <p className="text-[12.5px] leading-5 text-muted-foreground">
            Generating needs AI. Connect a provider in Settings → AI, or write it yourself.
          </p>
        )}
      </div>

      <div role="status" aria-live="polite" className="empty:hidden">
        {generation.state === 'running' && (
          <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-sm">
            <span className="flex items-center gap-2 text-muted-foreground">
              <Loading01Icon className="h-4 w-4 animate-spin" aria-hidden="true" />
              Reading {generation.host}. This can take up to a minute.
            </span>
            <span className="flex gap-3">
              <OnboardingTextButton onClick={() => stopGenerating({ focusTextarea: false })}>Cancel</OnboardingTextButton>
              <OnboardingTextButton onClick={() => stopGenerating({ focusTextarea: true })}>Write it myself</OnboardingTextButton>
            </span>
          </div>
        )}
        {generation.state === 'failed' && (
          <p className="text-sm leading-relaxed text-destructive">{generation.message}</p>
        )}
      </div>

      <div className="space-y-2">
        <Label htmlFor={`${id}-context`}>What your company does</Label>
        <Textarea
          ref={textareaRef}
          id={`${id}-context`}
          value={context}
          onChange={(event) => setContext(event.target.value)}
          placeholder="What you make, who it’s for, and the problems it solves."
          rows={8}
          className="max-h-[min(40vh,360px)] min-h-[160px] resize-y"
        />
        <p className="text-[12.5px] leading-5 text-muted-foreground">Agents use this to understand your product. You can change it later in Settings → Knowledge.</p>
      </div>

      <p role="alert" className="text-sm text-destructive empty:hidden">{saveError ?? ''}</p>
      <OnboardingActions>
        <OnboardingTextButton onClick={onContinue} disabled={saving}>Skip</OnboardingTextButton>
        <Button type="submit" className="w-full sm:w-auto sm:min-w-32" disabled={saving || generating}>
          {saving && <Loading01Icon className="mr-2 h-4 w-4 animate-spin" aria-hidden="true" />}
          {saving ? 'Saving…' : 'Save and continue'}
        </Button>
      </OnboardingActions>
    </form>
  );
}
