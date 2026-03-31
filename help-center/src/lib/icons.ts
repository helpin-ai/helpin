import { lazy, type ComponentType } from 'react'
import type { IconProps } from '@phosphor-icons/react'

function kebabToPascal(s: string): string {
  return s
    .split('-')
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join('')
}

const iconCache = new Map<string, ComponentType<IconProps>>()

/**
 * Returns a lazy-loaded Phosphor icon component for a given kebab-case name.
 * Each icon is imported individually (~2KB) from the SSR dist instead of
 * loading the full package (~5MB barrel export).
 */
export function getIconComponent(
  name: string,
): ComponentType<IconProps> | null {
  if (!name || !/^[a-z]/.test(name)) return null

  const cached = iconCache.get(name)
  if (cached) return cached

  const pascal = kebabToPascal(name)
  const LazyIcon = lazy(() =>
    import(`@phosphor-icons/react/dist/ssr/${pascal}.es.js`)
      .then((mod) => ({
        default: (mod.default ?? mod[pascal]) as ComponentType<IconProps>,
      }))
      .catch(() => ({
        default: (() => null) as unknown as ComponentType<IconProps>,
      })),
  )

  iconCache.set(name, LazyIcon)
  return LazyIcon
}
