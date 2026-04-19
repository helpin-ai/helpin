import type { AnchorHTMLAttributes } from 'react'
import { Link } from '@tanstack/react-router'
import { useDocsContext } from '@/contexts/DocsContext'
import { prefixBasepath } from '@/lib/pathUtils'

type DocsLinkProps = Omit<AnchorHTMLAttributes<HTMLAnchorElement>, 'href'> & {
  to: string
}

function isExternalUrl(path: string): boolean {
  return /^[a-z]+:/i.test(path) || path.startsWith('//')
}

export function DocsLink({ to, target, ...props }: DocsLinkProps) {
  const { basepath } = useDocsContext()

  if (isExternalUrl(to) || target) {
    return <a {...props} href={prefixBasepath(basepath, to)} target={target} />
  }

  return <Link {...(props as Record<string, unknown>)} to={to as never} />
}
