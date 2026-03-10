import { cn } from '@/lib/utils'

interface LoadingStateProps {
  message?: string
  fullScreen?: boolean
}

export function LoadingState({
  message = 'Loading...',
  fullScreen,
}: LoadingStateProps) {
  return (
    <div
      className={cn(
        'flex items-center justify-center',
        fullScreen ? 'min-h-screen' : 'py-20',
      )}
    >
      <div className="flex flex-col items-center gap-3">
        <div
          className="h-8 w-8 animate-spin rounded-full border-2 border-current border-t-transparent"
          style={{ color: 'var(--hc-accent)' }}
        />
        <p style={{ color: 'var(--hc-text-secondary)' }} className="text-sm">
          {message}
        </p>
      </div>
    </div>
  )
}
