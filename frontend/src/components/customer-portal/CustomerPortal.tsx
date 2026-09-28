import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type FormEvent,
  type ReactNode,
} from 'react'
import { Link, Outlet, useNavigate, useRouterState } from '@tanstack/react-router'
import {
  QuietBreadcrumbs,
  QuietConversationComposer,
  QuietComposerEditorSurface,
  QuietEmptyState,
  QuietMetaLine,
  QuietPageHeader,
  QuietPrimaryAction,
  QuietTextAction,
  QuietUnderlineInput,
  QuietUnderlineTextarea,
} from '@/components/design-system/quiet'
import { Label } from '@/components/ui/label'
import { Skeleton } from '@/components/ui/skeleton'
import { cn } from '@/lib/utils'
import { PortalActivityRow, PortalMessage } from './PortalConversation'
import { PortalRequestTable, RequestStatusText } from './PortalRequestTable'
import { requestNumber } from './portalRequestFormat'
import { formatPortalRelativeTime } from './portalTime'
import { usePortalLiveUpdates } from './usePortalLiveUpdates'
import { PortalAttachmentTray } from './PortalAttachmentTray'
import { pastedFiles, usePortalFileDrop } from './portalFileDrop'
import { usePortalAttachmentUploads } from './usePortalAttachmentUploads'
import {
  CustomerPortalApiError,
  customerPortalService,
  type CustomerPortalAttachmentPolicy,
  type CustomerPortalConfiguration,
  type CustomerPortalRequest,
  type CustomerPortalRequestDetail,
  type CustomerPortalSession,
} from '@/lib/services/customerPortalService'


// ── Shell and shared pieces ─────────────────────────────────────────────

/**
 * PortalShell is the public portal's page: a hairline header with the
 * workspace name and account actions, then one centered column at the
 * Quiet prose measure. It uses no backdrop card; the portal lives outside
 * the authenticated app shell.
 */
function PortalShell({ children, account, width = 'prose' }: { children: ReactNode; account?: ReactNode; width?: 'prose' | 'form' | 'wide' }) {
  const { configuration } = usePortalOptional()
  const name = configuration?.branding.name || 'Customer support'
  // The request table uses the wide column; threads and forms keep a reading measure.
  const columnClassName = width === 'form' ? 'max-w-[480px]' : width === 'wide' ? 'max-w-[1080px]' : 'max-w-[760px]'
  return (
    <div className="min-h-svh bg-background text-quiet-text-primary">
      <header className="border-b border-quiet-divider-strong">
        <div className={cn('mx-auto flex h-14 w-full items-center justify-between gap-4 px-4 sm:px-6', columnClassName)}>
          <div className="flex min-w-0 items-center gap-2.5">
            <PortalBrandMark name={name} logoUrl={configuration?.branding.logo_url} color={configuration?.branding.brand_color} />
            <span className="truncate text-sm font-semibold tracking-[-0.008em]">{name}</span>
          </div>
          {account}
        </div>
      </header>
      <main className={cn('mx-auto w-full px-4 pb-20 pt-8 sm:px-6 sm:pt-10', columnClassName)}>
        {children}
      </main>
    </div>
  )
}

/** PortalBrandMark shows the workspace logo, or its initial in the brand colour. */
function PortalBrandMark({ name, logoUrl, color }: { name: string; logoUrl?: string | null; color?: string | null }) {
  if (logoUrl) return <img src={logoUrl} alt="" className="size-7 shrink-0 rounded-md object-contain" />
  return (
    <span
      aria-hidden="true"
      className="flex size-7 shrink-0 items-center justify-center rounded-md bg-quiet-action text-[12px] font-semibold text-quiet-action-ink"
      style={color ? { backgroundColor: color, color: '#fff' } : undefined}
    >
      {name.trim().charAt(0).toUpperCase() || 'S'}
    </span>
  )
}

function PortalAccount() {
  const { session, signOut } = usePortal()
  return (
    <div className="flex min-w-0 items-center gap-4">
      {session?.customer.email ? (
        <span className="hidden min-w-0 truncate text-[12.5px] text-quiet-text-tertiary sm:block">{session.customer.email}</span>
      ) : null}
      <QuietTextAction onClick={() => void signOut()}>Sign out</QuietTextAction>
    </div>
  )
}

/** PortalField stacks a label above an underline control, as in Quiet form dialogs. */
function PortalField({ id, label, children }: { id: string; label: string; children: ReactNode }) {
  return (
    <div className="min-w-0 space-y-1">
      <Label htmlFor={id} className="text-sm font-medium text-quiet-text-secondary">{label}</Label>
      {children}
    </div>
  )
}

function PortalError({ children, className }: { children: ReactNode; className?: string }) {
  return <p role="alert" className={cn('text-sm text-destructive', className)}>{children}</p>
}

/** PortalDropCue tells the customer a drag over the form will attach files. */
function PortalDropCue({ visible }: { visible: boolean }) {
  if (!visible) return null
  return <p aria-hidden="true" className="text-[12.5px] font-medium text-quiet-text-secondary">Drop files to attach them</p>
}

/**
 * usePortalComposerFiles wires uploads, drop and paste for one portal form.
 * Uploads are disabled when the workspace turns file uploads off.
 */
function usePortalComposerFiles(upload: Parameters<typeof usePortalAttachmentUploads>[0]['upload'], policy: CustomerPortalAttachmentPolicy | undefined, enabled: boolean) {
  const uploads = usePortalAttachmentUploads({ upload, policy })
  const { dragging, dropProps } = usePortalFileDrop(uploads.addFiles, enabled)
  const onPaste = useCallback((event: React.ClipboardEvent) => {
    if (!enabled) return
    const files = pastedFiles(event)
    if (files.length === 0) return
    event.preventDefault()
    uploads.addFiles(files)
  }, [enabled, uploads])
  return { uploads, dragging, dropProps, onPaste }
}

// ── Session context ─────────────────────────────────────────────────────

type PortalStatus = 'loading' | 'unavailable' | 'anonymous' | 'authenticated'

interface PortalContextValue {
  slug: string
  configuration: CustomerPortalConfiguration | null
  session: CustomerPortalSession | null
  status: PortalStatus
  authenticate: (session: CustomerPortalSession) => void
  signOut: () => Promise<void>
  handleSessionError: (error: unknown) => boolean
}

const PortalContext = createContext<PortalContextValue | null>(null)

function usePortal() {
  const context = useContext(PortalContext)
  if (!context) throw new Error('usePortal must be used within CustomerPortalProvider')
  return context
}

// The shell also renders before the provider has a configuration.
function usePortalOptional(): Partial<PortalContextValue> {
  return useContext(PortalContext) ?? {}
}

function PortalLoading() {
  return (
    <PortalShell>
      <div role="status" aria-label="Loading">
        <Skeleton className="h-6 w-48 rounded-sm" />
        <Skeleton className="mt-3 h-4 w-full max-w-sm rounded-sm" />
      </div>
    </PortalShell>
  )
}

export function PortalUnavailable() {
  return (
    <PortalShell>
      <QuietPageHeader
        title="This portal is unavailable"
        description="The customer portal isn’t available right now. Please contact support through your usual channel."
      />
    </PortalShell>
  )
}

export function CustomerPortalProvider({ slug }: { slug: string }) {
  const navigate = useNavigate()
  const pathname = useRouterState({ select: (state) => state.location.pathname })
  const entryPath = useRef(pathname)
  const [configuration, setConfiguration] = useState<CustomerPortalConfiguration | null>(null)
  const [session, setSession] = useState<CustomerPortalSession | null>(null)
  const [status, setStatus] = useState<PortalStatus>('loading')

  const goToSignIn = useCallback(() => {
    setSession(null)
    setStatus('anonymous')
    if (!pathname.endsWith('/sign-in')) {
      void navigate({ to: '/portal/$slug/sign-in', params: { slug }, replace: true })
    }
  }, [navigate, pathname, slug])

  const handleSessionError = useCallback(
    (error: unknown) => {
      if (error instanceof CustomerPortalApiError && error.status === 401) {
        goToSignIn()
        return true
      }
      return false
    },
    [goToSignIn],
  )

  useEffect(() => {
    let active = true

    async function loadPortal() {
      try {
        const nextConfiguration = await customerPortalService.configuration(slug)
        if (!active) return
        if (!nextConfiguration.enabled) {
          setStatus('unavailable')
          return
        }
        setConfiguration(nextConfiguration)

        // The callback exchanges its own token. A concurrent session check can
        // return 401 after the exchange succeeds and overwrite the new session.
        if (entryPath.current.endsWith('/callback')) {
          setStatus('anonymous')
          return
        }

        try {
          const nextSession = await customerPortalService.session(slug)
          if (!active) return
          setSession(nextSession)
          setStatus('authenticated')
          if (entryPath.current.endsWith('/sign-in')) {
            void navigate({ to: '/portal/$slug', params: { slug }, replace: true })
          }
        } catch (error) {
          if (!active) return
          if (error instanceof CustomerPortalApiError && error.status === 401) {
            setStatus('anonymous')
            if (!entryPath.current.endsWith('/sign-in') && !entryPath.current.endsWith('/callback')) {
              void navigate({ to: '/portal/$slug/sign-in', params: { slug }, replace: true })
            }
            return
          }
          throw error
        }
      } catch {
        if (active) setStatus('unavailable')
      }
    }

    void loadPortal()
    return () => {
      active = false
    }
  }, [navigate, slug])

  const signOut = useCallback(async () => {
    try {
      await customerPortalService.signOut(slug)
    } finally {
      goToSignIn()
    }
  }, [goToSignIn, slug])

  const authenticate = useCallback((nextSession: CustomerPortalSession) => {
    setSession(nextSession)
    setStatus('authenticated')
  }, [])

  const value = useMemo(
    () => ({ slug, configuration, session, status, authenticate, signOut, handleSessionError }),
    [authenticate, configuration, handleSessionError, session, signOut, slug, status],
  )

  if (status === 'loading') return <PortalLoading />
  if (status === 'unavailable') return <PortalUnavailable />

  return (
    <PortalContext.Provider value={value}>
      <Outlet />
    </PortalContext.Provider>
  )
}

// ── Sign in and requests without sign-in ────────────────────────────────

function AnonymousIntakeForm({ slug, fileUploadsEnabled, policy }: { slug: string; fileUploadsEnabled: boolean; policy?: CustomerPortalAttachmentPolicy }) {
  const [email, setEmail] = useState('')
  const [token, setToken] = useState('')
  const [subject, setSubject] = useState('')
  const [message, setMessage] = useState('')
  const upload = useCallback<Parameters<typeof usePortalAttachmentUploads>[0]['upload']>(
    (file, options) => customerPortalService.uploadAnonymousAttachment(slug, token, file, options),
    [slug, token],
  )
  const { uploads, dragging, dropProps, onPaste } = usePortalComposerFiles(upload, policy, fileUploadsEnabled && Boolean(token))
  const [working, setWorking] = useState(false)
  const [submitted, setSubmitted] = useState(false)
  const [error, setError] = useState<string | null>(null)

  async function start(event: FormEvent) {
    event.preventDefault()
    setWorking(true)
    setError(null)
    try {
      const session = await customerPortalService.startAnonymousIntake(slug, email.trim())
      setToken(session.intake_token)
    } catch {
      setError('Unable to start a request. Please try again.')
    } finally { setWorking(false) }
  }

  async function send(event: FormEvent) {
    event.preventDefault()
    if (uploads.blocked || !token) return
    setWorking(true)
    setError(null)
    try {
      await customerPortalService.createAnonymousRequest(slug, token, {
        subject: subject.trim(), message: message.trim(), attachment_ids: uploads.attachmentIds,
      })
      setSubmitted(true)
      setToken('')
      uploads.clear()
    } catch {
      setError('Your request could not be sent. Please try again.')
    } finally { setWorking(false) }
  }

  return (
    <section className="mt-10 border-t border-quiet-divider-strong pt-6" aria-labelledby="anonymous-intake-heading">
      <h2 id="anonymous-intake-heading" className="text-[14px] font-semibold text-quiet-text-primary">Send a request without signing in</h2>
      {submitted ? (
        <p className="mt-2 text-sm leading-[1.6] text-quiet-text-tertiary" role="status">Request received. We’ve emailed you about next steps, and our team will reply by email.</p>
      ) : !token ? (
        <form className="mt-4 space-y-5" onSubmit={start}>
          <PortalField id="intake-email" label="Your email address">
            <QuietUnderlineInput id="intake-email" type="email" autoComplete="email" required value={email} onChange={(event) => setEmail(event.target.value)} />
          </PortalField>
          {error ? <PortalError>{error}</PortalError> : null}
          <QuietTextAction type="submit" disabled={working} className="text-sm font-medium text-quiet-text-primary">
            {working ? 'Starting…' : 'Continue'}
          </QuietTextAction>
        </form>
      ) : (
        <form className={cn('-mx-3 mt-1 space-y-5 rounded-lg px-3 py-3 transition-colors', dragging && 'bg-quiet-row-hover')} onSubmit={send} {...dropProps}>
          <p className="text-[12.5px] text-quiet-text-tertiary">
            Requesting as <span className="text-quiet-text-secondary">{email}</span>
            <span aria-hidden="true"> · </span>
            <QuietTextAction type="button" onClick={() => { setToken(''); uploads.clear() }}>Change email</QuietTextAction>
          </p>
          <PortalField id="intake-subject" label="Subject">
            <QuietUnderlineInput id="intake-subject" required maxLength={200} value={subject} onChange={(event) => setSubject(event.target.value)} />
          </PortalField>
          <PortalField id="intake-message" label="How can we help?">
            <QuietUnderlineTextarea id="intake-message" required rows={4} className="max-h-none min-h-24" value={message} onPaste={onPaste} onChange={(event) => setMessage(event.target.value)} />
          </PortalField>
          <PortalDropCue visible={dragging} />
          {fileUploadsEnabled ? <PortalAttachmentTray uploads={uploads} policy={policy} /> : null}
          {error ? <PortalError>{error}</PortalError> : null}
          <QuietPrimaryAction type="submit" disabled={working || uploads.blocked}>{working ? 'Sending…' : 'Send request'}</QuietPrimaryAction>
        </form>
      )}
    </section>
  )
}

export function CustomerPortalSignIn() {
  const { configuration, slug, status } = usePortal()
  const [email, setEmail] = useState('')
  const [submitted, setSubmitted] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  async function submit(event: FormEvent) {
    event.preventDefault()
    setSubmitting(true)
    setError(null)
    try {
      await customerPortalService.requestMagicLink(slug, email.trim())
      setSubmitted(true)
    } catch (requestError) {
      setError(
        requestError instanceof CustomerPortalApiError
          ? requestError.message
          : 'We could not send the sign-in link. Please try again.',
      )
    } finally {
      setSubmitting(false)
    }
  }

  if (status === 'authenticated') return null

  return (
    <PortalShell width="form">
      {submitted ? (
        <div role="status">
          <QuietPageHeader
            title="Check your email"
            description={<>If <span className="font-medium text-quiet-text-primary">{email}</span> has access to this portal, a sign-in link is on its way. It works once and expires after 15 minutes.</>}
          />
          <QuietTextAction className="mt-5" onClick={() => setSubmitted(false)}>Use a different email</QuietTextAction>
        </div>
      ) : (
        <>
          <QuietPageHeader title="Sign in to your requests" description="We’ll email you a secure link. No password needed." />
          <form className="mt-8 space-y-6" onSubmit={submit}>
            <PortalField id="portal-email" label="Email address">
              <QuietUnderlineInput
                id="portal-email"
                type="email"
                autoComplete="email"
                required
                value={email}
                onChange={(event) => setEmail(event.target.value)}
                placeholder="you@company.com"
              />
            </PortalField>
            {error ? <PortalError>{error}</PortalError> : null}
            <QuietPrimaryAction type="submit" disabled={submitting}>
              {submitting ? 'Sending link…' : 'Email me a sign-in link'}
            </QuietPrimaryAction>
          </form>
        </>
      )}
      {configuration?.anonymous_intake_enabled ? <AnonymousIntakeForm slug={slug} fileUploadsEnabled={configuration.file_uploads_enabled} policy={configuration.attachments} /> : null}
    </PortalShell>
  )
}

export function CustomerPortalCallback({ token }: { token?: string }) {
  const { authenticate, slug } = usePortal()
  const navigate = useNavigate()
  const [error, setError] = useState(!token)
  // A temporary server failure leaves the link unused, so it can be retried.
  const [temporaryFailure, setTemporaryFailure] = useState(false)
  const [attempt, setAttempt] = useState(0)
  // Sign-in links are single-use. StrictMode and remounts re-run this effect,
  // so every run shares the first exchange instead of spending the link twice.
  const exchange = useRef<{ token: string; attempt: number; session: Promise<CustomerPortalSession> } | null>(null)

  useEffect(() => {
    let active = true
    if (!token) return

    if (exchange.current?.token !== token || exchange.current.attempt !== attempt) {
      exchange.current = { token, attempt, session: customerPortalService.exchangeMagicLink(slug, token) }
    }
    exchange.current.session
      .then((nextSession) => {
        if (!active) return
        authenticate(nextSession)
        void navigate({ to: '/portal/$slug', params: { slug }, replace: true })
      })
      .catch((reason) => {
        if (!active) return
        if (reason instanceof CustomerPortalApiError && reason.status === 401) setError(true)
        else setTemporaryFailure(true)
      })

    return () => {
      active = false
    }
  }, [attempt, authenticate, navigate, slug, token])

  if (temporaryFailure) {
    return (
      <PortalShell width="form">
        <QuietPageHeader title="We couldn’t sign you in right now" description="Something went wrong on our side. Your link still works, so try again in a moment." />
        <QuietPrimaryAction
          className="mt-6"
          onClick={() => {
            setTemporaryFailure(false)
            setAttempt((current) => current + 1)
          }}
        >
          Try again
        </QuietPrimaryAction>
      </PortalShell>
    )
  }

  if (error) {
    return (
      <PortalShell width="form">
        <QuietPageHeader title="This link is no longer valid" description="Sign-in links expire and can only be used once. Request a new link to continue." />
        <QuietPrimaryAction asChild className="mt-6">
          <Link to="/portal/$slug/sign-in" params={{ slug }}>Request a new link</Link>
        </QuietPrimaryAction>
      </PortalShell>
    )
  }

  return (
    <PortalShell width="form">
      <div role="status">
        <QuietPageHeader title="Signing you in…" />
        <Skeleton className="mt-4 h-4 w-full max-w-xs rounded-sm" />
      </div>
    </PortalShell>
  )
}

// ── Requests ────────────────────────────────────────────────────────────

function NewRequestForm({ slug, fileUploadsEnabled, policy, onCancel, onCreated, handleSessionError }: {
  slug: string
  fileUploadsEnabled: boolean
  policy?: CustomerPortalAttachmentPolicy
  onCancel: () => void
  onCreated: (reference: string) => void
  handleSessionError: (error: unknown) => boolean
}) {
  const [subject, setSubject] = useState('')
  const [message, setMessage] = useState('')
  const upload = useCallback<Parameters<typeof usePortalAttachmentUploads>[0]['upload']>(
    (file, options) => customerPortalService.uploadAttachment(slug, file, undefined, options),
    [slug],
  )
  const { uploads, dragging, dropProps, onPaste } = usePortalComposerFiles(upload, policy, fileUploadsEnabled)
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  async function createRequest(event: FormEvent) {
    event.preventDefault()
    if (uploads.blocked) return
    setSubmitting(true)
    setError(null)
    try {
      const created = await customerPortalService.createRequest(slug, {
        subject: subject.trim(),
        message: message.trim(),
        attachment_ids: uploads.attachmentIds,
      })
      uploads.clear()
      onCreated(created.reference)
    } catch (requestError) {
      if (handleSessionError(requestError)) return
      setError('Your request could not be sent. Please try again.')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <form className={cn('-mx-3 mt-8 max-w-[784px] space-y-6 rounded-lg px-3 py-3 transition-colors', dragging && 'bg-quiet-row-hover')} onSubmit={createRequest} aria-label="New request" {...dropProps}>
      <PortalField id="request-subject" label="Subject">
        <QuietUnderlineInput id="request-subject" value={subject} onChange={(event) => setSubject(event.target.value)} required maxLength={200} placeholder="A short summary, e.g. “Invoices export is missing VAT”" />
      </PortalField>
      <PortalField id="request-message" label="How can we help?">
        <QuietUnderlineTextarea id="request-message" rows={6} className="max-h-none min-h-32" value={message} onPaste={onPaste} onChange={(event) => setMessage(event.target.value)} required placeholder="What happened, what you expected, and any steps to reproduce it." />
      </PortalField>
      <PortalDropCue visible={dragging} />
      {fileUploadsEnabled ? <PortalAttachmentTray uploads={uploads} policy={policy} /> : null}
      {error ? <PortalError>{error}</PortalError> : null}
      <div className="flex items-center gap-5">
        <QuietPrimaryAction type="submit" disabled={submitting || uploads.blocked}>{submitting ? 'Sending…' : 'Send request'}</QuietPrimaryAction>
        <QuietTextAction type="button" onClick={onCancel}>Cancel</QuietTextAction>
      </div>
    </form>
  )
}

export function CustomerPortalHome() {
  const { configuration, handleSessionError, slug, status } = usePortal()
  const [reload, setReload] = useState(0)
  // The skeleton shows only on the first load, so live refreshes keep the
  // table, its filters and its open groups in place.
  const [result, setResult] = useState<{ requests: CustomerPortalRequest[]; error: string | null } | null>(null)
  const supportName = configuration?.branding.name || 'Support'
  const refresh = useCallback(() => setReload((current) => current + 1), [])

  usePortalLiveUpdates(slug, status === 'authenticated', { onChange: refresh })

  useEffect(() => {
    if (status !== 'authenticated') return
    let active = true
    // Every request is loaded once; the table filters and counts them locally.
    customerPortalService
      .requests(slug, 'all')
      .then((nextRequests) => {
        if (active) setResult({ requests: nextRequests, error: null })
      })
      .catch((requestError) => {
        if (!active || handleSessionError(requestError)) return
        setResult((previous) => previous?.requests.length ? previous : { requests: [], error: 'Your requests could not be loaded.' })
      })
    return () => {
      active = false
    }
  }, [handleSessionError, slug, status, reload])

  if (status !== 'authenticated') return null

  return (
    <PortalShell account={<PortalAccount />} width="wide">
      <QuietPageHeader
        title="Your requests"
        actions={configuration?.intake_enabled ? (
          <QuietPrimaryAction asChild>
            <Link to="/portal/$slug/new" params={{ slug }}>New request</Link>
          </QuietPrimaryAction>
        ) : null}
      />

      <section className="mt-4" aria-label="Request history">
        {!result ? (
          <div className="border-t border-quiet-divider-strong" role="status" aria-label="Loading requests">
            {[0, 1, 2].map((item) => (
              <div key={item} className="space-y-2 border-b border-quiet-divider-light px-2 py-3 motion-safe:animate-pulse">
                <div className="h-2 w-16 bg-quiet-icon-well" />
                <div className="h-3 w-3/5 bg-quiet-icon-well" />
                <div className="h-2 w-2/5 bg-quiet-icon-well" />
              </div>
            ))}
          </div>
        ) : result.error ? (
          <QuietEmptyState
            title={result.error}
            description="Check your connection and try again."
            action={<QuietTextAction onClick={refresh}>Try again</QuietTextAction>}
          />
        ) : (
          <PortalRequestTable requests={result.requests} slug={slug} supportName={supportName} onRefresh={refresh} />
        )}
      </section>
    </PortalShell>
  )
}

export function CustomerPortalNewRequestPage() {
  const { configuration, handleSessionError, slug, status } = usePortal()
  const navigate = useNavigate()
  if (status !== 'authenticated') return null
  const backToRequests = () => void navigate({ to: '/portal/$slug', params: { slug } })
  return (
    <PortalShell account={<PortalAccount />} width="wide">
      <QuietPageHeader
        navigation={<QuietBreadcrumbs onBack={backToRequests} backLabel="Back to all requests" items={[{ id: 'requests', label: 'Requests', onClick: backToRequests }]} />}
        title="New request"
        description="Tell us what you need. We’ll reply here and by email."
      />
      {configuration?.intake_enabled ? (
        <NewRequestForm
          slug={slug}
          fileUploadsEnabled={Boolean(configuration?.file_uploads_enabled)}
          policy={configuration?.attachments}
          handleSessionError={handleSessionError}
          onCancel={backToRequests}
          onCreated={(reference) => void navigate({ to: '/portal/$slug/requests/$reference', params: { slug, reference } })}
        />
      ) : (
        <QuietEmptyState className="mt-8" title="New requests are turned off" description="Contact support through your usual channel." action={<QuietTextAction onClick={backToRequests}>All requests</QuietTextAction>} />
      )}
    </PortalShell>
  )
}

// ── Request detail ──────────────────────────────────────────────────────

function ReplyComposer({ slug, reference, fileUploadsEnabled, policy, sending, onSend }: {
  slug: string
  reference: string
  fileUploadsEnabled: boolean
  policy?: CustomerPortalAttachmentPolicy
  sending: boolean
  onSend: (content: string, attachmentIds: string[]) => Promise<boolean>
}) {
  const [reply, setReply] = useState('')
  const [focused, setFocused] = useState(false)
  const upload = useCallback<Parameters<typeof usePortalAttachmentUploads>[0]['upload']>(
    (file, options) => customerPortalService.uploadAttachment(slug, file, reference, options),
    [reference, slug],
  )
  const { uploads, dragging, dropProps, onPaste } = usePortalComposerFiles(upload, policy, fileUploadsEnabled)
  // As in chat, a reply may be text, files, or both.
  const canSend = (reply.trim().length > 0 || uploads.attachmentIds.length > 0) && !uploads.blocked && !sending

  async function submit(event: FormEvent) {
    event.preventDefault()
    if (!canSend) return
    if (await onSend(reply.trim(), uploads.attachmentIds)) {
      setReply('')
      uploads.clear()
    }
  }

  // Portal messages are plain text, so the composer omits rich-text formatting.
  return (
    <form onSubmit={(event) => void submit(event)} className="mt-6">
      <Label htmlFor="portal-reply" className="sr-only">Reply</Label>
      <div {...dropProps}>
        <QuietConversationComposer focused={focused || dragging}>
          <QuietComposerEditorSurface>
            <textarea
              id="portal-reply"
              value={reply}
              onChange={(event) => setReply(event.target.value)}
              onPaste={onPaste}
              onFocus={() => setFocused(true)}
              onBlur={() => setFocused(false)}
              rows={3}
              placeholder={dragging ? 'Drop files to attach them' : fileUploadsEnabled ? 'Write a reply, or drop files here…' : 'Write a reply…'}
              className="block max-h-72 min-h-16 w-full resize-none border-0 bg-transparent p-0 text-[14px] leading-[1.7] text-quiet-text-primary shadow-none outline-none placeholder:text-quiet-muted focus-visible:ring-0"
            />
          </QuietComposerEditorSurface>
          <div className="flex items-end justify-between gap-4 border-t border-quiet-divider-light px-4 py-2.5">
            {fileUploadsEnabled ? <PortalAttachmentTray uploads={uploads} policy={policy} /> : <span />}
            <QuietPrimaryAction type="submit" disabled={!canSend} className="shrink-0">
              {sending ? 'Sending…' : 'Send reply'}
            </QuietPrimaryAction>
          </div>
        </QuietConversationComposer>
      </div>
    </form>
  )
}

export function CustomerPortalRequestPage({ reference }: { reference: string }) {
  const { slug, status, configuration, handleSessionError } = usePortal()
  const navigate = useNavigate()
  const [request, setRequest] = useState<CustomerPortalRequestDetail | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [sending, setSending] = useState(false)
  const [refreshKey, setRefreshKey] = useState(0)
  const [typing, setTyping] = useState(false)
  const typingTimer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined)

  usePortalLiveUpdates(slug, status === 'authenticated', {
    onChange: (changed) => {
      if (changed === null || changed === reference) setRefreshKey((current) => current + 1)
    },
    onTyping: (changed, isTyping) => {
      if (changed !== reference) return
      clearTimeout(typingTimer.current)
      setTyping(isTyping)
      // A lost "stopped" signal must not leave the indicator on forever.
      if (isTyping) typingTimer.current = setTimeout(() => setTyping(false), 8000)
    },
  })
  useEffect(() => () => clearTimeout(typingTimer.current), [])

  useEffect(() => {
    if (status !== 'authenticated') return
    let active = true
    customerPortalService.requestDetail(slug, reference)
      .then((detail) => {
        if (!active) return
        setRequest(detail)
        setError(null)
      })
      .catch((reason) => {
        if (!active || handleSessionError(reason)) return
        setError(reason instanceof CustomerPortalApiError && reason.status === 404 ? 'This request is unavailable.' : 'Unable to load this request. Please try again.')
      })
      .finally(() => { if (active) setLoading(false) })
    return () => { active = false }
  }, [slug, reference, status, handleSessionError, refreshKey])

  async function send(content: string, attachmentIds: string[]) {
    if (!request?.can_reply) return false
    setSending(true)
    setError(null)
    try {
      setRequest(await customerPortalService.reply(slug, reference, content, attachmentIds))
      return true
    } catch (reason) {
      if (handleSessionError(reason)) return false
      if (reason instanceof CustomerPortalApiError && (reason.status === 409 || reason.status === 404)) {
        setRequest((previous) => previous ? { ...previous, can_reply: false } : previous)
        setError('This request can no longer receive replies.')
      } else setError('Your reply could not be sent. Please try again.')
      return false
    } finally { setSending(false) }
  }

  if (status !== 'authenticated') return null

  const backToRequests = () => void navigate({ to: '/portal/$slug', params: { slug } })

  return (
    <PortalShell account={<PortalAccount />} width="wide">
      <QuietPageHeader
        navigation={<QuietBreadcrumbs onBack={backToRequests} backLabel="Back to all requests" items={[{ id: 'requests', label: 'Requests', onClick: backToRequests }]} />}
        title={request?.subject ?? 'Request'}
        description={request ? (
          <QuietMetaLine
            className="text-[12px]"
            items={[requestNumber(request), request.last_activity_at ? `Updated ${formatPortalRelativeTime(request.last_activity_at)}` : null]}
          />
        ) : null}
        actions={request ? <RequestStatusText status={request.status} /> : null}
      />

      {loading ? (
        <div className="mt-8 space-y-4" role="status" aria-label="Loading request">
          <Skeleton className="h-20 w-full rounded-xl" />
          <Skeleton className="h-20 w-5/6 rounded-xl" />
        </div>
      ) : error && !request ? (
        <QuietEmptyState className="mt-8" title={error} description="Go back to your requests to see what you can open." action={<QuietTextAction onClick={backToRequests}>All requests</QuietTextAction>} />
      ) : request ? (
        <>
          <ol className="mt-6 border-t border-quiet-divider-strong pt-3" aria-label="Messages">
            {request.messages.map((message) => <PortalMessage key={message.id} message={message} supportName={configuration?.branding.name || 'Support'} />)}
            {request.ai_processing ? (
              <PortalActivityRow label={`${configuration?.branding.assistant_name || 'The AI assistant'} is working on your request…`} />
            ) : typing ? (
              <PortalActivityRow label={`${configuration?.branding.name || 'Support'} is replying…`} />
            ) : null}
          </ol>
          {error ? <PortalError className="mt-4">{error}</PortalError> : null}
          {request.can_reply ? (
            <ReplyComposer slug={slug} reference={reference} fileUploadsEnabled={Boolean(configuration?.file_uploads_enabled)} policy={configuration?.attachments} sending={sending} onSend={send} />
          ) : (
            <p className="mt-6 border-t border-quiet-divider-strong pt-4 text-sm text-quiet-text-tertiary">This request cannot receive replies.</p>
          )}
        </>
      ) : null}
    </PortalShell>
  )
}
