import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { describe, expect, it } from 'vitest'

describe('TopBar header links', () => {
  it('gives button-style header links a visible hover state', () => {
    const topBarPath = join(process.cwd(), 'src/components/layout/TopBar.tsx')
    const source = readFileSync(topBarPath, 'utf8')

    expect(source).toContain('hover:brightness-95')
  })
})
