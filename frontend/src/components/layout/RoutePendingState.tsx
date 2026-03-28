import { Loader2 } from 'lucide-react'

export function RoutePendingState() {
  return (
    <div className="flex h-full min-h-[42vh] items-center justify-center px-6">
      <div className="flex items-center gap-2 text-sm text-muted-foreground">
        <Loader2 className="h-4 w-4 animate-spin" />
        <span>Loading...</span>
      </div>
    </div>
  )
}
