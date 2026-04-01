import { useCallback, useEffect, useRef, useState } from 'react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { Progress } from '@/components/ui/progress';
import { Card, CardContent } from '@/components/ui/card';
import { cn } from '@/lib/utils';
import { toast } from 'sonner';
import { Check, Clock, CheckCircle2, XCircle, Loader2, AlertTriangle } from 'lucide-react';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Badge } from '@/components/ui/badge';
import { ConfirmDialog } from '@/components/pm/ConfirmDialog';
import { QuickTooltip } from '@/components/ui/quick-tooltip';
import { useDocsSpaces } from '@/hooks/queries';
import {
  docsImportService,
  type ImportPreviewResponse,
  type ImportStatusResponse,
} from '@/lib/services/docsImportService';

type WizardStep = 0 | 1 | 2;
const STEP_LABELS = ['Connect', 'Configure', 'Import'];

function formatDuration(startISO: string, endISO: string): string {
  const seconds = Math.round((new Date(endISO).getTime() - new Date(startISO).getTime()) / 1000);
  if (seconds < 60) return `${seconds}s`;
  const mins = Math.floor(seconds / 60);
  const secs = seconds % 60;
  return secs > 0 ? `${mins}m ${secs}s` : `${mins}m`;
}

export function HelpCenterImportSection({
  workspaceId,
  editable,
}: {
  workspaceId: string;
  editable: boolean;
}) {
  const STORAGE_KEY = `helpin_docs_import_job_${workspaceId}`;

  const [step, setStep] = useState<WizardStep>(0);
  const [apiKey, setApiKey] = useState('');
  const [connecting, setConnecting] = useState(false);
  const [preview, setPreview] = useState<ImportPreviewResponse | null>(null);
  const [selectedCollection, setSelectedCollection] = useState('');
  const [targetSpaceId, setTargetSpaceId] = useState('');
  const [newSpaceName, setNewSpaceName] = useState('');
  const [importStatus, setImportStatus] = useState<'draft' | 'published' | 'match_source'>('match_source');
  const [jobId, setJobId] = useState('');
  const [jobStatus, setJobStatus] = useState<ImportStatusResponse | null>(null);
  const [starting, setStarting] = useState(false);
  const [nowMs, setNowMs] = useState(() => Date.now());
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const { data: spaces } = useDocsSpaces(workspaceId);
  const isFinished = jobStatus?.status === 'done' || jobStatus?.status === 'failed';

  const startPolling = useCallback((id: string) => {
    if (pollRef.current) clearInterval(pollRef.current);
    pollRef.current = setInterval(async () => {
      const { data } = await docsImportService.getStatus(workspaceId, id);
      if (data) {
        setJobStatus(data);
        if (data.status === 'done' || data.status === 'failed') {
          if (pollRef.current) clearInterval(pollRef.current);
          pollRef.current = null;
        }
      }
    }, 2000);
  }, [workspaceId]);

  // Resume active import job on mount
  useEffect(() => {
    const savedJobId = localStorage.getItem(STORAGE_KEY);
    if (savedJobId) {
      docsImportService.getStatus(workspaceId, savedJobId).then(({ data }) => {
        if (data && (data.status === 'running' || data.status === 'pending')) {
          setJobId(savedJobId);
          setJobStatus(data);
          setStep(2);
          startPolling(savedJobId);
        } else if (data && (data.status === 'done' || data.status === 'failed')) {
          setJobId(savedJobId);
          setJobStatus(data);
          setStep(2);
        } else {
          localStorage.removeItem(STORAGE_KEY);
        }
      });
    }
    return () => {
      if (pollRef.current) clearInterval(pollRef.current);
    };
  }, [STORAGE_KEY, startPolling, workspaceId]);

  useEffect(() => {
    if (step !== 2 || isFinished || !jobStatus?.started_at || jobStatus.completed === 0) {
      return;
    }
    const timer = setInterval(() => {
      setNowMs(Date.now());
    }, 1000);
    return () => clearInterval(timer);
  }, [isFinished, jobStatus?.completed, jobStatus?.started_at, step]);

  const handleConnect = async () => {
    if (!apiKey.trim()) {
      toast.error('Please enter an API key');
      return;
    }
    setConnecting(true);
    const { data, error } = await docsImportService.previewHelpscout(workspaceId, apiKey);
    setConnecting(false);
    if (error) {
      toast.error(error);
      return;
    }
    if (data) {
      setPreview(data);
      if (data.collections.length > 0) {
        setSelectedCollection(data.collections[0].id);
      }
      setStep(1);
    }
  };

  const handleStart = async () => {
    if (!selectedCollection) {
      toast.error('Please select a collection');
      return;
    }
    if (targetSpaceId === '__new' && !newSpaceName.trim()) {
      toast.error('Please enter a name for the new space');
      return;
    }
    setStarting(true);
    const { data, error } = await docsImportService.startHelpscout(workspaceId, {
      api_key: apiKey,
      helpscout_collection_id: selectedCollection,
      target_space_id: targetSpaceId && targetSpaceId !== '__new' ? targetSpaceId : undefined,
      new_space_name: targetSpaceId === '__new' ? newSpaceName.trim() : undefined,
      import_status: importStatus,
    });
    setStarting(false);
    if (error) {
      toast.error(error);
      return;
    }
    if (data) {
      setJobId(data.job_id);
      setJobStatus(null);
      setStep(2);
      localStorage.setItem(STORAGE_KEY, data.job_id);
      startPolling(data.job_id);
    }
  };

  const handleRetry = async () => {
    const { error } = await docsImportService.retry(workspaceId, jobId, apiKey);
    if (error) {
      toast.error(error);
      return;
    }
    setJobStatus(null);
    startPolling(jobId);
  };

  const handleDownloadRedirectMap = async () => {
    const { data, error } = await docsImportService.getRedirectMap(workspaceId, jobId);
    if (error || !data) {
      toast.error(error || 'Failed to download redirect map');
      return;
    }
    const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = 'redirect-map.json';
    a.click();
    URL.revokeObjectURL(url);
  };

  const handleReset = () => {
    if (pollRef.current) clearInterval(pollRef.current);
    pollRef.current = null;
    localStorage.removeItem(STORAGE_KEY);
    setStep(0);
    setApiKey('');
    setPreview(null);
    setSelectedCollection('');
    setTargetSpaceId('');
    setNewSpaceName('');
    setImportStatus('match_source');
    setJobId('');
    setJobStatus(null);
  };

  const progressPercent = jobStatus && jobStatus.total > 0
    ? Math.round(((jobStatus.completed + jobStatus.failed) / jobStatus.total) * 100)
    : 0;

  const STATUS_OPTIONS = [
    { value: 'draft' as const, label: 'Import as draft', description: 'All articles will be imported as drafts for review' },
    { value: 'published' as const, label: 'Import as published', description: 'All articles will be published immediately' },
    { value: 'match_source' as const, label: 'Match source status', description: 'Preserves original published/draft status from HelpScout' },
  ];

  return (
    <div className="space-y-6">
      {/* Step indicator */}
      <div className="flex items-center justify-center gap-2">
        {STEP_LABELS.map((label, i) => (
          <div key={label} className="flex items-center gap-2">
            {i > 0 && <div className="h-px w-8 bg-border" />}
            <div className="flex items-center gap-1.5">
              <div
                className={cn(
                  'flex h-6 w-6 items-center justify-center rounded-full text-xs font-medium',
                  i < step
                    ? 'bg-primary text-primary-foreground'
                    : i === step
                      ? 'bg-primary text-primary-foreground'
                      : 'bg-muted text-muted-foreground',
                )}
              >
                {i < step ? <Check className="h-3.5 w-3.5" /> : i + 1}
              </div>
              <span className={cn('text-sm', i <= step ? 'font-medium' : 'text-muted-foreground')}>
                {label}
              </span>
            </div>
          </div>
        ))}
      </div>

      {/* Step 0: Connect */}
      {step === 0 && (
        <div className="mx-auto max-w-md rounded-xl border border-border bg-card p-6 space-y-4">
          <div className="space-y-2">
            <Label>Source</Label>
            <Select value="helpscout" disabled>
              <SelectTrigger>
                <SelectValue placeholder="Select source" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="helpscout">HelpScout</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div className="space-y-2">
            <Label>API Key</Label>
            <Input
              type="password"
              placeholder="Enter your HelpScout Docs API key"
              value={apiKey}
              onChange={(e) => setApiKey(e.target.value)}
              disabled={!editable || connecting}
            />
            <p className="text-xs text-muted-foreground">
              Find your API key in HelpScout → Your Profile → Authentication → API Keys
            </p>
          </div>
          <div className="flex justify-center">
            <Button onClick={handleConnect} disabled={!editable || connecting || !apiKey.trim()}>
              {connecting ? <><Loader2 className="h-4 w-4 animate-spin mr-1.5" />Connecting...</> : 'Connect & Preview'}
            </Button>
          </div>
        </div>
      )}

      {/* Step 1: Configure */}
      {step === 1 && (
        <div className="mx-auto max-w-md rounded-xl border border-border bg-card p-6 space-y-4">
          {preview && preview.collections.length > 0 && (
            <div className="space-y-2">
              <Label>Collection</Label>
              <Select value={selectedCollection} onValueChange={setSelectedCollection} disabled={preview.collections.length === 1}>
                <SelectTrigger>
                  <SelectValue placeholder="Select collection" />
                </SelectTrigger>
                <SelectContent>
                  {preview.collections.map((col) => (
                    <SelectItem key={col.id} value={col.id}>
                      {col.name} ({col.article_count} articles)
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          )}

          <div className="space-y-2">
            <Label>Target Space</Label>
            <Select value={targetSpaceId} onValueChange={setTargetSpaceId}>
              <SelectTrigger className="w-full">
                <SelectValue placeholder="Select a space or create new" />
              </SelectTrigger>
              <SelectContent>
                {spaces?.map((space) => (
                  <SelectItem key={space.id} value={space.id}>{space.name}</SelectItem>
                ))}
                <SelectItem value="__new">Create new space</SelectItem>
              </SelectContent>
            </Select>
            {targetSpaceId === '__new' && (
              <Input
                placeholder="New space name"
                value={newSpaceName}
                onChange={(e) => setNewSpaceName(e.target.value)}
              />
            )}
          </div>

          <div className="space-y-2">
            <Label>Import Status</Label>
            <div className="space-y-2">
              {STATUS_OPTIONS.map((option) => (
                <button
                  key={option.value}
                  type="button"
                  onClick={() => setImportStatus(option.value)}
                  className={cn(
                    'flex w-full items-start gap-3 rounded-md border p-3 text-left transition-colors',
                    importStatus === option.value
                      ? 'border-primary bg-primary/5'
                      : 'border-border hover:bg-muted/50',
                  )}
                >
                  <div className={cn(
                    'mt-0.5 h-4 w-4 shrink-0 rounded-full border-2',
                    importStatus === option.value
                      ? 'border-primary bg-primary'
                      : 'border-muted-foreground/40',
                  )} />
                  <div className="min-w-0">
                    <p className="text-sm font-medium">{option.label}</p>
                    <p className="text-xs text-muted-foreground">{option.description}</p>
                  </div>
                </button>
              ))}
            </div>
          </div>

          <div className="flex items-center justify-between">
            <Button variant="ghost" onClick={() => setStep(0)}>Back</Button>
            <Button onClick={handleStart} disabled={starting}>
              {starting ? 'Starting...' : 'Start Import'}
            </Button>
          </div>
        </div>
      )}

      {/* Step 2: Import Progress */}
      {step === 2 && (
        <div className="mx-auto max-w-md rounded-xl border border-border bg-card p-6 space-y-4">
          <div className="space-y-3">
            {!isFinished && (
              <div className="flex justify-center">
                <div className="h-6 w-6 animate-spin rounded-full border-2 border-muted-foreground/30 border-t-foreground" />
              </div>
            )}
            <Progress value={progressPercent} className="h-2" />
            <p className="text-sm text-muted-foreground text-center">
              {isFinished
                ? 'Import complete'
                : `Importing... ${jobStatus ? jobStatus.completed + jobStatus.failed : 0} of ${jobStatus?.total ?? '...'} articles`}
            </p>
            {!isFinished && jobStatus && jobStatus.completed > 0 && jobStatus.started_at && (() => {
              const elapsed = (nowMs - new Date(jobStatus.started_at).getTime()) / 1000;
              const done = jobStatus.completed + jobStatus.failed;
              const remaining = jobStatus.total - done;
              const perItem = elapsed / done;
              const etaSeconds = Math.round(remaining * perItem);
              const etaMin = Math.floor(etaSeconds / 60);
              const etaSec = etaSeconds % 60;
              return (
                <p className="text-xs text-muted-foreground/60 text-center">
                  ~{etaMin > 0 ? `${etaMin}m ` : ''}{etaSec}s remaining
                </p>
              );
            })()}
            {!isFinished && (
              <p className="text-xs text-muted-foreground/50 text-center">
                You can continue using the app. Come back here later to see the progress.
              </p>
            )}
          </div>

          {isFinished && jobStatus && (
            <Card>
              <CardContent className="pt-4 space-y-3">
                {jobStatus.summary && (
                  <div className="space-y-2">
                    <p className="text-sm text-muted-foreground">
                      Created {jobStatus.summary.collections_created}{' '}
                      {jobStatus.summary.collections_created === 1 ? 'collection' : 'collections'} and{' '}
                      {jobStatus.completed}{' '}
                      {jobStatus.completed === 1 ? 'article' : 'articles'}
                      {(jobStatus.summary.articles_published > 0 || jobStatus.summary.articles_drafted > 0) && (
                        <> ({jobStatus.summary.articles_published} published, {jobStatus.summary.articles_drafted} draft)</>
                      )}
                      . {jobStatus.summary.redirects_created} URL{' '}
                      {jobStatus.summary.redirects_created === 1 ? 'redirect' : 'redirects'} set up
                      {jobStatus.started_at && jobStatus.completed_at && (
                        <>. Completed in {formatDuration(jobStatus.started_at, jobStatus.completed_at)}</>
                      )}
                      .
                    </p>
                    {(jobStatus.summary.articles_uncategorized > 0 ||
                      jobStatus.summary.articles_with_conversion_warnings > 0 ||
                      jobStatus.summary.html_block_fallbacks > 0 ||
                      jobStatus.summary.image_rewrite_failures > 0 ||
                      jobStatus.summary.normalized_note_blocks > 0) && (
                      <div className="flex flex-wrap gap-2">
                        {jobStatus.summary.normalized_note_blocks > 0 && (
                          <Badge variant="secondary" className="text-xs">
                            {jobStatus.summary.normalized_note_blocks} note blocks normalized
                          </Badge>
                        )}
                        {jobStatus.summary.articles_uncategorized > 0 && (
                          <Badge variant="secondary" className="text-xs">
                            {jobStatus.summary.articles_uncategorized} uncategorized
                          </Badge>
                        )}
                        {jobStatus.summary.html_block_fallbacks > 0 && (
                          <Badge variant="secondary" className="text-xs">
                            {jobStatus.summary.html_block_fallbacks} HTML fallbacks
                          </Badge>
                        )}
                        {jobStatus.summary.image_rewrite_failures > 0 && (
                          <Badge variant="secondary" className="text-xs">
                            {jobStatus.summary.image_rewrite_failures} image URLs kept
                          </Badge>
                        )}
                        {jobStatus.summary.articles_with_conversion_warnings > 0 && (
                          <Badge variant="secondary" className="text-xs">
                            {jobStatus.summary.articles_with_conversion_warnings} articles with warnings
                          </Badge>
                        )}
                      </div>
                    )}
                  </div>
                )}
                <p className="text-sm">
                  {jobStatus.completed} succeeded, {jobStatus.failed} failed
                </p>

                {jobStatus.failures && jobStatus.failures.length > 0 && (
                  <div className="max-h-48 overflow-y-auto rounded-md border border-border bg-muted/30 p-3 space-y-2">
                    {jobStatus.failures.map((f) => (
                      <div key={f.article_id} className="text-xs">
                        <span className="font-medium">{f.title}</span>
                        <span className="text-destructive"> — {f.error}</span>
                      </div>
                    ))}
                  </div>
                )}

                <div className="flex flex-wrap items-center gap-2">
                  {jobStatus.failed > 0 && (
                    <Button size="sm" variant="outline" onClick={handleRetry}>
                      Retry Failed
                    </Button>
                  )}
                  <Button size="sm" variant="outline" onClick={handleDownloadRedirectMap}>
                    Download Redirect Map
                  </Button>
                  <Button size="sm" onClick={handleReset}>
                    Start New Import
                  </Button>
                </div>
              </CardContent>
            </Card>
          )}
        </div>
      )}
      {/* Import History */}
      <ImportHistory workspaceId={workspaceId} />
    </div>
  );
}

// ── Import History ──────────────────────────────────────────────────────────

const STATUS_CONFIG: Record<string, { label: string; icon: typeof Check; className: string }> = {
  pending: { label: 'Pending', icon: Clock, className: 'bg-muted text-muted-foreground' },
  running: { label: 'In Progress', icon: Loader2, className: 'bg-blue-100 text-blue-700' },
  done: { label: 'Completed', icon: CheckCircle2, className: 'bg-green-100 text-green-700' },
  failed: { label: 'Failed', icon: XCircle, className: 'bg-red-100 text-red-700' },
  interrupted: { label: 'Interrupted', icon: AlertTriangle, className: 'bg-amber-100 text-amber-700' },
};

function ImportHistory({ workspaceId }: { workspaceId: string }) {
  const [jobs, setJobs] = useState<ImportStatusResponse[]>([]);
  const [loading, setLoading] = useState(true);
  const [reconvertingId, setReconvertingId] = useState<string | null>(null);
  const [reconvertConfirmId, setReconvertConfirmId] = useState<string | null>(null);

  useEffect(() => {
    docsImportService.listJobs(workspaceId).then(({ data }) => {
      setJobs(data ?? []);
      setLoading(false);
    });
  }, [workspaceId]);

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
      ].filter(Boolean)
      toast.success(
        `Re-converted ${data.converted} of ${data.total} documents${data.failed > 0 ? ` (${data.failed} failed)` : ''}${qualityBits.length > 0 ? `. ${qualityBits.join(', ')}` : ''}`,
      );
    }
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
                      <span className="text-green-600">{job.completed}</span>
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
                            <><Loader2 className="h-3 w-3 animate-spin mr-1" />Converting...</>
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
