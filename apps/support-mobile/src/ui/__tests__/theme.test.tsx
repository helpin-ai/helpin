import { render, screen } from '@testing-library/react'
import { AppThemeProvider } from '../theme-provider'

// jsdom does not implement matchMedia; next-themes' `enableSystem` reads it
// on mount to detect the OS color scheme.
beforeAll(() => {
  window.matchMedia = window.matchMedia || ((query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addListener: () => {},
    removeListener: () => {},
    addEventListener: () => {},
    removeEventListener: () => {},
    dispatchEvent: () => false,
  })) as unknown as typeof window.matchMedia
})

test('renders children', () => {
  render(
    <AppThemeProvider>
      <div>hello world</div>
    </AppThemeProvider>,
  )
  expect(screen.getByText('hello world')).toBeDefined()
})

test('document root can carry the dark class next-themes drives', () => {
  document.documentElement.classList.remove('dark')
  expect(document.documentElement.classList.contains('dark')).toBe(false)

  document.documentElement.classList.add('dark')
  expect(document.documentElement.classList.contains('dark')).toBe(true)

  document.documentElement.classList.remove('dark')
})
