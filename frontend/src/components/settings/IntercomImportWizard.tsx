import { useRef, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Progress } from '@/components/ui/progress'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { useDocsSpaces } from '@/hooks/queries'
import { useWorkspaceAccess } from '@/hooks/queries/useSession'
import { useInboxScopes } from '@/hooks/queries/useSupport'
import { Loading01Icon } from '@/lib/icons'
import { unwrap } from '@/lib/queryUtils'
import { docsImportService } from '@/lib/services/docsImportService'
import {
  intercomImportService,
  type IntercomConversationsPreview,
  type IntercomDocsPreview,
  type IntercomRegion,
  type SupportImportJob,
} from '@/lib/services/intercomImportService'

function plural(count: number, one: string, many: string): string {
  return `${count} ${count === 1 ? one : many}`
}

function oneYearAgo(): string {
  const date = new Date()
  date.setFullYear(date.getFullYear() - 1)
  return date.toISOString().slice(0, 10)
}

const IMPORT_STATUS_OPTIONS = [
  { value: 'match_source', label: 'Match Intercom', description: 'Published articles stay published, drafts stay drafts.' },
  { value: 'draft', label: 'All as drafts', description: 'Review every article before it goes live.' },
  { value: 'published', label: 'Publish all', description: 'Drafts are published too.' },
]

interface Started {
  docsJobId?: string
  conversationJobId?: string
}

/** Imports an Intercom Help Center and conversations. */
export function IntercomImportWizard({ workspaceId }: { workspaceId: string }) {
  const { data: access } = useWorkspaceAccess(workspaceId)
  const canImportDocs = access?.permissions?.includes('docs.import') ?? false
  const canImportConversations = access?.permissions?.includes('support.admin') ?? false
  const { data: spaces } = useDocsSpaces(workspaceId)
  const { data: scopes } = useInboxScopes(workspaceId)

  const [token, setToken] = useState('')
  const [region, setRegion] = useState<IntercomRegion>('us')
  const [checking, setChecking] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [docsPreview, setDocsPreview] = useState<IntercomDocsPreview | null>(null)
  const [docsError, setDocsError] = useState<string | null>(null)
  const [conversationsError, setConversationsError] = useState<string | null>(null)
  const recountSeq = useRef(0)
  const [conversationsPreview, setConversationsPreview] = useState<IntercomConversationsPreview | null>(null)
  const [importDocs, setImportDocs] = useState(true)
  const [importConversations, setImportConversations] = useState(true)
  const [targetSpaceId, setTargetSpaceId] = useState('__new')
  const [newSpaceName, setNewSpaceName] = useState('Help Center')
  const [importStatus, setImportStatus] = useState('match_source')
  const [since, setSince] = useState(oneYearAgo)
  const [mailboxId, setMailboxId] = useState('shared')
  const [starting, setStarting] = useState(false)
  const [started, setStarted] = useState<Started | null>(null)

  const connected = Boolean(docsPreview || conversationsPreview)
  const workspaceName = docsPreview?.workspace_name ?? conversationsPreview?.workspace_name ?? 'Intercom'
  const helpCenterSpaces = (spaces ?? []).filter((space) => space.type === 'external_capable')

  const check = async () => {
    setChecking(true)
    setError(null)
    const [docs, conversations] = await Promise.all([
      canImportDocs ? intercomImportService.previewDocs(workspaceId, token.trim(), region) : Promise.resolve(null),
      canImportConversations ? intercomImportService.previewConversations(workspaceId, token.trim(), region, since) : Promise.resolve(null),
    ])
    setChecking(false)
    // Each part is checked on its own: a token may read conversations but
    // not articles, or the other way round.
    setDocsPreview(docs?.data ?? null)
    setConversationsPreview(conversations?.data ?? null)
    setDocsError(docs?.error ?? null)
    setConversationsError(conversations?.error ?? null)
    if (!docs?.data && !conversations?.data) {
      setError(docs?.error ?? conversations?.error ?? 'Intercom could not be reached.')
    }
  }

  const recount = async (value: string) => {
    setSince(value)
    if (!canImportConversations || !value) return
    const seq = ++recountSeq.current
    const response = await intercomImportService.previewConversations(workspaceId, token.trim(), region, value)
    if (seq !== recountSeq.current) return
    if (response.data) setConversationsPreview(response.data)
    else if (response.error) toast.error(response.error)
  }

  const start = async () => {
    setStarting(true)
    const result: Started = {}
    if (canImportDocs && importDocs && docsPreview) {
      const response = await intercomImportService.startDocs(workspaceId, {
        access_token: token.trim(), region, import_status: importStatus,
        ...(targetSpaceId === '__new' ? { new_space_name: newSpaceName.trim() || 'Help Center' } : { target_space_id: targetSpaceId }),
      })
      if (response.error) toast.error('Help Center import did not start', { description: response.error })
      else result.docsJobId = response.data?.job_id
    }
    if (canImportConversations && importConversations && conversationsPreview) {
      const response = await intercomImportService.startConversations(workspaceId, {
        access_token: token.trim(), region, since, ...(mailboxId !== 'shared' ? { mailbox_id: mailboxId } : {}),
      })
      if (response.error) toast.error('Conversation import did not start', { description: response.error })
      else result.conversationJobId = response.data?.id
    }
    setStarting(false)
    if (result.docsJobId || result.conversationJobId) {
      setToken('')
      setStarted(result)
    }
  }

  if (started) return <IntercomImportProgress workspaceId={workspaceId} started={started} />

  if (!canImportDocs && !canImportConversations) {
    return <p className="text-sm text-muted-foreground">You need permission to import Help Center articles or manage support to import from Intercom.</p>
  }

  return (
    <div className="space-y-6">
      <div>
        <h3 className="text-base font-semibold">Import from Intercom</h3>
        <p className="mt-1 text-sm text-muted-foreground">
          Bring over your Help Center and past conversations. Running an import again skips what was already imported.
        </p>
      </div>

      <form
        className="space-y-3 rounded-lg border border-border p-4"
        onSubmit={(event) => {
          event.preventDefault()
          void check()
        }}
      >
        <div className="space-y-1.5">
          <Label htmlFor="intercom-token">Access token</Label>
          <Input id="intercom-token" type="password" autoComplete="off" value={token} onChange={(event) => setToken(event.target.value)} placeholder="dG9rOm…" />
          <p className="text-xs text-muted-foreground">
            In Intercom, open Settings → Integrations → Developer Hub, create an app, and copy its access token. It needs read access to articles, conversations and contacts. The token is stored encrypted until the import finishes.
          </p>
        </div>
        <div className="flex flex-wrap items-end gap-3">
          <div className="space-y-1.5">
            <Label htmlFor="intercom-region">Data region</Label>
            <Select value={region} onValueChange={(value) => setRegion(value as IntercomRegion)}>
              <SelectTrigger id="intercom-region" className="w-40"><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem value="us">United States</SelectItem>
                <SelectItem value="eu">Europe</SelectItem>
                <SelectItem value="au">Australia</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <Button type="submit" variant={connected ? 'outline' : 'default'} disabled={!token.trim() || checking}>
            {checking ? <Loading01Icon className="mr-2 h-4 w-4 animate-spin" /> : null}
            {connected ? 'Check again' : 'Connect'}
          </Button>
        </div>
        {error ? <p role="alert" className="text-sm text-destructive">{error}</p> : null}
      </form>

      {connected ? (
        <div className="space-y-4">
          <p className="text-sm">Connected to <span className="font-medium">{workspaceName}</span>. Choose what to import.</p>

          {docsPreview ? (
            <section className="space-y-3 rounded-lg border border-border p-4">
              <label className="flex items-start gap-3">
                <Checkbox checked={importDocs} onCheckedChange={(checked) => setImportDocs(checked === true)} className="mt-0.5" />
                <span>
                  <span className="block text-sm font-medium">Help Center</span>
                  <span className="block text-sm text-muted-foreground">
                    {plural(docsPreview.collections, 'collection', 'collections')}, {docsPreview.articles_published} published and {plural(docsPreview.articles_draft, 'draft article', 'draft articles')}. Images are copied and old article links redirect.
                  </span>
                </span>
              </label>
              {importDocs ? (
                <div className="grid gap-3 pl-7 sm:grid-cols-2">
                  <div className="space-y-1.5">
                    <Label htmlFor="intercom-space">Space</Label>
                    <Select value={targetSpaceId} onValueChange={setTargetSpaceId}>
                      <SelectTrigger id="intercom-space"><SelectValue /></SelectTrigger>
                      <SelectContent>
                        <SelectItem value="__new">New space</SelectItem>
                        {helpCenterSpaces.map((space) => <SelectItem key={space.id} value={space.id}>{space.name}</SelectItem>)}
                      </SelectContent>
                    </Select>
                  </div>
                  {targetSpaceId === '__new' ? (
                    <div className="space-y-1.5">
                      <Label htmlFor="intercom-space-name">Space name</Label>
                      <Input id="intercom-space-name" value={newSpaceName} onChange={(event) => setNewSpaceName(event.target.value)} maxLength={100} />
                    </div>
                  ) : null}
                  <div className="space-y-1.5 sm:col-span-2">
                    <Label htmlFor="intercom-status">Article status</Label>
                    <Select value={importStatus} onValueChange={setImportStatus}>
                      <SelectTrigger id="intercom-status" className="sm:w-64"><SelectValue /></SelectTrigger>
                      <SelectContent>
                        {IMPORT_STATUS_OPTIONS.map((option) => <SelectItem key={option.value} value={option.value}>{option.label}</SelectItem>)}
                      </SelectContent>
                    </Select>
                    <p className="text-xs text-muted-foreground">{IMPORT_STATUS_OPTIONS.find((option) => option.value === importStatus)?.description}</p>
                  </div>
                </div>
              ) : null}
            </section>
          ) : null}

          {!docsPreview && docsError ? (
            <p className="rounded-lg border border-border p-4 text-sm text-muted-foreground" data-testid="intercom-docs-unavailable">
              <span className="font-medium text-foreground">Help Center</span> cannot be imported with this token: {docsError}
            </p>
          ) : null}

          {conversationsPreview ? (
            <section className="space-y-3 rounded-lg border border-border p-4">
              <label className="flex items-start gap-3">
                <Checkbox checked={importConversations} onCheckedChange={(checked) => setImportConversations(checked === true)} className="mt-0.5" />
                <span>
                  <span className="block text-sm font-medium">Conversations</span>
                  <span className="block text-sm text-muted-foreground">
                    {plural(conversationsPreview.conversations, 'conversation', 'conversations')} with their replies, internal notes and tags. Attachments are listed by name; files and assignments are not copied.
                  </span>
                </span>
              </label>
              {importConversations ? (
                <div className="grid gap-3 pl-7 sm:grid-cols-2">
                  <div className="space-y-1.5">
                    <Label htmlFor="intercom-since">Created on or after</Label>
                    <Input id="intercom-since" type="date" value={since} max={new Date().toISOString().slice(0, 10)} onChange={(event) => void recount(event.target.value)} />
                  </div>
                  <div className="space-y-1.5">
                    <Label htmlFor="intercom-inbox">Inbox</Label>
                    <Select value={mailboxId} onValueChange={setMailboxId}>
                      <SelectTrigger id="intercom-inbox"><SelectValue /></SelectTrigger>
                      <SelectContent>
                        <SelectItem value="shared">{scopes?.shared_inbox?.name ?? 'Main inbox'}</SelectItem>
                        {(scopes?.mailboxes ?? []).map((mailbox) => <SelectItem key={mailbox.id} value={mailbox.id}>{mailbox.name}</SelectItem>)}
                      </SelectContent>
                    </Select>
                  </div>
                  <p className="text-xs text-muted-foreground sm:col-span-2">
                    Open conversations arrive open and already read. Imported history gets no SLA targets and sends no webhooks.
                  </p>
                </div>
              ) : null}
            </section>
          ) : null}

          {!conversationsPreview && conversationsError ? (
            <p className="rounded-lg border border-border p-4 text-sm text-muted-foreground" data-testid="intercom-conversations-unavailable">
              <span className="font-medium text-foreground">Conversations</span> cannot be imported with this token: {conversationsError}
            </p>
          ) : null}

          <div className="flex justify-end">
            <Button
              onClick={() => void start()}
              disabled={starting || !token.trim() || (!(docsPreview && importDocs) && !(conversationsPreview && importConversations))}
            >
              {starting ? <Loading01Icon className="mr-2 h-4 w-4 animate-spin" /> : null}
              Start import
            </Button>
          </div>
        </div>
      ) : null}
    </div>
  )
}

function percent(done: number, total: number): number {
  return total > 0 ? Math.min(100, Math.round((done / total) * 100)) : 0
}

function IntercomImportProgress({ workspaceId, started }: { workspaceId: string; started: Started }) {
  const docsJob = useQuery({
    queryKey: ['intercom-import', workspaceId, 'docs', started.docsJobId],
    queryFn: async () => unwrap(await docsImportService.getStatus(workspaceId, started.docsJobId!)),
    enabled: Boolean(started.docsJobId),
    refetchInterval: (query) => (query.state.data && ['done', 'failed', 'interrupted'].includes(query.state.data.status) ? false : 3000),
  })
  const conversationJob = useQuery({
    queryKey: ['intercom-import', workspaceId, 'conversations', started.conversationJobId],
    queryFn: async () => (unwrap(await intercomImportService.listConversationImports(workspaceId)) ?? []).find((job) => job.id === started.conversationJobId) ?? null,
    enabled: Boolean(started.conversationJobId),
    refetchInterval: (query) => (query.state.data && ['done', 'failed', 'cancelled'].includes(query.state.data.status) ? false : 3000),
  })
  const cancel = async (job: SupportImportJob) => {
    const response = await intercomImportService.cancelConversationImport(workspaceId, job.id)
    if (response.error) toast.error(response.error)
    else void conversationJob.refetch()
  }
  const docs = docsJob.data
  const conversations = conversationJob.data
  return (
    <div className="space-y-4">
      <div>
        <h3 className="text-base font-semibold">Importing from Intercom</h3>
        <p className="mt-1 text-sm text-muted-foreground">You can leave this page; the import keeps running.</p>
      </div>
      {started.docsJobId ? (
        <section className="space-y-2 rounded-lg border border-border p-4" data-testid="intercom-docs-progress">
          <div className="flex items-center justify-between text-sm">
            <span className="font-medium">Help Center</span>
            <span className="text-muted-foreground">{docs ? `${docs.completed + docs.failed} of ${docs.total || '…'} articles` : 'Starting…'}</span>
          </div>
          <Progress value={docs ? percent(docs.completed + docs.failed, docs.total) : 0} />
          {docs?.status === 'done' ? <p className="text-sm text-muted-foreground">Done. {docs.completed} imported{docs.failed ? `, ${docs.failed} failed` : ''}.</p> : null}
          {docs?.status === 'failed' ? <p className="text-sm text-destructive">The Help Center import failed. See Import history for details.</p> : null}
          {docs?.status === 'interrupted' ? <p className="text-sm text-muted-foreground">The Help Center import was stopped. Articles imported so far stay.</p> : null}
          {docsJob.isError ? <p className="text-sm text-destructive">The import status could not be loaded.</p> : null}
        </section>
      ) : null}
      {started.conversationJobId ? (
        <section className="space-y-2 rounded-lg border border-border p-4" data-testid="intercom-conversations-progress">
          <div className="flex items-center justify-between text-sm">
            <span className="font-medium">Conversations</span>
            <span className="text-muted-foreground">
              {conversations ? `${conversations.imported + conversations.skipped + conversations.failed} of ${conversations.total || '…'}` : 'Starting…'}
            </span>
          </div>
          <Progress value={conversations ? percent(conversations.imported + conversations.skipped + conversations.failed, conversations.total) : 0} />
          {conversations && (conversations.status === 'pending' || conversations.status === 'running') ? (
            <div className="flex items-center justify-between gap-3">
              <p className="text-xs text-muted-foreground">{conversations.error || 'Importing…'}</p>
              <Button variant="ghost" size="sm" onClick={() => void cancel(conversations)}>Stop import</Button>
            </div>
          ) : null}
          {conversations?.status === 'done' ? (
            <p className="text-sm text-muted-foreground">
              Done. {conversations.imported} imported{conversations.skipped ? `, ${conversations.skipped} already imported` : ''}{conversations.failed ? `, ${conversations.failed} failed` : ''}.
            </p>
          ) : null}
          {conversations?.status === 'failed' ? <p className="text-sm text-destructive">{conversations.error || 'The conversation import failed.'}</p> : null}
          {conversations?.status === 'cancelled' ? <p className="text-sm text-muted-foreground">Stopped. Conversations imported so far stay.</p> : null}
          {conversationJob.isError ? <p className="text-sm text-destructive">The import status could not be loaded.</p> : null}
          {conversations && conversations.failures.length > 0 ? (
            <details className="text-xs text-muted-foreground">
              <summary className="cursor-pointer">Show failures</summary>
              <ul className="mt-1 space-y-0.5">
                {conversations.failures.slice(0, 20).map((failure) => <li key={failure.external_id}>#{failure.external_id}: {failure.error}</li>)}
              </ul>
            </details>
          ) : null}
        </section>
      ) : null}
    </div>
  )
}
