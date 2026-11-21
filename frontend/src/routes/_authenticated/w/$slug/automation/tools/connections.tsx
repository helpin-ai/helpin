import { createFileRoute, redirect } from '@tanstack/react-router';
import { validateExternalToolsSearch } from '@/lib/externalToolsSearch';

export const Route = createFileRoute('/_authenticated/w/$slug/automation/tools/connections')({
  validateSearch: validateExternalToolsSearch,
  beforeLoad: ({ params, search }) => {
    throw redirect({ to: '/w/$slug/settings/external-tools', params, search: { ...search, tab: 'mcp' }, replace: true });
  },
});
