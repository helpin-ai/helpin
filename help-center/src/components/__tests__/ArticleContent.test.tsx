import { render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ArticleContent } from '@/components/ArticleContent'

describe('ArticleContent', () => {
  const writeText = vi.fn().mockResolvedValue(undefined)

  beforeEach(() => {
    vi.stubGlobal('navigator', {
      clipboard: {
        writeText,
      },
    })
  })

  afterEach(() => {
    writeText.mockClear()
    vi.unstubAllGlobals()
  })

  it('renders article HTML and enhances headings and code blocks', async () => {
    render(
      <ArticleContent
        html={`
          <h2>Getting Started</h2>
          <h4>Step 1</h4>
          <div class="docs-callout docs-callout--yellow"><p>Important note</p></div>
          <pre><code>console.log("hi")</code></pre>
        `}
      />,
    )

    expect(screen.getByText('Step 1')).not.toBeNull()
    expect(screen.getByText('Important note')).not.toBeNull()

    await waitFor(() => {
      const heading = screen.getByText('Getting Started')
      expect(heading.getAttribute('id')).toBe('getting-started')
      expect(screen.getByRole('button', { name: 'Copy code' })).not.toBeNull()
    })
  })
})
