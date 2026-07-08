import { render, screen, waitFor } from '@testing-library/react'
import { useTheme } from 'next-themes'
import { useEffect } from 'react'
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

afterEach(() => {
  // next-themes persists the selected theme and stamps the root element;
  // reset both so tests stay independent.
  window.localStorage.removeItem('theme')
  document.documentElement.classList.remove('dark', 'light')
  document.documentElement.style.removeProperty('color-scheme')
})

test('renders children', () => {
  render(
    <AppThemeProvider>
      <div>hello world</div>
    </AppThemeProvider>,
  )
  expect(screen.getByText('hello world')).toBeDefined()
})

function ForceDark() {
  const { setTheme } = useTheme()
  useEffect(() => {
    setTheme('dark')
  }, [setTheme])
  return null
}

test('next-themes drives the dark class on the document root', async () => {
  expect(document.documentElement.classList.contains('dark')).toBe(false)

  render(
    <AppThemeProvider>
      <ForceDark />
    </AppThemeProvider>,
  )

  // Fails if AppThemeProvider does not use attribute="class" (e.g. a
  // data-theme attribute would leave classList untouched).
  await waitFor(() => {
    expect(document.documentElement.classList.contains('dark')).toBe(true)
  })
})
