import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { PropsWithChildren } from 'react'
import { describe, expect, it, vi } from 'vitest'

import { NavTree } from '@/components/navigation/NavTree'
import type { NavItem } from '@/lib/types'

vi.mock('@tanstack/react-router', () => ({
  useRouterState: ({ select }: { select: (state: { location: { pathname: string } }) => string }) =>
    select({ location: { pathname: '/' } }),
}))

vi.mock('@/contexts/DocsContext', () => ({
  useDocsContext: () => ({ enabledLocales: ['en'] }),
}))

vi.mock('@/components/DocsLink', () => ({
  DocsLink: ({ children, to, ...props }: PropsWithChildren<{ to: string }>) => (
    <a href={to} {...props}>{children}</a>
  ),
}))

vi.mock('@/components/PublicIcon', () => ({
  PublicIcon: () => null,
}))

function collection(id: string, name: string, articleName: string, position: number): NavItem {
  return {
    id,
    name,
    slug: id,
    public_id: `${id}-public`,
    icon: null,
    parent_collection_id: null,
    depth: 0,
    position,
    articles: [{
      id: `${id}-article`,
      title: articleName,
      slug: `${id}-article`,
      public_id: `${id}-article-public`,
      position: 0,
      published_at: null,
    }],
  }
}

describe('NavTree root collections', () => {
  it('opens the first root by default and lets visitors expand another root', async () => {
    const user = userEvent.setup()
    render(
      <NavTree
        locale="en"
        navigation={[
          collection('first', 'First collection', 'First article', 0),
          collection('second', 'Second collection', 'Second article', 1),
        ]}
      />,
    )

    const first = screen.getByRole('button', { name: 'First collection' })
    const second = screen.getByRole('button', { name: 'Second collection' })
    expect(first.className).toContain('w-full')
    expect(first.className).toContain('min-w-0')
    expect(first.className).toContain('pr-4')

    expect(first.getAttribute('aria-expanded')).toBe('true')
    expect(second.getAttribute('aria-expanded')).toBe('false')
    expect(screen.getByText('First article')).toBeTruthy()

    await user.click(second)

    expect(second.getAttribute('aria-expanded')).toBe('true')
    expect(screen.getByText('Second article')).toBeTruthy()
  })
})
