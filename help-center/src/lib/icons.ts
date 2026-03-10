import * as PhosphorIcons from '@phosphor-icons/react'
import type { Icon as PhosphorIcon } from '@phosphor-icons/react'

function pascalToKebab(s: string): string {
  return s.replace(/([a-z0-9])([A-Z])/g, '$1-$2').toLowerCase()
}

const SKIP = new Set(['IconContext', 'IconBase', 'IconWeight'])

// Build ICON_MAP dynamically from all Phosphor exports so any icon
// chosen in the admin settings will render on the public help center.
export const ICON_MAP: Record<string, PhosphorIcon> = (() => {
  const map: Record<string, PhosphorIcon> = {}
  for (const [name, exported] of Object.entries(PhosphorIcons)) {
    // Skip non-component exports
    if (SKIP.has(name)) continue
    // Icon components are PascalCase starting with uppercase
    if (!/^[A-Z][a-z]/.test(name)) continue
    if (typeof exported !== 'object' && typeof exported !== 'function') continue
    if (exported === null) continue
    map[pascalToKebab(name)] = exported as unknown as PhosphorIcon
  }
  return map
})()
