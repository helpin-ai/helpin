import { useEffect, useState } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { ShieldCheck } from 'lucide-react'
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
  const [email, setEmail] = useState('')
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

  return (
    <div className="flex min-h-screen items-center justify-center px-6 py-12">
      <Card className="w-full max-w-md border-border/80 bg-background/90 backdrop-blur">
        <CardHeader>
          <div className="flex items-center gap-2 text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground">
            <ShieldCheck className="h-3.5 w-3.5" />
            Helpin Admin
          </div>
          <CardTitle className="text-2xl">Sign in</CardTitle>
          <CardDescription>
            Use a registered platform admin passkey.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="space-y-5">
            <div className="space-y-2">
              <Label htmlFor="email">Email optional</Label>
              <Input
                id="email"
                type="email"
                name="email"
                autoComplete="username webauthn"
                value={email}
                onChange={(event) => setEmail(event.target.value)}
                placeholder="name@company.com"
              />
            </div>

            <label className="flex items-center gap-2 text-sm text-muted-foreground">
              <input
                type="checkbox"
                className="h-4 w-4 rounded border-input"
                checked={rememberMe}
                onChange={(event) => setRememberMe(event.target.checked)}
              />
              Keep me signed in
            </label>

            <Button type="button" variant="outline" className="w-full" disabled={submitting || !passkeySupported} onClick={() => void handlePasskeySignIn()}>
              {submitting ? 'Signing in...' : 'Sign in with passkey'}
            </Button>

            {!passkeySupported && (
              <p className="text-center text-xs text-muted-foreground">
                This browser does not support passkeys.
              </p>
            )}
          </div>
        </CardContent>
      </Card>
    </div>
  )
}
