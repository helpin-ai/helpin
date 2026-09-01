import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

const editorSource = readFileSync(resolve(process.cwd(), 'src/components/docs/DocsEditor.tsx'), 'utf8')
const globalStyles = readFileSync(resolve(process.cwd(), 'src/index.css'), 'utf8')

describe('DocsEditor layout', () => {
  it('owns the responsive document width and gutters in one CSS layout contract', () => {
    expect(editorSource).toContain('className="docs-editor-content-frame"')
    expect(editorSource).not.toContain('docs-editor-content-frame mx-auto max-w-4xl')
    expect(editorSource).toContain('min-h-[400px] px-4 pt-3 pb-8 md:px-6')
    expect(editorSource).toContain('group/title px-4 pt-10 pb-2 md:px-6')

    expect(globalStyles).toContain('.docs-editor-wrapper,\n.docs-comment-side-gutter {')
    expect(globalStyles).toContain('--docs-editor-content-max-width: 52rem;')
    expect(globalStyles).toContain('--docs-editor-page-gutter: 1rem;')
    expect(globalStyles).toContain('--docs-editor-page-gutter: 1.5rem;')
    expect(globalStyles).toContain('max-width: var(--docs-editor-content-max-width);')
    expect(globalStyles).not.toContain('calc((100% - 56rem)')
  })
})
