import { useEffect } from 'react'
import { useParams } from '@tanstack/react-router'
import { buildWorkspaceWebUrl, openWorkspaceWebUrl } from '@desktop/lib/webApp'

interface OpenInWebPageProps {
  pathBuilder: (params: Record<string, string>) => string
  title: string
}

export function OpenInWebPage({ pathBuilder, title }: OpenInWebPageProps) {
  const params = useParams({ strict: false }) as Record<string, string>
  const pathname = pathBuilder(params)
  const href = buildWorkspaceWebUrl(pathname)

  useEffect(() => {
    openWorkspaceWebUrl(pathname)
  }, [pathname])

  return (
    <div className="flex min-h-full items-center justify-center px-6">
      <div className="w-full max-w-md rounded-3xl border border-border/70 bg-card/95 p-8 text-center shadow-lg shadow-black/5">
        <h1 className="text-xl font-semibold">{title}</h1>
        <p className="mt-2 text-sm text-muted-foreground">
          This workflow opens in the Helpin web app.
        </p>
        <a
          href={href}
          target="_blank"
          rel="noreferrer"
          className="mt-6 inline-flex h-10 items-center justify-center rounded-xl bg-primary px-4 text-sm font-medium text-primary-foreground transition hover:opacity-95"
        >
          Open In Browser
        </a>
      </div>
    </div>
  )
}
