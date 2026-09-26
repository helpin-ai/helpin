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
import { ArrowRight, FileText, LifeBuoy, LogOut, Plus, Send } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Skeleton } from '@/components/ui/skeleton'
import { Textarea } from '@/components/ui/textarea'
import {
  CustomerPortalApiError,
  customerPortalService,
  type CustomerPortalConfiguration,
  type CustomerPortalRequest,
  type CustomerPortalSession,
} from '@/lib/services/customerPortalService'

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

function PortalFrame({ children }: { children: ReactNode }) {
  return (
    <div className="min-h-svh bg-[#e9e8e5] px-4 py-6 text-[#1c1a17] sm:px-8 sm:py-10">
      <div className="mx-auto min-h-[calc(100svh-3rem)] max-w-5xl bg-background sm:min-h-[calc(100svh-5rem)]">
        {children}
      </div>
    </div>
  )
}

function PortalLoading() {
  return (
    <PortalFrame>
      <div className="border-b border-border px-6 py-5 sm:px-10">
        <Skeleton className="h-5 w-32 rounded-sm" />
      </div>
      <main className="mx-auto max-w-2xl px-6 py-20 sm:px-10">
        <Skeleton className="h-8 w-56 rounded-sm" />
        <Skeleton className="mt-4 h-4 w-full max-w-md rounded-sm" />
      </main>
    </PortalFrame>
  )
}

export function PortalUnavailable() {
  return (
    <PortalFrame>
      <main className="mx-auto flex min-h-[70svh] max-w-lg flex-col justify-center px-6 py-20 sm:px-10">
        <LifeBuoy className="mb-6 size-6 text-muted-foreground" aria-hidden="true" />
        <p className="text-xs font-medium uppercase tracking-[0.14em] text-muted-foreground">
          Customer support
        </p>
        <h1 className="mt-3 text-3xl font-semibold tracking-tight">This portal is unavailable</h1>
        <p className="mt-4 max-w-md leading-7 text-muted-foreground">
          The customer portal is not available right now. Please use your usual support channel
          to get in touch.
        </p>
      </main>
    </PortalFrame>
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
    <PortalFrame>
      <header className="flex items-center gap-3 border-b border-border px-6 py-5 sm:px-10">
        {configuration?.branding.logo_url ? (
          <img
            src={configuration.branding.logo_url}
            alt=""
            className="size-7 object-contain"
          />
        ) : (
          <LifeBuoy className="size-5 text-muted-foreground" aria-hidden="true" />
        )}
        <span className="font-medium">{configuration?.branding.name || 'Customer support'}</span>
      </header>
      <main className="mx-auto max-w-md px-6 py-16 sm:px-10 sm:py-24">
        <p className="text-xs font-medium uppercase tracking-[0.14em] text-muted-foreground">
          Customer portal
        </p>
        <h1 className="mt-3 text-3xl font-semibold tracking-tight">Sign in to your requests</h1>
        <p className="mt-4 leading-7 text-muted-foreground">
          We’ll email you a secure link. No password needed.
        </p>

        {submitted ? (
          <div className="mt-10 border-t border-border pt-8" role="status">
            <Send className="size-5 text-muted-foreground" aria-hidden="true" />
            <h2 className="mt-4 text-lg font-semibold">Check your email</h2>
            <p className="mt-2 leading-6 text-muted-foreground">
              If an account exists for <strong className="font-medium text-foreground">{email}</strong>,
              a sign-in link is on its way.
            </p>
            <button
              type="button"
              className="mt-6 text-sm font-medium underline underline-offset-4"
              onClick={() => setSubmitted(false)}
            >
              Use a different email
            </button>
          </div>
        ) : (
          <form className="mt-10" onSubmit={submit}>
            <Label htmlFor="portal-email">Email address</Label>
            <Input
              id="portal-email"
              type="email"
              autoComplete="email"
              required
              value={email}
              onChange={(event) => setEmail(event.target.value)}
              className="mt-2"
              placeholder="you@company.com"
            />
            {error ? <p className="mt-3 text-sm text-destructive" role="alert">{error}</p> : null}
            <Button className="mt-5 w-full" type="submit" disabled={submitting}>
              {submitting ? 'Sending link…' : 'Email me a sign-in link'}
              {!submitting ? <ArrowRight className="size-4" aria-hidden="true" /> : null}
            </Button>
          </form>
        )}
      </main>
    </PortalFrame>
  )
}

export function CustomerPortalCallback({ token }: { token?: string }) {
  const { authenticate, slug } = usePortal()
  const navigate = useNavigate()
  const [error, setError] = useState(!token)

  useEffect(() => {
    let active = true
    if (!token) return

    customerPortalService
      .exchangeMagicLink(slug, token)
      .then((nextSession) => {
        if (!active) return
        authenticate(nextSession)
        void navigate({ to: '/portal/$slug', params: { slug }, replace: true })
      })
      .catch(() => {
        if (active) setError(true)
      })

    return () => {
      active = false
    }
  }, [authenticate, navigate, slug, token])

  if (error) {
    return (
      <PortalFrame>
        <main className="mx-auto max-w-md px-6 py-24 sm:px-10">
          <p className="text-xs font-medium uppercase tracking-[0.14em] text-muted-foreground">
            Sign-in link
          </p>
          <h1 className="mt-3 text-3xl font-semibold tracking-tight">This link is no longer valid</h1>
          <p className="mt-4 leading-7 text-muted-foreground">
            Sign-in links expire and can only be used once. Request a new link to continue.
          </p>
          <Button asChild className="mt-8">
            <Link to="/portal/$slug/sign-in" params={{ slug }}>Request a new link</Link>
          </Button>
        </main>
      </PortalFrame>
    )
  }

  return (
    <PortalFrame>
      <main className="mx-auto max-w-md px-6 py-24 sm:px-10" role="status">
        <Skeleton className="h-8 w-64 rounded-sm" />
        <p className="mt-4 text-muted-foreground">Signing you in securely…</p>
      </main>
    </PortalFrame>
  )
}

function RequestList({ requests }: { requests: CustomerPortalRequest[] }) {
  if (requests.length === 0) {
    return (
      <div className="border-t border-border py-12 text-center">
        <FileText className="mx-auto size-6 text-muted-foreground" aria-hidden="true" />
        <h2 className="mt-4 font-semibold">No requests yet</h2>
        <p className="mt-2 text-sm text-muted-foreground">New support requests will appear here.</p>
      </div>
    )
  }

  return (
    <ul className="divide-y divide-border border-y border-border">
      {requests.map((request) => (
        <li key={request.id} className="flex items-center justify-between gap-6 py-5">
          <div className="min-w-0">
            <p className="truncate font-medium">{request.subject}</p>
            <p className="mt-1 text-sm text-muted-foreground">
              Updated {new Date(request.updated_at).toLocaleDateString()}
            </p>
          </div>
          <span className="shrink-0 text-sm capitalize text-muted-foreground">{request.status}</span>
        </li>
      ))}
    </ul>
  )
}

export function CustomerPortalHome() {
  const { configuration, handleSessionError, session, signOut, slug, status } = usePortal()
  const [requests, setRequests] = useState<CustomerPortalRequest[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [showRequestForm, setShowRequestForm] = useState(false)
  const [subject, setSubject] = useState('')
  const [message, setMessage] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [submissionError, setSubmissionError] = useState<string | null>(null)

  useEffect(() => {
    if (status !== 'authenticated') return
    let active = true
    customerPortalService
      .requests(slug)
      .then((nextRequests) => {
        if (active) setRequests(nextRequests)
      })
      .catch((requestError) => {
        if (!active || handleSessionError(requestError)) return
        setError('Your requests could not be loaded. Please try again.')
      })
      .finally(() => {
        if (active) setLoading(false)
      })
    return () => {
      active = false
    }
  }, [handleSessionError, slug, status])

  async function createRequest(event: FormEvent) {
    event.preventDefault()
    setSubmitting(true)
    setSubmissionError(null)
    try {
      const request = await customerPortalService.createRequest(slug, {
        subject: subject.trim(),
        message: message.trim(),
      })
      setRequests((current) => [request, ...current])
      setSubject('')
      setMessage('')
      setShowRequestForm(false)
    } catch (requestError) {
      if (handleSessionError(requestError)) return
      setSubmissionError('Your request could not be sent. Please try again.')
    } finally {
      setSubmitting(false)
    }
  }

  if (status !== 'authenticated') return null

  return (
    <PortalFrame>
      <header className="flex items-center justify-between gap-4 border-b border-border px-6 py-5 sm:px-10">
        <div className="flex min-w-0 items-center gap-3">
          {configuration?.branding.logo_url ? (
            <img src={configuration.branding.logo_url} alt="" className="size-7 object-contain" />
          ) : (
            <LifeBuoy className="size-5 shrink-0 text-muted-foreground" aria-hidden="true" />
          )}
          <span className="truncate font-medium">{configuration?.branding.name || 'Customer support'}</span>
        </div>
        <Button variant="ghost" size="sm" onClick={() => void signOut()}>
          <LogOut className="size-4" aria-hidden="true" />
          Sign out
        </Button>
      </header>
      <main className="mx-auto max-w-3xl px-6 py-12 sm:px-10 sm:py-16">
        <div className="flex flex-col justify-between gap-6 sm:flex-row sm:items-end">
          <div>
            <p className="text-xs font-medium uppercase tracking-[0.14em] text-muted-foreground">
              {configuration?.requests_only ? 'Support requests' : 'Customer portal'}
            </p>
            <h1 className="mt-3 text-3xl font-semibold tracking-tight">Your requests</h1>
            <p className="mt-3 text-muted-foreground">
              Signed in as {session?.customer.email}
            </p>
          </div>
          {configuration?.intake_enabled ? (
            <Button onClick={() => setShowRequestForm((current) => !current)}>
              <Plus className="size-4" aria-hidden="true" />
              {showRequestForm ? 'Cancel' : 'New request'}
            </Button>
          ) : null}
        </div>

        {showRequestForm ? (
          <form className="mt-10 border-t border-border pt-8" onSubmit={createRequest}>
            <p className="text-xs font-medium uppercase tracking-[0.14em] text-muted-foreground">
              New support request
            </p>
            <div className="mt-6">
              <Label htmlFor="request-subject">Subject</Label>
              <Input
                id="request-subject"
                value={subject}
                onChange={(event) => setSubject(event.target.value)}
                className="mt-2"
                required
                maxLength={200}
              />
            </div>
            <div className="mt-5">
              <Label htmlFor="request-message">How can we help?</Label>
              <Textarea
                id="request-message"
                value={message}
                onChange={(event) => setMessage(event.target.value)}
                className="mt-2 min-h-32"
                required
              />
            </div>
            {submissionError ? (
              <p className="mt-3 text-sm text-destructive" role="alert">{submissionError}</p>
            ) : null}
            <Button className="mt-5" type="submit" disabled={submitting}>
              {submitting ? 'Sending…' : 'Send request'}
            </Button>
          </form>
        ) : null}

        <section className="mt-12" aria-labelledby="requests-heading">
          <h2 id="requests-heading" className="mb-4 text-xs font-medium uppercase tracking-[0.14em] text-muted-foreground">
            Request history
          </h2>
          {loading ? (
            <div className="space-y-px border-y border-border py-2">
              {[0, 1, 2].map((item) => <Skeleton key={item} className="h-16 rounded-none" />)}
            </div>
          ) : error ? (
            <p className="border-t border-border py-8 text-sm text-destructive" role="alert">{error}</p>
          ) : (
            <RequestList requests={requests} />
          )}
        </section>
      </main>
    </PortalFrame>
  )
}
