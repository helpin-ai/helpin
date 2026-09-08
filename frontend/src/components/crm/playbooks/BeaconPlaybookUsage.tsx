import { Link } from '@tanstack/react-router';
import { usePermissions, useWorkspaceAccess } from '@/hooks/queries/useSession';
import { useCRMPlaybookAgentUsage } from '@/hooks/queries/useCRMPlaybookAutomation';
import { PlaybookHelp } from './PlaybookUI';

// Read-only CRM dependencies; never edits the shared Agent or saved skills.
export function BeaconPlaybookUsage({ ws, slug, agentId }: { ws: string; slug: string; agentId: string }) {
  const access = useWorkspaceAccess(ws);
  const { has, canAccessModule } = usePermissions(access.data);
  const usage = useCRMPlaybookAgentUsage(ws, agentId, has('crm.read') && canAccessModule('crm'));
  if (!usage.data?.length) return null;
  return <section className="border-b border-quiet-divider-strong py-4">
    <h3 className="mb-2 flex items-center gap-1 text-sm font-medium">Used by Playbooks<PlaybookHelp label="About playbook connections">Each playbook keeps its reviewed configuration and job skills. Editing Beacon here does not silently change work already in progress.</PlaybookHelp></h3>
    <ul className="space-y-2">{usage.data.map((book) => <li key={book.playbook_id}><Link to="/w/$slug/crm/playbooks/$playbookId" params={{ slug, playbookId: book.playbook_id }} className="text-sm text-quiet-accent hover:underline">{book.name}</Link></li>)}</ul>
  </section>;
}
