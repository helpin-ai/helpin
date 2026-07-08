import { render, screen } from '@testing-library/react'
import { Badge } from '../badge'

test.each([
  ['neutral', 'bg-muted'],
  ['primary', 'bg-primary/10'],
  ['warning', 'bg-amber-500/15'],
  ['success', 'bg-emerald-500/15'],
  ['destructive', 'bg-destructive/15'],
] as const)('renders the %s tone class', (tone, expectedClass) => {
  render(<Badge tone={tone}>Open</Badge>)
  expect(screen.getByText('Open').className).toContain(expectedClass)
})
