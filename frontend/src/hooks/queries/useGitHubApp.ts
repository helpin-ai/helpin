import { useMutation, useQuery } from '@tanstack/react-query';
import type { GitHubAppManifestRequest } from '@/lib/pmTypes';
import { queryKeys } from '@/lib/queryKeys';
import { unwrap } from '@/lib/queryUtils';
import { gitService } from '@/lib/services/gitService';

/** Status of the instance GitHub App (source, slug, owner, whether it can be created here). */
export function useGitHubAppStatus(
  workspaceId?: string,
  options?: { enabled?: boolean; refetchOnWindowFocus?: boolean | 'always' },
) {
  return useQuery({
    queryKey: queryKeys.git.githubAppStatus(workspaceId ?? ''),
    queryFn: async () => unwrap(await gitService.getGitHubAppStatus(workspaceId!)),
    enabled: Boolean(workspaceId) && (options?.enabled ?? true),
    staleTime: 30_000,
    refetchOnWindowFocus: options?.refetchOnWindowFocus ?? true,
  });
}

/** Prepares a GitHub App manifest; the caller submits it to GitHub with a form POST. */
export function useCreateGitHubAppManifest(workspaceId?: string) {
  return useMutation({
    mutationFn: async (request: GitHubAppManifestRequest = {}) =>
      unwrap(await gitService.createGitHubAppManifest(workspaceId!, request)),
  });
}
