import { useEffect, useState, type FormEvent } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { writeSession } from '@helpin-ai/support-core'
import { TextField } from '@mobile/ui/text-field'
import { Spinner } from '@mobile/ui/spinner'
import { HelpinLogo } from '@mobile/ui/helpin-logo'
import { authService } from '@mobile/lib/services/auth-service'
import { googleAuthErrorMessage, startGoogleAuth } from '@mobile/lib/google-auth'
import { bootstrapAuth } from '@mobile/stores/auth-store'
import type { AuthResponse, SigninResponse } from '@mobile/lib/types'

const EMAIL_RE = /^[^\s@]+@[^\s@]+\.[^\s@]+$/

export function validateEmail(email: string): string | null {
  const trimmed = email.trim()
  if (!trimmed) return 'Email is required'
  if (!EMAIL_RE.test(trimmed)) return 'Enter a valid email address'
  return null
}

export function validatePassword(password: string): string | null {
  if (!password) return 'Password is required'
  return null
}

interface Touched {
  email?: boolean
  password?: boolean
}

export function LoginScreen() {
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [touched, setTouched] = useState<Touched>({})
  const [submitError, setSubmitError] = useState<string | null>(null)
  const [pending, setPending] = useState(false)
  const [twoFaToken, setTwoFaToken] = useState<string | null>(null)
  const [twoFactorCode, setTwoFactorCode] = useState('')
  const [useRecoveryCode, setUseRecoveryCode] = useState(false)
  const navigate = useNavigate()

  const emailError = touched.email ? validateEmail(email) : undefined
  const passwordError = touched.password ? validatePassword(password) : undefined

  const finishLogin = async (data: AuthResponse) => {
    await writeSession({ accessToken: data.access_token, refreshToken: data.refresh_token, rememberMe: true })
    await bootstrapAuth()
    navigate({ to: '/workspaces' })
  }

  const acceptSigninResponse = async (data: SigninResponse): Promise<void> => {
    if (data.requires_2fa && data.two_fa_token) {
      setTwoFaToken(data.two_fa_token)
      setTwoFactorCode('')
      setUseRecoveryCode(false)
      return
    }
    if (!data.user || !data.access_token || !data.refresh_token) {
      throw new Error('Sign in failed')
    }
    await finishLogin({ user: data.user, access_token: data.access_token, refresh_token: data.refresh_token })
  }

  useEffect(() => {
    const params = new URLSearchParams(window.location.search)
    const googleResult = params.get('google')
    const googleError = params.get('google_error')
    if (!googleResult && !googleError) return

    // Remove the callback marker before starting async work. This prevents
    // React StrictMode's development remount from rotating the refresh token
    // twice against the same Google browser session.
    window.history.replaceState({}, '', window.location.pathname)
    if (googleError) {
      setSubmitError(googleAuthErrorMessage(googleError))
      return
    }

    let cancelled = false
    setPending(true)
    void authService.refreshBrowserSession().then(async ({ data, error }) => {
      if (cancelled) return
      if (error || !data) {
        setSubmitError(error || 'Google sign in could not create an app session')
        return
      }
      await finishLogin(data)
    }).catch((error: unknown) => {
      if (!cancelled) setSubmitError(error instanceof Error ? error.message : 'Google sign in failed')
    }).finally(() => {
      if (!cancelled) setPending(false)
    })

    return () => {
      cancelled = true
    }
  }, [])

  const handleSubmit = async (event: FormEvent) => {
    event.preventDefault()
    setSubmitError(null)

    if (!twoFaToken) {
      setTouched({ email: true, password: true })
      if (validateEmail(email) || validatePassword(password)) return
    } else if (!twoFactorCode.trim()) {
      setSubmitError(useRecoveryCode ? 'Recovery code is required' : 'Authenticator code is required')
      return
    }

    setPending(true)
    try {
      if (twoFaToken) {
        const { data, error } = await authService.verify2FASignin(
          twoFaToken,
          twoFactorCode.trim(),
          useRecoveryCode,
        )
        if (error || !data) {
          setSubmitError(error || 'Verification failed')
          return
        }
        await finishLogin(data)
        return
      }

      const { data, error } = await authService.signin(email.trim(), password, true)
      if (error || !data) {
        setSubmitError(error || 'Sign in failed')
        return
      }
      await acceptSigninResponse(data)
    } catch (error) {
      setSubmitError(error instanceof Error ? error.message : 'Sign in failed')
    } finally {
      setPending(false)
    }
  }

  const handleGoogle = async () => {
    setSubmitError(null)
    setPending(true)
    try {
      await startGoogleAuth()
      // Native returns through a deep link; the hosted build navigates away.
      // Re-enable the control if opening the system browser returns normally.
      setPending(false)
    } catch (error) {
      setSubmitError(error instanceof Error ? error.message : 'Could not open Google sign in')
      setPending(false)
    }
  }

  return (
    <div
      className={[
        'flex min-h-dvh flex-col px-4 pt-[var(--safe-top)] pb-[var(--keyboard-inset)]',
        'bg-[radial-gradient(circle_at_20%_20%,rgba(188,214,231,0.75),rgba(245,248,251,0.92)_45%,rgba(187,210,229,0.55)_100%)]',
        'dark:bg-[radial-gradient(circle_at_20%_20%,rgba(45,63,92,0.65),rgba(9,12,20,0.97)_45%,rgba(20,30,48,0.7)_100%)]',
      ].join(' ')}
    >
      <div className="flex flex-1 flex-col items-center justify-center gap-10">
        <div className="text-center">
          <HelpinLogo />
        </div>

        <form onSubmit={handleSubmit} className="w-full max-w-sm space-y-3" noValidate>
          {twoFaToken ? (
            <div className="space-y-3">
              <div className="rounded-xl border border-border/80 bg-background/70 px-4 py-3 text-footnote text-muted-foreground">
                {useRecoveryCode
                  ? 'Enter one of your saved recovery codes.'
                  : 'Enter the 6-digit code from your authenticator app.'}
              </div>
              <TextField
                label={useRecoveryCode ? 'Recovery code' : 'Authenticator code'}
                autoComplete="one-time-code"
                value={twoFactorCode}
                onChange={setTwoFactorCode}
                disabled={pending}
              />
              <div className="flex items-center justify-between gap-3 px-1 text-footnote">
                <button
                  type="button"
                  className="min-h-11 text-primary"
                  onClick={() => {
                    setUseRecoveryCode((current) => !current)
                    setTwoFactorCode('')
                    setSubmitError(null)
                  }}
                >
                  {useRecoveryCode ? 'Use authenticator code' : 'Use recovery code'}
                </button>
                <button
                  type="button"
                  className="min-h-11 text-muted-foreground"
                  onClick={() => {
                    setTwoFaToken(null)
                    setTwoFactorCode('')
                    setSubmitError(null)
                  }}
                >
                  Back
                </button>
              </div>
            </div>
          ) : (
            <>
              <button
                type="button"
                disabled={pending}
                onClick={() => void handleGoogle()}
                className="flex h-[52px] w-full items-center justify-center gap-3 rounded-xl border border-input bg-background text-body font-medium text-foreground transition active:scale-[0.98] disabled:opacity-60"
              >
                <GoogleMark />
                Continue with Google
              </button>

              <div className="flex items-center gap-3 py-1" aria-hidden="true">
                <span className="h-px flex-1 bg-border" />
                <span className="text-caption uppercase tracking-[0.12em] text-muted-foreground">or</span>
                <span className="h-px flex-1 bg-border" />
              </div>

              <TextField
                label="Email"
                type="email"
                autoComplete="username"
                value={email}
                onChange={setEmail}
                onBlur={() => setTouched((current) => ({ ...current, email: true }))}
                error={emailError ?? undefined}
                disabled={pending}
              />
              <TextField
                label="Password"
                type="password"
                autoComplete="current-password"
                value={password}
                onChange={setPassword}
                onBlur={() => setTouched((current) => ({ ...current, password: true }))}
                error={passwordError ?? undefined}
                disabled={pending}
              />
            </>
          )}

          {submitError && <p className="px-1 text-footnote text-destructive">{submitError}</p>}

          <button
            type="submit"
            disabled={pending}
            className="flex h-[52px] w-full items-center justify-center rounded-xl bg-primary text-body font-medium text-primary-foreground transition disabled:opacity-60"
          >
            {pending ? <Spinner size={20} className="text-primary-foreground" /> : twoFaToken ? 'Verify and continue' : 'Sign in'}
          </button>
        </form>
      </div>
    </div>
  )
}

function GoogleMark() {
  return (
    <svg aria-hidden="true" viewBox="0 0 24 24" className="h-5 w-5">
      <path fill="#4285F4" d="M21.6 12.23c0-.71-.06-1.4-.18-2.07H12v3.91h5.38a4.6 4.6 0 0 1-2 3.02v2.54h3.24c1.9-1.75 2.98-4.33 2.98-7.4Z" />
      <path fill="#34A853" d="M12 22c2.7 0 4.98-.9 6.63-2.37l-3.24-2.54c-.9.6-2.05.96-3.39.96-2.61 0-4.82-1.76-5.61-4.13H3.04v2.62A10 10 0 0 0 12 22Z" />
      <path fill="#FBBC05" d="M6.39 13.92A6 6 0 0 1 6.08 12c0-.67.11-1.32.31-1.92V7.46H3.04A10 10 0 0 0 2 12c0 1.61.39 3.14 1.04 4.54l3.35-2.62Z" />
      <path fill="#EA4335" d="M12 5.95c1.47 0 2.79.51 3.83 1.5l2.87-2.88A9.63 9.63 0 0 0 12 2a10 10 0 0 0-8.96 5.46l3.35 2.62C7.18 7.71 9.39 5.95 12 5.95Z" />
    </svg>
  )
}
