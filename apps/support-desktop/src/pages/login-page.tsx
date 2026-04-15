import { useState, type FormEvent } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { toast } from 'sonner'
import { isTauriDesktop } from '@desktop/lib/desktopHost'
import { workspacesService } from '@/lib/services/workspacesService'
import { useAuthStore } from '@/stores/authStore'

export function LoginPage() {
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [rememberMe, setRememberMe] = useState(true)
  const [loading, setLoading] = useState(false)
  const navigate = useNavigate()
  const signIn = useAuthStore((state) => state.signIn)

  const handleSubmit = async (event: FormEvent) => {
    event.preventDefault()
    setLoading(true)

    const result = await signIn(email, password, rememberMe)
    if (result.error) {
      toast.error(result.error)
      setLoading(false)
      return
    }

    const { data: workspaces } = await workspacesService.list()
    setLoading(false)

    if (!workspaces || workspaces.length === 0) {
      navigate({ to: '/workspaces' })
      return
    }

    const user = useAuthStore.getState().user
    const defaultWorkspace = user?.default_workspace_id
      ? workspaces.find((workspace) => workspace.id === user.default_workspace_id) ?? null
      : null

    navigate({
      to: '/w/$slug/support',
      params: { slug: (defaultWorkspace ?? workspaces[0]).slug },
    })
  }

  return (
    <div className="flex min-h-full items-center justify-center px-6 py-10">
      <form
        className="w-full max-w-md rounded-3xl border border-border/70 bg-card/95 p-8 shadow-xl shadow-black/5 backdrop-blur"
        onSubmit={handleSubmit}
      >
        <div className="space-y-2 text-center">
          <div className="text-xs font-semibold uppercase tracking-[0.24em] text-muted-foreground">
            Helpin Support Desktop
          </div>
          <h1 className="text-3xl font-semibold tracking-tight">Sign in</h1>
          <p className="text-sm text-muted-foreground">
            Auth-first scaffold for the support desktop client.
          </p>
          <p className="text-xs text-muted-foreground">
            {isTauriDesktop() ? 'Running inside the native Tauri shell.' : 'Running in browser preview mode.'}
          </p>
        </div>

        <div className="mt-8 space-y-4">
          <label className="block space-y-2">
            <span className="text-sm font-medium">Email</span>
            <input
              className="flex h-11 w-full rounded-xl border border-input bg-background px-3 text-sm outline-none transition focus:border-primary"
              type="email"
              value={email}
              onChange={(event) => setEmail(event.target.value)}
              placeholder="you@example.com"
              required
            />
          </label>

          <label className="block space-y-2">
            <span className="text-sm font-medium">Password</span>
            <input
              className="flex h-11 w-full rounded-xl border border-input bg-background px-3 text-sm outline-none transition focus:border-primary"
              type="password"
              value={password}
              onChange={(event) => setPassword(event.target.value)}
              placeholder="••••••••"
              required
            />
          </label>

          <label className="flex items-center gap-3 rounded-xl border border-border/70 bg-background/70 px-3 py-3 text-sm">
            <input
              type="checkbox"
              checked={rememberMe}
              onChange={(event) => setRememberMe(event.target.checked)}
            />
            <span>Remember me on this device</span>
          </label>
        </div>

        <button
          className="mt-6 inline-flex h-11 w-full items-center justify-center rounded-xl bg-primary px-4 text-sm font-medium text-primary-foreground transition hover:opacity-95 disabled:cursor-not-allowed disabled:opacity-60"
          type="submit"
          disabled={loading}
        >
          {loading ? 'Signing in...' : 'Sign in'}
        </button>
      </form>
    </div>
  )
}
