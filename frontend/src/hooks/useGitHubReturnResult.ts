import { useEffect, useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { readGitHubReturnResult, stripGitHubReturnParams, type GitHubReturnResult } from '@/lib/githubReturn';

/**
 * Reads the result of a GitHub App create/install flow from the URL once,
 * removes the flags from the address bar, and refreshes GitHub-dependent data.
 */
export function useGitHubReturnResult(): GitHubReturnResult | null {
  const queryClient = useQueryClient();
  const [result] = useState(() => (typeof window === 'undefined' ? null : readGitHubReturnResult(window.location.search)));

  useEffect(() => {
    if (!result) return;
    stripGitHubReturnParams();
    void queryClient.invalidateQueries({ queryKey: ['git'] });
    void queryClient.invalidateQueries({
      predicate: (query) => query.queryKey[0] === 'workspaces' && query.queryKey[2] === 'capabilities',
    });
  }, [queryClient, result]);

  return result;
}
