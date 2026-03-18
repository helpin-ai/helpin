import { useEffect, useRef, useState } from 'react';
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
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { cn } from '@/lib/utils';
import { toast } from 'sonner';
import { Check } from 'lucide-react';
import { useDocsSpaces } from '@/hooks/queries';
import {
  docsImportService,
  type ImportPreviewResponse,
  type ImportStatusResponse,
} from '@/lib/services/docsImportService';

type WizardStep = 0 | 1 | 2;
const STEP_LABELS = ['Connect', 'Configure', 'Import'];

export function HelpCenterImportSection({
  workspaceId,
  editable,
}: {
  workspaceId: string;
  editable: boolean;
}) {
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
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const { data: spaces } = useDocsSpaces(workspaceId);

  useEffect(() => {
    return () => {
      if (pollRef.current) clearInterval(pollRef.current);
    };
  }, []);

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
      startPolling(data.job_id);
    }
  };

  const startPolling = (id: string) => {
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

  const handleDownloadRedirectMap = () => {
    const url = docsImportService.getRedirectMapUrl(workspaceId, jobId);
    const token = localStorage.getItem('access_token');
    window.open(`${url}&token=${encodeURIComponent(token || '')}`, '_blank');
  };

  const handleReset = () => {
    if (pollRef.current) clearInterval(pollRef.current);
    pollRef.current = null;
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

  const selectedCollectionData = preview?.collections.find((c) => c.id === selectedCollection);
  const isFinished = jobStatus?.status === 'done' || jobStatus?.status === 'failed';
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
      <div className="flex items-center gap-2">
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
        <div className="space-y-4">
          <div className="space-y-2">
            <Label>Source</Label>
            <Select value="helpscout" disabled>
              <SelectTrigger className="w-full">
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
              Find your API key in HelpScout → Manage → API Keys
            </p>
          </div>
          <div className="flex justify-end">
            <Button onClick={handleConnect} disabled={!editable || connecting || !apiKey.trim()}>
              {connecting ? 'Connecting...' : 'Connect & Preview'}
            </Button>
          </div>
        </div>
      )}

      {/* Step 1: Configure */}
      {step === 1 && (
        <div className="space-y-4">
          {selectedCollectionData && (
            <Card>
              <CardHeader className="pb-2">
                <CardTitle className="text-sm font-medium">{selectedCollectionData.name}</CardTitle>
              </CardHeader>
              <CardContent>
                <p className="text-sm text-muted-foreground">
                  Found {selectedCollectionData.category_count} categories and {selectedCollectionData.article_count} articles
                </p>
              </CardContent>
            </Card>
          )}

          {preview && preview.collections.length > 1 && (
            <div className="space-y-2">
              <Label>Collection</Label>
              <Select value={selectedCollection} onValueChange={setSelectedCollection}>
                <SelectTrigger className="w-full">
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
        <div className="space-y-4">
          <div className="space-y-2">
            <Progress value={progressPercent} className="h-2" />
            <p className="text-sm text-muted-foreground">
              {isFinished
                ? 'Import complete'
                : `Importing... ${jobStatus ? jobStatus.completed + jobStatus.failed : 0} of ${jobStatus?.total ?? '...'} articles`}
            </p>
          </div>

          {isFinished && jobStatus && (
            <Card>
              <CardContent className="pt-4 space-y-3">
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
    </div>
  );
}
