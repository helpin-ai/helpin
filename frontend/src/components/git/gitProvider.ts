export type GitProvider = 'github' | 'gitlab';

export const gitProviderLabels: Record<GitProvider, string> = {
  github: 'GitHub',
  gitlab: 'GitLab',
};

/** The provider for a stored integration or repository value; anything that isn't GitLab is GitHub. */
export function gitProviderOf(value: string | null | undefined): GitProvider {
  return value === 'gitlab' ? 'gitlab' : 'github';
}
