export function gitWebBaseURL(provider?: string, baseURL?: string | null) {
  const trimmed = baseURL?.trim().replace(/\/+$/, '');
  if (trimmed) return trimmed;
  return provider === 'gitlab' ? 'https://gitlab.com' : 'https://github.com';
}

export function gitRepoURL(provider: string | undefined, repoFullName: string, baseURL?: string | null) {
  const repoPath = repoFullName.split('/').map(encodeURIComponent).join('/');
  return `${gitWebBaseURL(provider, baseURL)}/${repoPath}`;
}

export function gitBranchURL(provider: string | undefined, repoFullName: string, branch: string, baseURL?: string | null) {
  const branchPath = encodeURIComponent(branch);
  const branchSegment = provider === 'gitlab' ? '-/tree' : 'tree';
  return `${gitRepoURL(provider, repoFullName, baseURL)}/${branchSegment}/${branchPath}`;
}

export function gitCommitURL(provider: string | undefined, repoFullName: string, sha: string, baseURL?: string | null) {
  const commitSegment = provider === 'gitlab' ? '-/commit' : 'commit';
  return `${gitRepoURL(provider, repoFullName, baseURL)}/${commitSegment}/${encodeURIComponent(sha)}`;
}
