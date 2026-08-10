import { useCallback, useEffect, useRef, useState } from 'react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Card, CardContent } from '@/components/ui/card';
import { Progress } from '@/components/ui/progress';
import { Badge } from '@/components/ui/badge';
import { cn } from '@/lib/utils';
import { toast } from 'sonner';
import { Tick01Icon, Upload01Icon, AlertCircleIcon, InformationCircleIcon, Loading01Icon } from '@/lib/icons';
import {
  docsImportService,
  type NextraImportPreviewResponse,
  type ImportStatusResponse,
} from '@/lib/services/docsImportService';
import { useDocsSpaces } from '@/hooks/queries/useDocs';
import { ImportHistory } from '@/components/settings/ImportHistory';

type WizardStep = 0 | 1 | 2;
const STEP_LABELS = ['Select Source', 'Configure', 'Import'];

const STATUS_OPTIONS = [
  { value: 'draft' as const, label: 'Import as draft', description: 'All articles will be imported as drafts for review' },
  { value: 'published' as const, label: 'Import as published', description: 'All articles will be published immediately' },
];

function formatDuration(startISO: string, endISO: string): string {
  const seconds = Math.round((new Date(endISO).getTime() - new Date(startISO).getTime()) / 1000);
  if (seconds < 60) return `${seconds}s`;
  const mins = Math.floor(seconds / 60);
  const secs = seconds % 60;
  return secs > 0 ? `${mins}m ${secs}s` : `${mins}m`;
}

export function NextraImportWizard({ workspaceId }: { workspaceId: string }) {
  const STORAGE_KEY = `helpin_nextra_import_job_${workspaceId}`;

  const [step, setStep] = useState<WizardStep>(0);
  const [file, setFile] = useState<File | null>(null);
  const [sourceCommit, setSourceCommit] = useState('');
  const [uploading, setUploading] = useState(false);
  const [uploadStatus, setUploadStatus] = useState('');
  const [preview, setPreview] = useState<NextraImportPreviewResponse | null>(null);
  const [targetSpaceId, setTargetSpaceId] = useState('');
  const [newSpaceName, setNewSpaceName] = useState('');
  const [importStatus, setImportStatus] = useState<'draft' | 'published'>('draft');
  const [jobId, setJobId] = useState('');
  const [jobStatus, setJobStatus] = useState<ImportStatusResponse | null>(null);
  const [starting, setStarting] = useState(false);
  const [nowMs, setNowMs] = useState(() => Date.now());
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

  const { data: spaces } = useDocsSpaces(workspaceId);
  const isFinished = jobStatus?.status === 'done' || jobStatus?.status === 'failed';

  // --- Polling ---

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

  // Resume the newest active server job, including imports started in another browser.
  useEffect(() => {
    let active = true;
    const showJob = (job: ImportStatusResponse) => {
      if (!active) return;
      setJobId(job.id);
      setJobStatus(job);
      setStep(2);
      localStorage.setItem(STORAGE_KEY, job.id);
      if (job.status === 'running' || job.status === 'pending') startPolling(job.id);
    };
    const resumeImport = async () => {
      const { data: jobs } = await docsImportService.listJobs(workspaceId);
      const runningJob = (jobs ?? []).find(
        (job) =>
          job.source === 'nextra' &&
          Boolean(job.space_id) &&
          (job.status === 'running' || job.status === 'pending'),
      );
      if (runningJob) {
        showJob(runningJob);
        return;
      }

      const savedJobId = localStorage.getItem(STORAGE_KEY);
      if (!savedJobId) return;
      const { data: savedJob } = await docsImportService.getStatus(workspaceId, savedJobId);
      if (savedJob?.source === 'nextra') {
        showJob(savedJob);
      } else {
        localStorage.removeItem(STORAGE_KEY);
      }
    };
    void resumeImport();
    return () => {
      active = false;
      if (pollRef.current) clearInterval(pollRef.current);
    };
  }, [STORAGE_KEY, startPolling, workspaceId]);

  // ETA timer.
  useEffect(() => {
    if (step !== 2 || isFinished || !jobStatus?.started_at || jobStatus.completed === 0) return;
    const timer = setInterval(() => setNowMs(Date.now()), 1000);
    return () => clearInterval(timer);
  }, [isFinished, jobStatus?.completed, jobStatus?.started_at, step]);

  // --- Handlers ---

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const selected = e.target.files?.[0];
    if (selected) {
      if (!selected.name.endsWith('.zip')) {
        toast.error('Please select a .zip file');
        return;
      }
      setFile(selected);
    }
  };

  const handlePreview = async () => {
    if (!file) return;
    setUploading(true);
    setUploadStatus(`Analyzing ${file.name} (${(file.size / 1024 / 1024).toFixed(1)} MB)...`);
    try {
      const { data, error } = await docsImportService.previewNextra(workspaceId, {
        archive: file,
        source_commit: sourceCommit || undefined,
      });
      setUploading(false);
      setUploadStatus('');
      if (error) {
        toast.error(error);
        return;
      }
      if (data) {
        data.spaces = data.spaces || [];
        data.warnings = data.warnings || [];
        data.broken_links = data.broken_links || [];
        data.unsupported_components = data.unsupported_components || [];
        setPreview(data);
        setNewSpaceName('');
        setStep(1);
      }
    } catch {
      setUploading(false);
      setUploadStatus('');
      toast.error('Analysis failed. The file may be too large or the connection timed out.');
    }
  };

  const handleStart = async () => {
    if (!preview) return;
    if (targetSpaceId === '__new' && !newSpaceName.trim()) {
      toast.error('Please enter a name for the new space');
      return;
    }
    setStarting(true);
    const { data, error } = await docsImportService.startNextra(workspaceId, {
      job_id: preview.job_id,
      target_space_id: targetSpaceId && targetSpaceId !== '__new' ? targetSpaceId : undefined,
      new_space_name: targetSpaceId === '__new' ? newSpaceName.trim() : undefined,
      import_status: importStatus,
      create_redirects: true,
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
    setFile(null);
    setSourceCommit('');
    setPreview(null);
    setTargetSpaceId('');
    setNewSpaceName('');
    setImportStatus('draft');
    setJobId('');
    setJobStatus(null);
  };

  const progressPercent = jobStatus && jobStatus.total > 0
    ? Math.round(((jobStatus.completed + jobStatus.failed) / jobStatus.total) * 100)
    : 0;

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
                {i < step ? <Tick01Icon className="h-3.5 w-3.5" /> : i + 1}
              </div>
              <span className={cn('text-sm', i <= step ? 'font-medium' : 'text-muted-foreground')}>
                {label}
              </span>
            </div>
          </div>
        ))}
      </div>

      {/* Step 0: Upload */}
      {step === 0 && (
        <div className="mx-auto max-w-md rounded-xl border border-border bg-card p-6 space-y-4">
          <div className="space-y-2">
            <Label htmlFor="nextra-archive">Archive (.zip)</Label>
            <div className="flex items-center gap-3">
              <input
                ref={fileInputRef}
                id="nextra-archive"
                type="file"
                accept=".zip"
                onChange={handleFileChange}
                className="hidden"
              />
              <Button
                variant="outline"
                size="sm"
                onClick={() => fileInputRef.current?.click()}
                disabled={uploading}
                className="gap-2"
              >
                <Upload01Icon className="h-4 w-4" />
                Choose file
              </Button>
              {file && (
                <span className="text-sm text-muted-foreground truncate">
                  {file.name} ({(file.size / 1024 / 1024).toFixed(1)} MB)
                </span>
              )}
            </div>
            <p className="text-xs text-muted-foreground">
              Select a .zip of your Nextra docs repository
            </p>
          </div>

          <div className="flex flex-col items-center gap-2">
            <Button onClick={handlePreview} disabled={!file || uploading} className="w-full">
              {uploading ? <><Loading01Icon className="h-4 w-4 animate-spin mr-1.5" />Analyzing...</> : 'Analyze & Preview'}
            </Button>
            {uploading && uploadStatus && (
              <p className="text-xs text-muted-foreground">{uploadStatus}</p>
            )}
          </div>
        </div>
      )}

      {/* Step 1: Configure */}
      {step === 1 && preview && (
        <div className="mx-auto max-w-md rounded-xl border border-border bg-card p-6 space-y-4">
          {/* Preview stats */}
          <div className="grid grid-cols-2 gap-2">
            <div className="rounded-md bg-muted/50 p-2.5 text-center">
              <p className="text-lg font-bold">{preview.collections}</p>
              <p className="text-xs text-muted-foreground">Collections</p>
            </div>
            <div className="rounded-md bg-muted/50 p-2.5 text-center">
              <p className="text-lg font-bold">{preview.articles}</p>
              <p className="text-xs text-muted-foreground">Articles</p>
            </div>
            <div className="rounded-md bg-muted/50 p-2.5 text-center">
              <p className="text-lg font-bold">{preview.assets}</p>
              <p className="text-xs text-muted-foreground">Assets</p>
            </div>
            <div className="rounded-md bg-muted/50 p-2.5 text-center">
              <p className="text-lg font-bold">{preview.redirects}</p>
              <p className="text-xs text-muted-foreground">Redirects</p>
            </div>
          </div>

          {/* Warnings */}
          {preview.unsupported_components.length > 0 && (
            <div className="flex items-start gap-2 rounded-md bg-amber-500/10 p-3 text-sm">
              <InformationCircleIcon className="h-4 w-4 mt-0.5 shrink-0 text-amber-600" />
              <div>
                <span className="font-medium">Unsupported components: </span>
                {preview.unsupported_components.map((c) => (
                  <Badge key={c.name} variant="secondary" className="mx-0.5 text-xs">
                    {c.name} ({c.count})
                  </Badge>
                ))}
              </div>
            </div>
          )}

          {preview.broken_links.length > 0 && (
            <div className="flex items-start gap-2 rounded-md bg-destructive/10 p-3 text-sm">
              <AlertCircleIcon className="h-4 w-4 mt-0.5 shrink-0 text-destructive" />
              <span>{preview.broken_links.length} broken internal link{preview.broken_links.length > 1 ? 's' : ''} detected</span>
            </div>
          )}

          {preview.warnings.length > 0 && (
            <details className="text-xs text-muted-foreground">
              <summary className="cursor-pointer">{preview.warnings.length} warning{preview.warnings.length > 1 ? 's' : ''}</summary>
              <ul className="mt-2 space-y-1 pl-4 list-disc max-h-32 overflow-y-auto">
                {preview.warnings.slice(0, 30).map((w, i) => (
                  <li key={i}>{w.message}</li>
                ))}
                {preview.warnings.length > 30 && <li>...and {preview.warnings.length - 30} more</li>}
              </ul>
            </details>
          )}

          {/* Target space */}
          <div className="space-y-2">
            <Label>Target Space</Label>
            <Select value={targetSpaceId} onValueChange={setTargetSpaceId}>
              <SelectTrigger className="w-full">
                <SelectValue placeholder="Select a space or create new" />
              </SelectTrigger>
              <SelectContent>
                {spaces?.filter((s) => s.type === 'external_capable').map((s) => (
                  <SelectItem key={s.id} value={s.id}>{s.name}</SelectItem>
                ))}
                <SelectItem value="__new">Create new space</SelectItem>
              </SelectContent>
            </Select>
            {targetSpaceId === '__new' && (
              <Input
                placeholder="New space name"
                value={newSpaceName}
                onChange={(e) => setNewSpaceName(e.target.value)}
                autoFocus
              />
            )}
          </div>

          {/* Import status */}
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
              {starting ? <><Loading01Icon className="h-4 w-4 animate-spin mr-1.5" />Starting...</> : 'Start Import'}
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

            {/* ETA */}
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

          {/* Completion details */}
          {isFinished && jobStatus && (
            <Card>
              <CardContent className="pt-4 space-y-3">
                {jobStatus.summary && (
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
      <ImportHistory workspaceId={workspaceId} source="nextra" />
    </div>
  );
}
