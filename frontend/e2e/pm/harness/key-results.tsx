import { useState } from 'react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createRoot } from 'react-dom/client';
import { ConfirmProvider } from '@/components/ui/confirm-dialog';
import { TooltipProvider } from '@/components/ui/tooltip';
import { QuietSectionHeader } from '@/components/design-system/quiet';
import { ObjectiveKeyResultRow } from '@/pages/pm/ObjectiveKeyResultRow';
import { LogKeyResultDialog } from '@/pages/pm/LogKeyResultDialog';
import { KeyResultEditorDialog } from '@/pages/pm/KeyResultEditorDialog';
import { pmObjectiveService } from '@/lib/services/pmObjectiveService';
import { unwrap } from '@/lib/queryUtils';
import type { KeyResult } from '@/lib/pmTypes';
import '@/index.css';

const params = new URLSearchParams(location.search);
if (params.get('theme') === 'dark') document.documentElement.classList.add('dark');
const base = { objective_id: 'objective', position: 0, created_at: '2026-09-01T00:00:00Z', updated_at: '2026-09-25T10:00:00Z' };
const fixtures: KeyResult[] = [
  { ...base, id: 'activation', name: 'Increase trial-to-paid conversion', result_type: 'percent', initial_value: 0, current_value: 35, target_value: 100, progress: 35 },
  { ...base, id: 'response', name: 'Reduce the average first response time for customers contacting our support team', result_type: 'numeric', initial_value: 60, current_value: 30, target_value: 10, progress: 60 },
  { ...base, id: 'launch', name: 'Launch the new onboarding experience', result_type: 'boolean', initial_value: 0, current_value: 0, target_value: 1, progress: 0 },
];
const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
export function Harness() {
  const [results, setResults] = useState(fixtures);
  const [logging, setLogging] = useState<string | null>(null);
  const [editing, setEditing] = useState<string | null>(null);
  const update = (updated: KeyResult) => setResults(values => values.map(value => value.id === updated.id ? updated : value));
  return <main className="min-h-screen bg-quiet-surface px-4 py-8 text-quiet-text-primary sm:px-8"><div className="mx-auto max-w-5xl">
    <h1 className="mb-8 text-xl font-semibold">Improve customer activation and support</h1>
    <section aria-label="Key results"><QuietSectionHeader title="Key Results" count={results.length} /><ul className="mt-2 border-t border-quiet-divider-light">
      {results.map(kr => <ObjectiveKeyResultRow key={kr.id} kr={kr} workspaceId="ws" memberMap={new Map()} onEdit={() => setEditing(kr.id)} onLog={() => setLogging(kr.id)} startDate="2026-09-01" deadline="2026-10-31" readOnly={params.has('readonly')} />)}
    </ul></section>
    {logging && <LogKeyResultDialog result={results.find(kr => kr.id === logging)!} onClose={() => setLogging(null)} onSave={async current_value => update(unwrap(await pmObjectiveService.updateKeyResult('ws', logging, { current_value })))} />}
    {editing && <KeyResultEditorDialog result={results.find(kr => kr.id === editing)} onClose={() => setEditing(null)} onSave={async values => update(unwrap(await pmObjectiveService.updateKeyResult('ws', editing, values)))} />}
  </div></main>;
}
createRoot(document.getElementById('root')!).render(<QueryClientProvider client={client}><ConfirmProvider><TooltipProvider><Harness /></TooltipProvider></ConfirmProvider></QueryClientProvider>);
