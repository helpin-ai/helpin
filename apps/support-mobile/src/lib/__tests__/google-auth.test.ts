import {
  buildGoogleAuthStartUrl,
  googleAuthErrorMessage,
  parseGoogleAuthDeepLink,
} from '../google-auth'

test('builds native and hosted mobile Google start URLs', () => {
  expect(new URL(buildGoogleAuthStartUrl(true)).searchParams.get('client')).toBe('mobile_native')
  expect(new URL(buildGoogleAuthStartUrl(false)).searchParams.get('client')).toBe('mobile_web')
})

test('parses only the fixed Google auth deep-link route', () => {
  expect(parseGoogleAuthDeepLink('helpin://auth/google?code=one-time-code')).toEqual({ code: 'one-time-code' })
  expect(parseGoogleAuthDeepLink('helpin://auth/google?error=invalid_state')).toEqual({ error: 'invalid_state' })
  expect(parseGoogleAuthDeepLink('helpin://w/acme/support/conversation-1')).toBeNull()
  expect(parseGoogleAuthDeepLink('https://auth/google?code=leak')).toBeNull()
})

test('maps backend callback reasons to user-safe messages', () => {
  expect(googleAuthErrorMessage('invalid_state')).toContain('expired')
  expect(googleAuthErrorMessage('exchange_failed')).not.toContain('exchange')
})
