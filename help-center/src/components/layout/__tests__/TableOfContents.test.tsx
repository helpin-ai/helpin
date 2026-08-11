import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'

import { TableOfContents } from '@/components/layout/TableOfContents'

describe('TableOfContents', () => {
  it('jumps immediately so user scrolling is not opposed by a smooth-scroll animation', async () => {
    const user = userEvent.setup()
    const heading = document.createElement('h2')
    heading.id = 'faqs'
    const scrollIntoView = vi.fn()
    heading.scrollIntoView = scrollIntoView
    document.body.appendChild(heading)

    try {
      render(
        <TableOfContents
          items={[
            {
              id: 'faqs',
              text: 'FAQs',
              level: 2,
            },
          ]}
        />,
      )

      await user.click(screen.getByRole('link', { name: 'FAQs' }))

      expect(scrollIntoView).toHaveBeenCalledWith({
        behavior: 'auto',
        block: 'start',
      })
    } finally {
      heading.remove()
    }
  })
})
