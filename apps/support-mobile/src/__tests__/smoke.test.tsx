import { render, screen } from '@testing-library/react'
import { App } from '../main'

test('renders app shell', () => {
  render(<App />)
  expect(screen.getByText('Helpin Support')).toBeDefined()
})
