import { render, screen } from '@testing-library/react'
import { HelpinLogo } from '../helpin-logo'

test('renders the Helpin brand lockup with theme-aware web assets', () => {
  const { container } = render(<HelpinLogo />)

  expect(screen.getByLabelText('Helpin')).toBeTruthy()
  expect(container.querySelectorAll('img')).toHaveLength(2)
  expect(screen.getByText('Helpin')).toBeTruthy()
})
