import { useEffect, useState } from 'react';
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';
import { toast } from 'sonner';
import { Tick01Icon, Clock01Icon, CheckmarkCircle02Icon, CancelCircleIcon, Loading01Icon, Alert01Icon } from '@/lib/icons';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Badge } from '@/components/ui/badge';
import { ConfirmDialog } from '@/components/pm/ConfirmDialog';
import { QuickTooltip } from '@/components/ui/quick-tooltip';
import {
  docsImportService,
  type ImportStatusResponse,
} from '@/lib/services/docsImportService';

const STATUS_CONFIG: Record<string, { label: string; icon: typeof Tick01Icon; className: string }> = {
  pending: { label: 'Pending', icon: Clock01Icon, className: 'bg-muted text-muted-foreground' },
  running: { label: 'In Progress', icon: Loading01Icon, className: 'bg-blue-100 text-blue-700' },
  done: { label: 'Completed', icon: CheckmarkCircle02Icon, className: 'bg-green-100 text-green-700' },
  failed: { label: 'Failed', icon: CancelCircleIcon, className: 'bg-red-100 text-red-700' },
  interrupted: { label: 'Interrupted', icon: Alert01Icon, className: 'bg-amber-100 text-amber-700' },
};

export function ImportHistory({ workspaceId, source }: { workspaceId: string; source?: string }) {
  const [jobs, setJobs] = useState<ImportStatusResponse[]>([]);
  const [loading, setLoading] = useState(true);
  const [reconvertingId, setReconvertingId] = useState<string | null>(null);
  const [reconvertConfirmId, setReconvertConfirmId] = useState<string | null>(null);
  const [cancelingId, setCancelingId] = useState<string | null>(null);

  useEffect(() => {
    docsImportService.listJobs(workspaceId).then(({ data }) => {
      let allJobs = data ?? [];
      if (source) {
        allJobs = allJobs.filter((j) => j.source === source);
      }
      setJobs(allJobs);
      setLoading(false);
    });
  }, [workspaceId, source]);

  const handleReconvert = async (jobId: string) => {
    setReconvertConfirmId(null);
    setReconvertingId(jobId);
    const { data, error } = await docsImportService.reconvert(workspaceId, jobId);
    setReconvertingId(null);
    if (error) {
      toast.error(error);
    } else if (data) {
      const qualityBits = [
        data.articles_with_warnings > 0 ? `${data.articles_with_warnings} with warnings` : '',
        data.normalized_note_blocks > 0 ? `${data.normalized_note_blocks} note blocks normalized` : '',
      ].filter(Boolean);
      toast.success(
        `Re-converted ${data.converted} of ${data.total} documents${data.failed > 0 ? ` (${data.failed} failed)` : ''}${qualityBits.length > 0 ? `. ${qualityBits.join(', ')}` : ''}`,
      );
    }
  };

  const handleCancel = async (jobId: string) => {
    setCancelingId(jobId);
    const { error } = await docsImportService.cancel(workspaceId, jobId);
    setCancelingId(null);
    if (error) {
      toast.error(error);
      return;
    }
    setJobs((current) =>
      current.map((job) => (job.id === jobId ? { ...job, status: 'interrupted' } : job)),
    );
    toast.success('Import canceled');
  };

  if (loading) return null;
  if (jobs.length === 0) return null;

  return (
    <div className="mt-6 max-w-2xl mx-auto">
      <h4 className="text-sm font-medium mb-3">Import History</h4>
      <div className="rounded-lg border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-[160px]">Date</TableHead>
              <TableHead className="w-[80px]">Source</TableHead>
              <TableHead className="w-[140px]">Articles</TableHead>
              <TableHead className="w-[110px]">Status</TableHead>
              <TableHead className="w-[90px]"></TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {jobs.map((job) => {
              const config = STATUS_CONFIG[job.status] ?? STATUS_CONFIG.pending;
              const Icon = config.icon;
              const date = new Date(job.created_at);
              return (
                <TableRow key={job.id}>
                  <TableCell className="text-sm">
                    {date.toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: 'numeric' })}
                    <span className="text-muted-foreground ml-1.5 text-xs">
                      {date.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' })}
                    </span>
                  </TableCell>
                  <TableCell>
                    <span className="text-sm capitalize">{job.source || 'HelpScout'}</span>
                  </TableCell>
                  <TableCell>
                    <span className="text-sm">
                      <span className="text-green-600 dark:text-green-400">{job.completed}</span>
                      {job.failed > 0 && (
                        <span className="text-destructive ml-1">/ {job.failed} failed</span>
                      )}
                      <span className="text-muted-foreground ml-1">of {job.total}</span>
                    </span>
                  </TableCell>
                  <TableCell>
                    <Badge variant="secondary" className={cn('gap-1 text-xs', config.className)}>
                      <Icon className={cn('h-3 w-3', job.status === 'running' && 'animate-spin')} />
                      {config.label}
                    </Badge>
                  </TableCell>
                  <TableCell>
                    {(job.status === 'pending' || job.status === 'running') && (
                      <Button
                        variant="ghost"
                        size="sm"
                        className="h-7 text-xs"
                        disabled={cancelingId === job.id}
                        onClick={() => handleCancel(job.id)}
                      >
                        {cancelingId === job.id ? 'Canceling...' : 'Cancel'}
                      </Button>
                    )}
                    {job.status === 'done' && (
                      <QuickTooltip label="Re-run conversion with latest logic. Overwrites imported doc content.">
                        <Button
                          variant="ghost"
                          size="sm"
                          className="h-7 text-xs"
                          disabled={reconvertingId === job.id}
                          onClick={() => setReconvertConfirmId(job.id)}
                        >
                          {reconvertingId === job.id ? (
                            <><Loading01Icon className="h-3 w-3 animate-spin mr-1" />Converting...</>
                          ) : (
                            'Re-convert'
                          )}
                        </Button>
                      </QuickTooltip>
                    )}
                  </TableCell>
                </TableRow>
              );
            })}
          </TableBody>
        </Table>
      </div>

      <ConfirmDialog
        open={reconvertConfirmId !== null}
        onOpenChange={(open) => { if (!open) setReconvertConfirmId(null); }}
        title="Re-convert imported documents"
        description="This will re-convert all imported documents from this job using the latest conversion logic. Any manual edits made to imported documents after import will be overwritten. This cannot be undone."
        confirmLabel="Re-convert"
        variant="destructive"
        onConfirm={() => { if (reconvertConfirmId) handleReconvert(reconvertConfirmId); }}
      />
    </div>
  );
}
