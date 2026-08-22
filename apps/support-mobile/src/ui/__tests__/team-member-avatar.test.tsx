import { render, screen } from '@testing-library/react'
import { TeamMemberAvatar } from '../team-member-avatar'

test('prefers an uploaded teammate avatar', () => {
  render(
    <TeamMemberAvatar
      name="Ada Lovelace"
      member={{
        user_id: 'user-1',
        avatar_url: 'https://example.com/ada.png',
        avatar_style: 'personas',
        avatar_seed: 'generated-ada',
      }}
    />,
  )

  expect(screen.getByRole('img', { name: 'Ada Lovelace' }).getAttribute('src')).toBe('https://example.com/ada.png')
})

test('renders the same generated teammate avatar profile used by web', async () => {
  render(
    <TeamMemberAvatar
      name="Grace Hopper"
      member={{
        user_id: 'user-2',
        avatar_style: 'micah',
        avatar_seed: 'generated-grace',
        avatar_background_mode: 'color',
        avatar_background_color: '#3b82f6',
      }}
    />,
  )

  expect((await screen.findByRole('img', { name: 'Grace Hopper' })).getAttribute('src')).toMatch(/^data:image\/svg\+xml/)
})

test('falls back to a single colored initial when no profile avatar exists', () => {
  render(<TeamMemberAvatar name="Linus Torvalds" fallbackSeed="user-3" initialCount={1} />)
  expect(screen.getByText('L')).toBeDefined()
})
