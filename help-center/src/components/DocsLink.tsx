import type { AnchorHTMLAttributes, MouseEvent } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { useDocsContext } from '@/contexts/DocsContext'
import { prefixBasepath } from '@/lib/pathUtils'

type DocsLinkProps = Omit<AnchorHTMLAttributes<HTMLAnchorElement>, 'href'> & {
  to: string
}

function shouldHandleClientNavigation(
  event: MouseEvent<HTMLAnchorElement>,
  target?: string,
) {
  return (
    event.button === 0 &&
    !event.metaKey &&
    !event.altKey &&
    !event.ctrlKey &&
    !event.shiftKey &&
    !target
  )
}

export function DocsLink({ to, onClick, target, ...props }: DocsLinkProps) {
  const navigate = useNavigate()
  const { basepath } = useDocsContext()
  const href = prefixBasepath(basepath, to)

  return (
    <a
      {...props}
      href={href}
      target={target}
      onClick={(event) => {
        onClick?.(event)
        if (event.defaultPrevented || !shouldHandleClientNavigation(event, target)) {
          return
        }
        event.preventDefault()
        void navigate({ to })
      }}
    />
  )
}
