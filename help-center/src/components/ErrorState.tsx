import { cn } from '@/lib/utils'

interface ErrorStateProps {
  title?: string
  message?: string
  statusCode?: number
  fullScreen?: boolean
}

export function ErrorState({
  title = 'Something went wrong',
  message = "We couldn't load this page. Please try again later.",
  statusCode,
  fullScreen,
}: ErrorStateProps) {
  return (
    <div
      className={cn(
        'flex items-center justify-center px-4',
        fullScreen ? 'min-h-screen' : 'py-20',
      )}
    >
      <div className="text-center max-w-md">
        {statusCode && (
          <p
            className="text-6xl font-bold mb-2"
            style={{ color: 'var(--hc-text-muted)' }}
          >
            {statusCode}
          </p>
        )}
        <h1 className="text-xl font-semibold mb-2">{title}</h1>
        <p className="text-sm" style={{ color: 'var(--hc-text-secondary)' }}>
          {message}
        </p>
      </div>
    </div>
  )
}
