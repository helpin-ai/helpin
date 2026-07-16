import type { CSSProperties } from 'react'

import { useDocsContext } from '@/contexts/DocsContext'
import { prefixBasepath } from '@/lib/pathUtils'

const ICON_CATALOG_VERSION = '4.1.1'
const ICON_ID_PATTERN = /^[a-z0-9-]+$/
const RAW_ICON_NAME_PATTERN = /Icon$|[a-z0-9][A-Z]/

interface PublicIconProps {
  name: string
  size?: number
  className?: string
}

export function PublicIcon({
  name,
  size = 20,
  className,
}: PublicIconProps) {
  const { basepath } = useDocsContext()

  if (!name) return null

  if (!ICON_ID_PATTERN.test(name)) {
    if (RAW_ICON_NAME_PATTERN.test(name)) {
      return (
        <span
          aria-hidden="true"
          className={className}
          style={{ display: 'inline-block', flexShrink: 0, height: size, width: size }}
        />
      )
    }
    return (
      <span
        aria-hidden="true"
        className={className}
        style={{ fontSize: size * 0.85, lineHeight: 1 }}
      >
        {name}
      </span>
    )
  }

  const assetURL = prefixBasepath(
    basepath,
    `/assets/helpin-icons/hugeicons/${ICON_CATALOG_VERSION}/${encodeURIComponent(name)}.svg`,
  )
  const style: CSSProperties = {
    width: size,
    height: size,
    backgroundColor: 'currentColor',
    display: 'inline-block',
    flexShrink: 0,
    maskImage: `url("${assetURL}")`,
    maskPosition: 'center',
    maskRepeat: 'no-repeat',
    maskSize: 'contain',
    WebkitMaskImage: `url("${assetURL}")`,
    WebkitMaskPosition: 'center',
    WebkitMaskRepeat: 'no-repeat',
    WebkitMaskSize: 'contain',
  }

  return <span aria-hidden="true" className={className} style={style} />
}
