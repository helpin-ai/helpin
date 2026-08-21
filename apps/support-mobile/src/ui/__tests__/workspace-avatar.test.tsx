import { fireEvent, render, screen } from '@testing-library/react'
import { WorkspaceAvatar } from '../workspace-avatar'

test('prefers an uploaded workspace logo', () => {
  const { container } = render(
    <WorkspaceAvatar workspace={{ name: 'Acme Labs', logo_url: 'https://cdn.example.com/acme.png', website_url: 'acme.test' }} />,
  )
  expect(container.querySelector('img')?.getAttribute('src')).toBe('https://cdn.example.com/acme.png')
})

test('falls back from a broken logo to the workspace website favicon', () => {
  const { container } = render(
    <WorkspaceAvatar workspace={{ name: 'Acme Labs', logo_url: 'https://cdn.example.com/broken.png', website_url: 'https://www.acme.test/about' }} />,
  )
  const image = container.querySelector('img')!
  fireEvent.error(image)
  expect(image.getAttribute('src')).toContain('domain=acme.test')
})

test('falls back to initials when no visual source is available', () => {
  render(<WorkspaceAvatar workspace={{ name: 'Acme Labs' }} />)
  expect(screen.getByText('AL')).toBeDefined()
})
