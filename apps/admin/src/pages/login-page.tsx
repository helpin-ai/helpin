import { useEffect, useState, type FormEvent } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { KeyRound, ShieldCheck } from 'lucide-react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { passkeyService } from '@/lib/services/passkeyService'
import { useAuthStore } from '@/stores/authStore'

export function LoginPage() {
  const navigate = useNavigate()
  const signInWithPasskey = useAuthStore((state) => state.signInWithPasskey)
  const signInWithPassword = useAuthStore((state) => state.signInWithPassword)
  const verify2FASignIn = useAuthStore((state) => state.verify2FASignIn)
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [totpCode, setTotpCode] = useState('')
  const [useRecoveryCode, setUseRecoveryCode] = useState(false)
  const [twoFaToken, setTwoFaToken] = useState('')
  const [rememberMe, setRememberMe] = useState(true)
  const [submitting, setSubmitting] = useState(false)
  const passkeySupported = passkeyService.isSupported()

  useEffect(() => {
    if (!passkeySupported) {
      return
    }

    let cancelled = false

    const beginPasskeyAutofill = async () => {
      const autofillSupported = await passkeyService.isAutofillSupported()
      if (cancelled || !autofillSupported) {
        return
      }

      const result = await signInWithPasskey(undefined, rememberMe, { useAutofill: true })
      if (cancelled || result.cancelled) {
        return
      }
      if (result.error) {
        toast.error('Passkey sign in failed', { description: result.error })
        return
      }

      setSubmitting(true)
      try {
        await navigate({ to: '/chat-playground' })
      } finally {
        setSubmitting(false)
      }
    }

    void beginPasskeyAutofill()

    return () => {
      cancelled = true
      passkeyService.cancelPendingAuthentication()
    }
  }, [navigate, passkeySupported, rememberMe, signInWithPasskey])

  const handlePasskeySignIn = async () => {
    setSubmitting(true)
    try {
      const { error } = await signInWithPasskey(email, rememberMe)
      if (error) {
        toast.error('Passkey sign in failed', { description: error })
        return
      }

      await navigate({ to: '/chat-playground' })
    } finally {
      setSubmitting(false)
    }
  }

  const handlePasswordSignIn = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    setSubmitting(true)
    try {
      if (twoFaToken) {
        const { error } = await verify2FASignIn(twoFaToken, totpCode, useRecoveryCode, rememberMe)
        if (error) {
          toast.error('Verification failed', { description: error })
          return
        }
        await navigate({ to: '/chat-playground' })
        return
      }

      const result = await signInWithPassword(email, password, rememberMe)
      if (result.error) {
        toast.error('Sign in failed', { description: result.error })
        return
      }
      if (result.requires2FA && result.twoFAToken) {
        setTwoFaToken(result.twoFAToken)
        setTotpCode('')
        return
      }
      await navigate({ to: '/chat-playground' })
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="flex min-h-screen items-center justify-center px-6 py-12">
      <Card className="w-full max-w-md border-border bg-background">
        <CardHeader>
          <div className="flex items-center gap-2 text-sm font-medium text-muted-foreground">
            <ShieldCheck className="h-4 w-4" />
            Helpin Admin
          </div>
          <CardTitle className="text-2xl">Sign in</CardTitle>
          <CardDescription>
            Use a platform admin account. Two-factor verification is required for admin access.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form className="space-y-4" onSubmit={handlePasswordSignIn}>
            <div className="space-y-2">
              <Label htmlFor="email">Email</Label>
              <Input
                id="email"
                type="email"
                name="email"
                autoComplete="username"
                value={email}
                onChange={(event) => setEmail(event.target.value)}
                placeholder="name@company.com"
                disabled={Boolean(twoFaToken)}
                required
              />
            </div>

            {!twoFaToken ? (
              <div className="space-y-2">
                <Label htmlFor="password">Password</Label>
                <Input
                  id="password"
                  type="password"
                  name="password"
                  autoComplete="current-password"
                  value={password}
                  onChange={(event) => setPassword(event.target.value)}
                  required
                />
              </div>
            ) : (
              <div className="space-y-2">
                <Label htmlFor="totp-code">
                  {useRecoveryCode ? 'Recovery code' : 'Authentication code'}
                </Label>
                <Input
                  id="totp-code"
                  type="text"
                  inputMode={useRecoveryCode ? 'text' : 'numeric'}
                  autoComplete="one-time-code"
                  value={totpCode}
                  onChange={(event) => setTotpCode(event.target.value)}
                  placeholder={useRecoveryCode ? 'XXXX-XXXX' : '123456'}
                  required
                />
                <button
                  type="button"
                  className="text-xs text-muted-foreground underline-offset-4 hover:text-foreground hover:underline"
                  onClick={() => {
                    setUseRecoveryCode((value) => !value)
                    setTotpCode('')
                  }}
                >
                  {useRecoveryCode ? 'Use authenticator code' : 'Use recovery code'}
                </button>
              </div>
            )}

            <label className="flex items-center gap-2 text-sm text-muted-foreground">
              <input
                type="checkbox"
                className="h-4 w-4 rounded border-input"
                checked={rememberMe}
                onChange={(event) => setRememberMe(event.target.checked)}
              />
              Keep me signed in
            </label>

            <Button type="submit" className="w-full" disabled={submitting}>
              {submitting ? 'Signing in...' : twoFaToken ? 'Verify and continue' : 'Sign in'}
            </Button>

            {twoFaToken && (
              <Button
                type="button"
                variant="ghost"
                className="w-full"
                disabled={submitting}
                onClick={() => {
                  setTwoFaToken('')
                  setTotpCode('')
                  setUseRecoveryCode(false)
                }}
              >
                Back to password
              </Button>
            )}
          </form>

          <div className="mt-5 border-t pt-5">
            <Button
              type="button"
              variant="outline"
              className="w-full"
              disabled={submitting || !passkeySupported}
              onClick={() => void handlePasskeySignIn()}
            >
              <KeyRound className="h-4 w-4" />
              Sign in with passkey
            </Button>
            {!passkeySupported && (
              <p className="mt-2 text-center text-xs text-muted-foreground">
                This browser does not support passkeys.
              </p>
            )}
          </div>
        </CardContent>
      </Card>
    </div>
  )
}
