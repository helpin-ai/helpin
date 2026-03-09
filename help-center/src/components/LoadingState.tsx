export function LoadingState({ message = 'Loading...' }: { message?: string }) {
  return (
    <div className="flex min-h-screen items-center justify-center">
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
