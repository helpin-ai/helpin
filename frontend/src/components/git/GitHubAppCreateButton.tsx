import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { useCreateGitHubAppManifest, useGitHubAppStatus, useWorkspaceAccess } from '@/hooks/queries';
import { Loading01Icon, PlusSignIcon } from '@/lib/icons';

export type GitHubAppCreateButtonProps = {
  workspaceId?: string;
  /** GitHub organization login that should own the App; the signed-in GitHub user owns it when omitted. */
  organization?: string;
  variant?: 'default' | 'outline';
  size?: 'sm' | 'default';
  className?: string;
};

/**
 * Sends the browser to GitHub with the App manifest. The manifest flow only
 * accepts a top-level form POST with a `manifest` field, so fetch cannot be used.
 */
export function submitGitHubAppManifest(postURL: string, manifest: Record<string, unknown>, doc: Document = document) {
  const form = doc.createElement('form');
  form.method = 'post';
  form.action = postURL;
  form.style.display = 'none';
  const input = doc.createElement('input');
  input.type = 'hidden';
  input.name = 'manifest';
  input.value = JSON.stringify(manifest);
  form.appendChild(input);
  doc.body.appendChild(form);
  form.submit();
}

/**
 * "Create GitHub App" for Community owners when no instance GitHub App is
 * configured. Renders nothing otherwise.
 */
export function GitHubAppCreateButton({
  workspaceId,
  organization,
  variant = 'default',
  size = 'sm',
  className,
}: GitHubAppCreateButtonProps) {
  const { data: status } = useGitHubAppStatus(workspaceId);
  const { data: access } = useWorkspaceAccess(workspaceId ?? '');
  const createManifest = useCreateGitHubAppManifest(workspaceId);
  const isOwner = access?.membership?.role === 'owner';

  if (!workspaceId || !status || status.configured || !status.manifest_available || !isOwner) {
    return null;
  }

  const start = async () => {
    try {
      const result = await createManifest.mutateAsync(organization?.trim() || undefined);
      submitGitHubAppManifest(result.post_url, result.manifest);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Could not prepare the GitHub App');
    }
  };

  return (
    <Button
      type="button"
      variant={variant}
      size={size}
      className={className}
      disabled={createManifest.isPending}
      onClick={() => void start()}
    >
      {createManifest.isPending
        ? <Loading01Icon className="mr-1.5 h-3.5 w-3.5 animate-spin" />
        : <PlusSignIcon className="mr-1.5 h-3.5 w-3.5" />}
      Create GitHub App
    </Button>
  );
}
