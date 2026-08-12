import { useState, type FormEvent } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { writeSession } from '@helpin-ai/support-core'
import { TextField } from '@mobile/ui/text-field'
import { Spinner } from '@mobile/ui/spinner'
import { HelpinLogo } from '@mobile/ui/helpin-logo'
import { authService } from '@mobile/lib/services/auth-service'
import { bootstrapAuth } from '@mobile/stores/auth-store'

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
  const navigate = useNavigate()

  const emailError = touched.email ? validateEmail(email) : undefined
  const passwordError = touched.password ? validatePassword(password) : undefined

  const handleSubmit = async (event: FormEvent) => {
    event.preventDefault()
    setTouched({ email: true, password: true })
    setSubmitError(null)

    if (validateEmail(email) || validatePassword(password)) return

    setPending(true)
    try {
      const { data, error } = await authService.signin(email.trim(), password, true)
      if (error || !data) {
        setSubmitError(error || 'Sign in failed')
        return
      }

      await writeSession({ accessToken: data.access_token, refreshToken: data.refresh_token, rememberMe: true })
      await bootstrapAuth()
      navigate({ to: '/workspaces' })
    } finally {
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
          <p className="mt-1 text-footnote text-muted-foreground">Support</p>
        </div>

        <form onSubmit={handleSubmit} className="w-full max-w-sm space-y-3" noValidate>
          <TextField
            label="Email"
            type="email"
            autoComplete="email"
            value={email}
            onChange={setEmail}
            onBlur={() => setTouched((current) => ({ ...current, email: true }))}
            error={emailError ?? undefined}
          />
          <TextField
            label="Password"
            type="password"
            autoComplete="current-password"
            value={password}
            onChange={setPassword}
            onBlur={() => setTouched((current) => ({ ...current, password: true }))}
            error={passwordError ?? undefined}
          />

          {submitError && <p className="px-1 text-footnote text-destructive">{submitError}</p>}

          <button
            type="submit"
            disabled={pending}
            className="flex h-[52px] w-full items-center justify-center rounded-xl bg-primary text-body font-medium text-primary-foreground transition disabled:opacity-60"
          >
            {pending ? <Spinner size={20} className="text-primary-foreground" /> : 'Sign in'}
          </button>
        </form>
      </div>
    </div>
  )
}
