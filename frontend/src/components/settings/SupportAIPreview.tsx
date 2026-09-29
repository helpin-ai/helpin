import { useState } from 'react';
import { useMutation, useQuery } from '@tanstack/react-query';
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { Label } from '@/components/ui/label';
import { Textarea } from '@/components/ui/textarea';
import { agentService } from '@/lib/services/agentService';
import { unwrap } from '@/lib/queryUtils';
import type { SupportAIPreviewResponse } from '@/lib/pmTypes';

export function SupportAIPreview({ workspaceId, agentId }: { workspaceId: string; agentId: string }) {
  const [message, setMessage] = useState('');
  const [started, setStarted] = useState<SupportAIPreviewResponse | null>(null);
  const start = useMutation({
    mutationFn: async () => unwrap(await agentService.previewSupportReply(workspaceId, agentId, { message: message.trim() })),
    onSuccess: setStarted,
  });
  const result = useQuery({
    queryKey: ['support-preview', workspaceId, agentId, started?.run_id],
    enabled: Boolean(started?.run_id),
    queryFn: async () => unwrap(await agentService.getSupportPreview(workspaceId, agentId, started!.run_id!)),
    refetchInterval: (query) => query.state.data && query.state.data.final_decision !== 'pending' ? false : 1000,
    retry: 2,
  });
  const response = result.data ?? started;
  const busy = start.isPending || response?.final_decision === 'pending';
  const stop = useMutation({
    mutationFn: async () => unwrap(await agentService.cancelSupportPreview(workspaceId, agentId, started!.run_id!)),
    onSuccess: () => { void result.refetch(); },
  });
  const error = start.error ?? result.error ?? stop.error;
  return (
    <Dialog>
      <DialogTrigger asChild>
        <Button variant="outline" size="sm">Test AI response</Button>
      </DialogTrigger>
      <DialogContent className="max-h-[85dvh] overflow-y-auto sm:max-w-2xl">
        <DialogHeader className="pr-8">
          <DialogTitle>Test AI response</DialogTitle>
          <DialogDescription>Uses saved assistant settings without contacting customers. External tools are unavailable; normal model and knowledge-search usage applies.</DialogDescription>
        </DialogHeader>
        <div className="space-y-3">
          <Label htmlFor="support-preview-message">Customer message</Label>
          <Textarea id="support-preview-message" value={message} onChange={(event) => setMessage(event.target.value)} maxLength={16000} rows={3} placeholder="Ask a question your customers might ask" />
          <div className="flex gap-2">
            <Button size="sm" onClick={() => start.mutate()} disabled={busy || !message.trim()}>{busy ? 'Testing…' : 'Test response'}</Button>
            {busy && started?.run_id && <Button size="sm" variant="outline" disabled={stop.isPending} onClick={() => stop.mutate()}>Stop</Button>}
          </div>
          {error && <p role="alert" className="text-sm text-destructive">{error.message}</p>}
          {response && <div className="space-y-2" aria-live="polite">
            {response.provider && <p className="text-xs text-muted-foreground">{response.provider} / {response.model}</p>}
            <p className="text-sm">{response.final_decision === 'pending' ? 'Waiting for the assistant…' : `${response.final_decision === 'no_reply' ? 'No reply sent' : response.final_decision}: ${response.final_reason.replaceAll('_', ' ')}`}</p>
            {response.answer && <>
              {!response.answer.can_answer && <p className="text-sm text-destructive">This proposed reply failed validation and would not be sent.</p>}
              <p className="whitespace-pre-wrap rounded-md bg-muted p-3 text-sm">{response.answer.content}</p>
            </>}
            {(response.retrieval?.results?.length ?? 0) > 0 && <details className="text-sm"><summary>Knowledge used ({response.retrieval.results.length})</summary><ul className="mt-2 space-y-2">{response.retrieval.results.map((item, index) => <li key={`${item.reference_id}:${index}`}><strong>{item.title || 'Untitled source'}</strong><p className="text-muted-foreground">{item.snippet}</p></li>)}</ul></details>}
            {(response.excluded_tools?.length ?? 0) > 0 && <details className="text-xs text-muted-foreground"><summary>Tools unavailable in preview</summary>{response.excluded_tools?.join(', ')}</details>}
          </div>}
        </div>
      </DialogContent>
    </Dialog>
  );
}
