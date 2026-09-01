import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

const sidebarSource = readFileSync(resolve(__dirname, '../../Sidebar.tsx'), 'utf8')

describe('workspace sidebar module layout', () => {
  it('keeps module utilities bottom-aligned above the shared search footer', () => {
    expect(sidebarSource).toContain('data-slot="sidebar-module-navigation"')
    expect(sidebarSource).toContain("'flex min-h-0 flex-1 flex-col'")
    expect(sidebarSource.indexOf('data-slot="sidebar-module-navigation"')).toBeLessThan(
      sidebarSource.indexOf('<SidebarSearchFooter'),
    )
  })
})
