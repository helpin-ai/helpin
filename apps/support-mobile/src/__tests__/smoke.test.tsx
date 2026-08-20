import { render, screen } from '@testing-library/react'
import { App } from '../main'

test('renders app shell at the login route', async () => {
  window.history.pushState({}, '', '/login')
  render(<App />)
  expect(await screen.findByText('Helpin')).toBeDefined()
  expect(screen.getByRole('button', { name: 'Continue with Google' })).toBeDefined()
  expect(screen.getByRole('button', { name: 'Sign in with passkey' })).toBeDefined()
})
