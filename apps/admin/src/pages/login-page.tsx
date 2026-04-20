import { useState } from 'react'
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
  const signIn = useAuthStore((state) => state.signIn)
  const signInWithPasskey = useAuthStore((state) => state.signInWithPasskey)
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [rememberMe, setRememberMe] = useState(true)
  const [submitting, setSubmitting] = useState(false)
  const passkeySupported = passkeyService.isSupported()

  const handleSubmit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    setSubmitting(true)

    try {
      const { error } = await signIn(email, password, rememberMe)
      if (error) {
        toast.error('Sign in failed', { description: error })
        return
      }

      await navigate({ to: '/chat-playground' })
    } finally {
      setSubmitting(false)
    }
  }

  const handlePasskeySignIn = async () => {
    setSubmitting(true)
    try {
      const { error } = await signInWithPasskey(undefined, rememberMe)
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
            Use the same account you use in the main app. Admin tools stay workspace-scoped.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form className="space-y-5" onSubmit={handleSubmit}>
            <div className="space-y-2">
              <Label htmlFor="email">Email</Label>
              <Input
                id="email"
                type="email"
                autoComplete="email"
                value={email}
                onChange={(event) => setEmail(event.target.value)}
                placeholder="name@company.com"
                required
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="password">Password</Label>
              <Input
                id="password"
                type="password"
                autoComplete="current-password"
                value={password}
                onChange={(event) => setPassword(event.target.value)}
                required
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

            <Button type="submit" className="w-full" disabled={submitting}>
              {submitting ? 'Signing in...' : 'Sign in'}
            </Button>

            <Button type="button" variant="outline" className="w-full" disabled={submitting || !passkeySupported} onClick={() => void handlePasskeySignIn()}>
              Sign in with passkey
            </Button>

            {!passkeySupported && (
              <p className="text-center text-xs text-muted-foreground">
                This browser does not support passkeys.
              </p>
            )}
          </form>
        </CardContent>
      </Card>
    </div>
  )
}
