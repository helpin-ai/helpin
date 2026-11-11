import { useId, useRef, useState, type ReactNode } from 'react'
import { useQuery } from '@tanstack/react-query'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Progress } from '@/components/ui/progress'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { useDocsSpaces } from '@/hooks/queries'
import { useWorkspaceAccess } from '@/hooks/queries/useSession'
import { Loading01Icon } from '@/lib/icons'
import { unwrap } from '@/lib/queryUtils'
import {
  docsImportService,
  type ImportStatusResponse,
  type ImportSummary,
  type PublicHelpCenterPreview,
} from '@/lib/services/docsImportService'
import { useWorkspaceStore } from '@/stores/workspaceStore'

type ArticleStatus = 'published' | 'draft'

const ARTICLE_STATUS_OPTIONS: { value: ArticleStatus; label: string; description: string }[] = [
  { value: 'published', label: 'Publish', description: 'Articles go live on your Help Center as they are imported.' },
  { value: 'draft', label: 'Keep as drafts', description: 'Nothing goes live until you publish it.' },
]

const NEW_SPACE = '__new'
const FINISHED_STATUSES = ['done', 'failed', 'interrupted']

function plural(count: number, one: string, many: string): string {
  return `${count.toLocaleString()} ${count === 1 ? one : many}`
}

/** Shows a web address the way people say it: no scheme, no trailing slash. */
function displayAddress(url: string): string {
  return url.replace(/^https?:\/\//, '').replace(/\/$/, '')
}

interface PublicHelpCenterImportWizardProps {
  workspaceId: string
  /** An import of this kind that is already running; its progress is shown instead of the form. */
  activeJobId?: string
}

/** Copies a public Intercom help center into a Help Center space, by its address. */
export function PublicHelpCenterImportWizard({ workspaceId, activeJobId }: PublicHelpCenterImportWizardProps) {
  const { data: access, isLoading: accessLoading } = useWorkspaceAccess(workspaceId)
  const canImport = access?.permissions?.includes('docs.import') ?? false
  const { data: spaces } = useDocsSpaces(workspaceId)
  const helpCenterSpaces = (spaces ?? []).filter((space) => space.type === 'external_capable')

  const [address, setAddress] = useState('')
  const [checking, setChecking] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [preview, setPreview] = useState<PublicHelpCenterPreview | null>(null)
  // The address the preview was read from; editing the field makes it stale.
  const [checkedAddress, setCheckedAddress] = useState('')
  const [targetSpaceId, setTargetSpaceId] = useState(NEW_SPACE)
  const [newSpaceName, setNewSpaceName] = useState('')
  const [articleStatus, setArticleStatus] = useState<ArticleStatus>('published')
  const [starting, setStarting] = useState(false)
  const [startError, setStartError] = useState<string | null>(null)
  const [jobId, setJobId] = useState<string | null>(activeJobId ?? null)
  const addressRef = useRef<HTMLInputElement>(null)
  const checkSeq = useRef(0)
  const errorId = useId()

  // A running import found after this screen opened takes over the form,
  // unless the form has already started one. The job stays on screen after it
  // finishes and drops out of the running list, so its result can be read.
  const [seenActiveJobId, setSeenActiveJobId] = useState(activeJobId)
  if (activeJobId !== seenActiveJobId) {
    setSeenActiveJobId(activeJobId)
    if (activeJobId && !jobId) setJobId(activeJobId)
  }

  const trimmedAddress = address.trim()
  const previewIsCurrent = Boolean(preview) && trimmedAddress === checkedAddress
  const spaceName = newSpaceName.trim()

  const check = async () => {
    if (!trimmedAddress || checking) return
    const seq = ++checkSeq.current
    setChecking(true)
    setError(null)
    setStartError(null)
    const response = await docsImportService.previewPublicHelpCenter(workspaceId, trimmedAddress)
    if (seq !== checkSeq.current) return
    setChecking(false)
    if (!response.data) {
      setPreview(null)
      setError(response.error ?? 'The help center could not be read. Try again in a moment.')
      // Put the cursor back where the fix happens.
      addressRef.current?.focus()
      return
    }
    setPreview(response.data)
    setCheckedAddress(trimmedAddress)
    setNewSpaceName((current) => current.trim() || response.data!.name || 'Help Center')
  }

  const start = async () => {
    if (!preview || starting) return
    setStarting(true)
    setStartError(null)
    const response = await docsImportService.startPublicHelpCenter(workspaceId, {
      url: preview.url,
      import_status: articleStatus,
      ...(targetSpaceId === NEW_SPACE ? { new_space_name: spaceName || 'Help Center' } : { target_space_id: targetSpaceId }),
    })
    setStarting(false)
    if (response.error || !response.data?.job_id) {
      setStartError(response.error ?? 'The import could not be started. Try again in a moment.')
      return
    }
    setJobId(response.data.job_id)
  }

  const reset = () => {
    setJobId(null)
    setPreview(null)
    setCheckedAddress('')
    setAddress('')
    setNewSpaceName('')
    setError(null)
    setStartError(null)
  }

  if (jobId) {
    return <PublicHelpCenterImportProgress workspaceId={workspaceId} jobId={jobId} sourceName={preview?.name || undefined} onImportAnother={reset} />
  }

  if (accessLoading) return null
  if (!canImport) {
    return <p className="text-sm text-muted-foreground">You need permission to import Help Center articles to use this import.</p>
  }

  return (
    <div className="space-y-6">
      <div>
        <h3 className="text-base font-semibold">Import a public help center</h3>
        <p className="mt-1 text-sm text-muted-foreground">
          Copy an Intercom help center by its address. No access token is needed, and running the import again skips what was already imported.
        </p>
      </div>

      <form
        className="rounded-lg border border-border p-4"
        noValidate
        onSubmit={(event) => {
          event.preventDefault()
          void check()
        }}
      >
        <p className="sr-only" role="status">{checking ? 'Reading the help center. This can take a few seconds.' : ''}</p>
        <div className="space-y-1.5">
          <Label htmlFor="public-help-center-address">Help center address</Label>
          <div className="flex flex-col gap-2 sm:flex-row">
            <Input
              ref={addressRef}
              id="public-help-center-address"
              type="url"
              inputMode="url"
              autoComplete="off"
              autoCapitalize="none"
              spellCheck={false}
              value={address}
              onChange={(event) => {
                setAddress(event.target.value)
                if (error) setError(null)
              }}
              placeholder="https://help.example.com"
              aria-invalid={error ? true : undefined}
              aria-describedby={error ? errorId : 'public-help-center-address-hint'}
              className="sm:flex-1"
            />
            <Button type="submit" variant={previewIsCurrent ? 'outline' : 'default'} disabled={!trimmedAddress || checking} className="shrink-0">
              {checking ? <Loading01Icon className="mr-2 h-4 w-4 animate-spin" /> : null}
              {checking ? 'Reading…' : previewIsCurrent ? 'Check again' : 'Check address'}
            </Button>
          </div>
          {error ? (
            <p id={errorId} role="alert" className="text-sm text-destructive">{error}</p>
          ) : (
            <p id="public-help-center-address-hint" className="text-xs text-muted-foreground">
              The home page of the help center, or any article in it. Only public articles in its main language are imported.
            </p>
          )}
        </div>
      </form>

      {preview && previewIsCurrent ? (
        <div className="space-y-4">
          <section className="space-y-3 rounded-lg border border-border p-4" aria-labelledby="public-help-center-found" data-testid="public-help-center-preview">
            <div>
              <p id="public-help-center-found" className="text-sm font-medium">{preview.name || displayAddress(preview.url)}</p>
              <p className="text-sm text-muted-foreground">
                {plural(preview.articles, 'article', 'articles')} in {plural(preview.collections, 'collection', 'collections')} at {displayAddress(preview.url)}
              </p>
            </div>
            <CollectionCounts collections={preview.top_collections} />
            <p className="text-xs text-muted-foreground">
              Images are copied, links between articles are pointed at their new addresses, and the old addresses redirect. Public articles that sit in no collection are found through links and placed in “Other articles”.
              {preview.collections_merged > 0
                ? ` ${plural(preview.collections_merged, 'collection is nested deeper than Helpin supports; its', 'collections are nested deeper than Helpin supports; their')} articles move into the collection above.`
                : ''}
            </p>
          </section>

          <section className="grid gap-3 rounded-lg border border-border p-4 sm:grid-cols-2">
            <div className="space-y-1.5">
              <Label htmlFor="public-help-center-space">Space</Label>
              <Select value={targetSpaceId} onValueChange={setTargetSpaceId}>
                <SelectTrigger id="public-help-center-space" className="w-full"><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value={NEW_SPACE}>New space</SelectItem>
                  {helpCenterSpaces.map((space) => <SelectItem key={space.id} value={space.id}>{space.name}</SelectItem>)}
                </SelectContent>
              </Select>
            </div>
            {targetSpaceId === NEW_SPACE ? (
              <div className="space-y-1.5">
                <Label htmlFor="public-help-center-space-name">Space name</Label>
                <Input id="public-help-center-space-name" value={newSpaceName} onChange={(event) => setNewSpaceName(event.target.value)} maxLength={100} />
              </div>
            ) : null}
            <div className="space-y-1.5 sm:col-span-2">
              <Label htmlFor="public-help-center-status">Imported articles</Label>
              <Select value={articleStatus} onValueChange={(value) => setArticleStatus(value as ArticleStatus)}>
                <SelectTrigger id="public-help-center-status" className="w-full sm:w-64"><SelectValue /></SelectTrigger>
                <SelectContent>
                  {ARTICLE_STATUS_OPTIONS.map((option) => <SelectItem key={option.value} value={option.value}>{option.label}</SelectItem>)}
                </SelectContent>
              </Select>
              <p className="text-xs text-muted-foreground">{ARTICLE_STATUS_OPTIONS.find((option) => option.value === articleStatus)?.description}</p>
            </div>
          </section>

          <div className="flex flex-col items-end gap-2">
            {startError ? <p role="alert" className="text-sm text-destructive">{startError}</p> : null}
            <Button onClick={() => void start()} disabled={starting || (targetSpaceId === NEW_SPACE && !spaceName)}>
              {starting ? <Loading01Icon className="mr-2 h-4 w-4 animate-spin" /> : null}
              Import {plural(preview.articles, 'article', 'articles')}
            </Button>
          </div>
        </div>
      ) : null}
    </div>
  )
}

/** The top-level collections of a help center with their article counts. */
function CollectionCounts({ collections }: { collections: PublicHelpCenterPreview['top_collections'] }) {
  const [expanded, setExpanded] = useState(false)
  if (collections.length === 0) return null
  const limit = 6
  const visible = expanded ? collections : collections.slice(0, limit)
  return (
    <div>
      <ul className="divide-y divide-border border-y border-border text-sm">
        {visible.map((collection, index) => (
          <li key={`${collection.name}-${index}`} className="flex items-baseline justify-between gap-4 py-1.5">
            <span className="min-w-0 truncate">{collection.name}</span>
            <span className="shrink-0 tabular-nums text-muted-foreground">
              {collection.articles.toLocaleString()}
              <span className="sr-only"> {collection.articles === 1 ? 'article' : 'articles'}</span>
            </span>
          </li>
        ))}
      </ul>
      {collections.length > limit ? (
        <button type="button" className="mt-2 text-xs text-muted-foreground underline underline-offset-2 hover:text-foreground" onClick={() => setExpanded((value) => !value)} aria-expanded={expanded}>
          {expanded ? 'Show fewer' : `Show all ${collections.length} top-level collections`}
        </button>
      ) : null}
    </div>
  )
}

function percent(done: number, total: number): number {
  return total > 0 ? Math.min(100, Math.round((done / total) * 100)) : 0
}

/** One line that says where a running import is. */
function progressLine(job: ImportStatusResponse | undefined): string {
  if (!job || job.status === 'pending') return 'Starting…'
  if (job.total === 0) return 'Reading the help center…'
  const done = job.completed + job.failed
  if (done >= job.total) return 'Updating links between articles and restoring their order…'
  return `${done.toLocaleString()} of ${plural(job.total, 'article', 'articles')}`
}

interface PublicHelpCenterImportProgressProps {
  workspaceId: string
  jobId: string
  /** The help center's name when this screen started the import; a resumed job reads it from the job. */
  sourceName?: string
  onImportAnother: () => void
}

function PublicHelpCenterImportProgress({ workspaceId, jobId, sourceName: knownSourceName, onImportAnother }: PublicHelpCenterImportProgressProps) {
  const workspaceSlug = useWorkspaceStore((state) => state.currentWorkspace?.slug)
  const [acting, setActing] = useState<'stop' | 'resume' | null>(null)
  const jobQuery = useQuery({
    queryKey: ['public-help-center-import', workspaceId, jobId],
    queryFn: async () => unwrap(await docsImportService.getStatus(workspaceId, jobId)),
    refetchInterval: (query) => (query.state.data && FINISHED_STATUSES.includes(query.state.data.status) ? false : 3000),
  })
  const job = jobQuery.data ?? undefined
  const running = !job || job.status === 'pending' || job.status === 'running'
  const summary = job?.summary ?? null
  const sourceAddress = summary?.source_url || job?.config?.source_url
  const sourceName = summary?.source_name || knownSourceName || (sourceAddress ? displayAddress(sourceAddress) : 'the help center')
  const spaceHref = workspaceSlug && job?.space_id ? `/w/${workspaceSlug}/docs/spaces/${job.space_id}` : null

  const stop = async () => {
    setActing('stop')
    const response = await docsImportService.cancel(workspaceId, jobId)
    setActing(null)
    if (response.error) toast.error('The import could not be stopped', { description: response.error })
    else void jobQuery.refetch()
  }

  const resume = async () => {
    setActing('resume')
    const response = await docsImportService.resumePublicHelpCenter(workspaceId, jobId)
    setActing(null)
    if (response.error) toast.error('The import could not be continued', { description: response.error })
    else void jobQuery.refetch()
  }

  const canResume = job && (job.status === 'failed' || job.status === 'interrupted' || (job.status === 'done' && job.failed > 0))
  const resumeLabel = job?.status === 'interrupted' ? 'Continue import' : job?.status === 'failed' ? 'Try again' : 'Retry failed articles'

  return (
    <div className="space-y-4">
      <div>
        <h3 className="text-base font-semibold">{running ? `Importing ${sourceName}` : `Import of ${sourceName}`}</h3>
        {running ? <p className="mt-1 text-sm text-muted-foreground">You can leave this page; the import keeps running.</p> : null}
      </div>

      <section className="space-y-3 rounded-lg border border-border p-4" data-testid="public-help-center-progress">
        {running ? (
          <>
            <div className="flex items-center justify-between gap-3 text-sm">
              <span role="status" aria-live="polite">{progressLine(job)}</span>
              <Button variant="ghost" size="sm" onClick={() => void stop()} disabled={acting !== null || !job}>
                {acting === 'stop' ? 'Stopping…' : 'Stop import'}
              </Button>
            </div>
            <Progress value={job ? percent(job.completed + job.failed, job.total) : 0} aria-label="Import progress" />
          </>
        ) : null}

        {jobQuery.isError ? <p role="alert" className="text-sm text-destructive">The import status could not be loaded. It will be checked again shortly.</p> : null}

        {job?.status === 'done' ? (
          <div role="status" className="space-y-2">
            <p className="text-sm">
              <span className="font-medium">
                {job.completed > 0 ? `${plural(job.completed, 'article', 'articles')} imported` : 'No new articles to import'}
              </span>
              {job.failed > 0 ? <span className="text-destructive">, {job.failed.toLocaleString()} failed</span> : null}
              .
            </p>
            <ImportOutcome summary={summary} />
          </div>
        ) : null}
        {job?.status === 'failed' ? (
          <p role="alert" className="text-sm text-destructive">
            {job.error || 'The import failed before any article was imported.'}
          </p>
        ) : null}
        {job?.status === 'interrupted' ? (
          <p role="status" className="text-sm text-muted-foreground">
            Stopped after {plural(job.completed, 'article', 'articles')}. What was imported stays, and continuing picks up where it stopped.
          </p>
        ) : null}

        {job && !running ? <ImportAttention workspaceSlug={workspaceSlug} job={job} summary={summary} /> : null}

        {job && !running ? (
          <div className="flex flex-wrap items-center gap-2 pt-1">
            {spaceHref && job.completed > 0 ? <Button asChild size="sm"><a href={spaceHref}>Open the space</a></Button> : null}
            {canResume ? (
              <Button variant="outline" size="sm" onClick={() => void resume()} disabled={acting !== null}>
                {acting === 'resume' ? <Loading01Icon className="mr-2 h-4 w-4 animate-spin" /> : null}
                {resumeLabel}
              </Button>
            ) : null}
            <Button variant="ghost" size="sm" onClick={onImportAnother}>Import another help center</Button>
          </div>
        ) : null}
      </section>
    </div>
  )
}

/** What a finished import did, one fact per line, only the ones that happened. */
function ImportOutcome({ summary }: { summary: ImportSummary | null }) {
  if (!summary) return null
  const lines: string[] = []
  if (summary.articles_drafted > 0 && summary.articles_published === 0) lines.push('They are drafts; nothing is live yet.')
  if (summary.images_copied) lines.push(`${plural(summary.images_copied, 'image', 'images')} copied.`)
  if (summary.links_rewritten) lines.push(`${plural(summary.links_rewritten, 'link', 'links')} between articles now ${summary.links_rewritten === 1 ? 'points' : 'point'} to the new addresses.`)
  if (summary.redirects_created) lines.push(`${plural(summary.redirects_created, 'old address redirects', 'old addresses redirect')} to the imported pages.`)
  if (summary.articles_outside_collections) lines.push(`${plural(summary.articles_outside_collections, 'article that was', 'articles that were')} in no collection went into “Other articles”.`)
  if (summary.collections_merged) lines.push(`${plural(summary.collections_merged, 'deeply nested collection was', 'deeply nested collections were')} merged into the collection above.`)
  if (lines.length === 0) return null
  return (
    <ul className="space-y-0.5 text-sm text-muted-foreground">
      {lines.map((line) => <li key={line}>{line}</li>)}
    </ul>
  )
}

interface ImportAttentionProps {
  workspaceSlug?: string
  job: ImportStatusResponse
  summary: ImportSummary | null
}

/** Everything a finished import left for a person to look at. Renders nothing when there is nothing. */
function ImportAttention({ workspaceSlug, job, summary }: ImportAttentionProps) {
  const failures = job.failures ?? []
  const unresolved = summary?.unresolved_links ?? []
  const unresolvedTotal = summary?.unresolved_links_total ?? unresolved.length
  const failedImages = summary?.failed_images ?? []
  const failedImagesTotal = summary?.failed_images_total ?? failedImages.length
  if (failures.length === 0 && unresolvedTotal === 0 && failedImagesTotal === 0) return null

  const articleLink = (documentId: string | undefined, title: string) => {
    const label = title || 'Untitled article'
    return workspaceSlug && documentId
      ? <a className="font-medium text-foreground underline underline-offset-2" href={`/w/${workspaceSlug}/docs/documents/${documentId}`}>{label}</a>
      : <span className="font-medium text-foreground">{label}</span>
  }

  return (
    <div className="space-y-2 border-t border-border pt-3" data-testid="public-help-center-attention">
      <p className="text-sm font-medium">Needs a look</p>
      {failures.length > 0 ? (
        <AttentionList title={`${plural(job.failed, 'article was', 'articles were')} not imported`} shown={failures.length} total={job.failed} open>
          {failures.map((failure) => (
            <li key={failure.article_id}>
              <span className="font-medium text-foreground">{failure.title || `Article ${failure.article_id}`}</span>: {failure.error}
            </li>
          ))}
        </AttentionList>
      ) : null}
      {unresolvedTotal > 0 ? (
        <AttentionList
          title={`${plural(unresolvedTotal, 'link still points', 'links still point')} at the original help center`}
          hint={unresolvedTotal === 1 ? 'Its target was not found there, so it was left as written.' : 'Their targets were not found there, so they were left as written.'}
          shown={unresolved.length}
          total={unresolvedTotal}
        >
          {unresolved.map((link, index) => (
            <li key={`${link.document_id}-${link.url}-${index}`} className="break-words">
              {articleLink(link.document_id, link.article_title)}: <ExternalAddress url={link.url} />
            </li>
          ))}
        </AttentionList>
      ) : null}
      {failedImagesTotal > 0 ? (
        <AttentionList
          title={`${plural(failedImagesTotal, 'image was', 'images were')} not copied`}
          hint={failedImagesTotal === 1 ? 'The article still shows it from the original help center.' : 'The articles still show them from the original help center.'}
          shown={failedImages.length}
          total={failedImagesTotal}
        >
          {failedImages.map((image, index) => (
            <li key={`${image.document_id ?? ''}-${image.url}-${index}`} className="break-words">
              {articleLink(image.document_id, image.article_title)}: <ExternalAddress url={image.url} />
              {image.reason ? <span> ({image.reason})</span> : null}
            </li>
          ))}
        </AttentionList>
      ) : null}
    </div>
  )
}

interface AttentionListProps {
  title: string
  hint?: string
  shown: number
  total: number
  open?: boolean
  children: ReactNode
}

function AttentionList({ title, hint, shown, total, open, children }: AttentionListProps) {
  return (
    <details className="text-sm text-muted-foreground" open={open}>
      <summary className="cursor-pointer text-foreground">{title}</summary>
      {hint ? <p className="mt-1 text-xs">{hint}</p> : null}
      <ul className="mt-1.5 max-h-64 space-y-1 overflow-y-auto pr-1 text-xs">{children}</ul>
      {total > shown ? <p className="mt-1.5 text-xs">Showing the first {shown.toLocaleString()} of {total.toLocaleString()}.</p> : null}
    </details>
  )
}

function ExternalAddress({ url }: { url: string }) {
  return (
    <a className="underline underline-offset-2" href={url} target="_blank" rel="noopener noreferrer">
      {displayAddress(url)}
    </a>
  )
}
