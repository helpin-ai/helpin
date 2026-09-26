import { createFileRoute } from '@tanstack/react-router';
import { parseGitHubInstalledQuery } from '@/lib/githubReturn';
import { GitHubInstalledPage } from '@/pages/github/GitHubInstalledPage';

export const Route = createFileRoute('/_authenticated/github/installed')({
  validateSearch: parseGitHubInstalledQuery,
  component: GitHubInstalledRoute,
});

function GitHubInstalledRoute() {
  return <GitHubInstalledPage query={Route.useSearch()} />;
}
