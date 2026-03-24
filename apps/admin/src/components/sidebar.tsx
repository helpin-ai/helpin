import { Link, useLocation } from '@tanstack/react-router'
import { FlaskConical, LogOut, Mail, ShieldCheck } from 'lucide-react'
import { useAuthStore } from '@/stores/authStore'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

const NAV_ITEMS = [
  { to: '/chat-playground' as const, label: 'Chat Playground', icon: FlaskConical },
  { to: '/webhook-events' as const, label: 'Webhook Events', icon: Mail },
]

export function Sidebar() {
  const user = useAuthStore((s) => s.user)
  const signOut = useAuthStore((s) => s.signOut)
  const { pathname } = useLocation()

  return (
    <aside className="sticky top-0 flex h-screen w-56 shrink-0 flex-col border-r bg-background/50">
      <div className="flex items-center gap-2 border-b px-4 py-4">
        <ShieldCheck className="h-4 w-4 text-primary" />
        <span className="text-sm font-semibold tracking-tight">Helpin Admin</span>
      </div>

      <nav className="flex-1 space-y-1 px-2 py-3">
        {NAV_ITEMS.map((item) => (
          <Link
            key={item.to}
            to={item.to}
            className={cn(
              'flex items-center gap-2.5 rounded-md px-3 py-2 text-sm font-medium transition-colors',
              pathname === item.to || pathname.startsWith(item.to + '/')
                ? 'bg-primary/10 text-primary'
                : 'text-muted-foreground hover:bg-muted hover:text-foreground',
            )}
          >
            <item.icon className="h-4 w-4" />
            {item.label}
          </Link>
        ))}
      </nav>

      {user ? (
        <div className="border-t px-3 py-3">
          <div className="mb-2 px-1">
            <p className="truncate text-sm font-medium">{user.full_name}</p>
            <p className="truncate text-xs text-muted-foreground">{user.email}</p>
          </div>
          <Button
            variant="ghost"
            size="sm"
            className="w-full justify-start text-muted-foreground"
            onClick={signOut}
          >
            <LogOut className="mr-2 h-3.5 w-3.5" />
            Sign out
          </Button>
        </div>
      ) : null}
    </aside>
  )
}
