import { useId, useState, type FormEvent } from 'react';
import { toast } from 'sonner';
import { QuietPrimaryAction, QuietTextAction, QuietUnderlineInput } from '@/components/design-system/quiet';
import { Button } from '@/components/ui/button';
import { useCreateGitHubAppManifest, useGitHubAppStatus, useWorkspaceAccess } from '@/hooks/queries';
import { GITHUB_LOGIN_PATTERN } from '@/lib/githubReturn';
import { Loading01Icon, PlusSignIcon } from '@/lib/icons';
import type { GitHubReturnTo } from '@/lib/pmTypes';
import { cn } from '@/lib/utils';
import { GITHUB_APP_OWNER_HINT, submitGitHubAppManifest } from './githubApp';

export type GitHubAppCreateButtonProps = {
  workspaceId?: string;
  /** Prefills the GitHub organization that should own the App. */
  organization?: string;
  /** Helpin page GitHub returns to after the App is created and installed. */
  returnTo?: GitHubReturnTo;
  variant?: 'default' | 'outline';
  size?: 'sm' | 'default';
  className?: string;
};

type AppOwner = 'organization' | 'personal';

/**
 * "Create GitHub App" for Community owners when no instance GitHub App is
 * configured. It first asks who should own the App, then hands over to
 * GitHub's own confirmation page. Renders the reachability problem instead
 * when GitHub could not reach this server, and nothing otherwise.
 */
export function GitHubAppCreateButton({
  workspaceId,
  organization,
  returnTo = 'settings',
  variant = 'default',
  size = 'sm',
  className,
}: GitHubAppCreateButtonProps) {
  const { data: status } = useGitHubAppStatus(workspaceId);
  const { data: access } = useWorkspaceAccess(workspaceId ?? '');
  const createManifest = useCreateGitHubAppManifest(workspaceId);
  const [open, setOpen] = useState(false);
  const panelId = useId();
  const isOwner = access?.membership?.role === 'owner';

  if (!workspaceId || !status || status.configured || !isOwner) {
    return null;
  }
  if (status.manifest_blocked_reason) {
    return <p className={cn('max-w-xl text-[12.5px] leading-5 text-quiet-text-tertiary', className)}>{status.manifest_blocked_reason}</p>;
  }
  if (!status.manifest_available) {
    return null;
  }

  const start = async (owner: string | undefined) => {
    try {
      const result = await createManifest.mutateAsync({ organization: owner, return_to: returnTo });
      submitGitHubAppManifest(result.post_url, result.manifest);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Could not prepare the GitHub App');
    }
  };

  return (
    <div className={cn('min-w-0', className)}>
      <Button
        type="button"
        variant={variant}
        size={size}
        aria-expanded={open}
        aria-controls={panelId}
        onClick={() => setOpen((value) => !value)}
      >
        <PlusSignIcon className="mr-1.5 h-3.5 w-3.5" aria-hidden="true" />
        Create GitHub App
      </Button>
      <div id={panelId} hidden={!open}>
        {open && (
          <GitHubAppOwnerForm
            defaultOrganization={organization}
            pending={createManifest.isPending}
            onCancel={() => setOpen(false)}
            onContinue={(owner) => void start(owner)}
          />
        )}
      </div>
    </div>
  );
}

function GitHubAppOwnerForm({
  defaultOrganization,
  pending,
  onCancel,
  onContinue,
}: {
  defaultOrganization?: string;
  pending: boolean;
  onCancel: () => void;
  onContinue: (organization: string | undefined) => void;
}) {
  const [owner, setOwner] = useState<AppOwner>('organization');
  const [login, setLogin] = useState(defaultOrganization?.trim() ?? '');
  const [error, setError] = useState<string | null>(null);
  const id = useId();
  const loginId = `${id}-login`;
  const hintId = `${id}-hint`;
  const errorId = `${id}-error`;

  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (owner === 'personal') {
      onContinue(undefined);
      return;
    }
    const value = login.trim();
    if (!GITHUB_LOGIN_PATTERN.test(value)) {
      setError(value ? 'Use the organization’s GitHub login: letters, numbers and hyphens, not starting with a hyphen.' : 'Enter the GitHub organization login.');
      return;
    }
    setError(null);
    onContinue(value);
  };

  const radioClassName = 'mt-0.5 h-3.5 w-3.5 shrink-0 accent-quiet-text-primary';
  return (
    <form className="mt-3 max-w-md space-y-3 border-l border-quiet-divider-strong pl-4" onSubmit={submit} noValidate>
      <fieldset className="space-y-2" aria-describedby={hintId}>
        <legend className="text-sm font-medium text-quiet-text-primary">Who should own the App?</legend>
        <label className="flex items-start gap-2 text-[12.5px] text-quiet-text-secondary">
          <input
            type="radio"
            name={`${id}-owner`}
            value="organization"
            checked={owner === 'organization'}
            onChange={() => setOwner('organization')}
            className={radioClassName}
          />
          <span>
            <span className="text-quiet-text-primary">A GitHub organization</span>
            <span className="ml-1.5 text-[11.5px] font-semibold uppercase tracking-[0.03em] text-quiet-positive">Recommended</span>
          </span>
        </label>
        {owner === 'organization' && (
          <div className="pl-5">
            <label htmlFor={loginId} className="block text-[12px] text-quiet-text-tertiary">Organization login</label>
            <QuietUnderlineInput
              id={loginId}
              value={login}
              onChange={(event) => {
                setLogin(event.target.value);
                if (error) setError(null);
              }}
              placeholder="acme-inc"
              autoComplete="off"
              spellCheck={false}
              aria-invalid={error ? true : undefined}
              aria-describedby={error ? errorId : undefined}
              className="max-w-64"
            />
            {error && <p id={errorId} role="alert" className="mt-1 text-[12px] text-quiet-accent">{error}</p>}
          </div>
        )}
        <label className="flex items-start gap-2 text-[12.5px] text-quiet-text-primary">
          <input
            type="radio"
            name={`${id}-owner`}
            value="personal"
            checked={owner === 'personal'}
            onChange={() => setOwner('personal')}
            className={radioClassName}
          />
          My personal GitHub account
        </label>
      </fieldset>
      <p id={hintId} className="text-[12px] leading-5 text-quiet-text-tertiary">{GITHUB_APP_OWNER_HINT}</p>
      <div className="flex items-center gap-4">
        <QuietPrimaryAction type="submit" disabled={pending}>
          {pending && <Loading01Icon className="mr-1.5 h-3.5 w-3.5 animate-spin" aria-hidden="true" />}
          Continue to GitHub
        </QuietPrimaryAction>
        <QuietTextAction type="button" onClick={onCancel} disabled={pending}>Cancel</QuietTextAction>
      </div>
    </form>
  );
}
