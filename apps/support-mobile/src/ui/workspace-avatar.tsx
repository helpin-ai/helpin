import { useEffect, useMemo, useState } from 'react'
import { getGoogleFaviconUrl } from '@/lib/favicon'
import { cn } from '@mobile/lib/cn'
import type { Workspace } from '@mobile/lib/types'
import { getInitials } from './avatar'

interface WorkspaceAvatarProps {
  workspace: Pick<Workspace, 'name' | 'logo_url' | 'website_url'>
  size?: number
  className?: string
}

/** Workspace mark with web parity: uploaded logo, website favicon, then initials. */
export function WorkspaceAvatar({ workspace, size = 40, className }: WorkspaceAvatarProps) {
  const sources = useMemo(() => {
    const candidates = [
      workspace.logo_url?.trim(),
      getGoogleFaviconUrl(workspace.website_url, Math.max(64, size * 2)),
    ].filter((value): value is string => !!value)
    return [...new Set(candidates)]
  }, [size, workspace.logo_url, workspace.website_url])
  const sourceKey = sources.join('\n')
  const [sourceIndex, setSourceIndex] = useState(0)

  useEffect(() => setSourceIndex(0), [sourceKey])

  const src = sources[sourceIndex]
  return (
    <span
      className={cn(
        'relative inline-flex shrink-0 select-none items-center justify-center overflow-hidden rounded-xl border border-border/70 bg-muted/40 font-semibold text-muted-foreground',
        className,
      )}
      style={{ width: size, height: size }}
    >
      {src ? (
        <img
          src={src}
          alt=""
          className="h-full w-full object-contain"
          loading="lazy"
          onError={() => setSourceIndex((index) => index + 1)}
        />
      ) : (
        <span className="uppercase" style={{ fontSize: Math.max(9, size * 0.32) }}>
          {getInitials(workspace.name)}
        </span>
      )}
    </span>
  )
}
