import { validateEmail, validatePassword } from '../login-screen'

test.each([
  ['', 'Email is required'],
  ['   ', 'Email is required'],
  ['not-an-email', 'Enter a valid email address'],
  ['missing-domain@', 'Enter a valid email address'],
  ['a@b.com', null],
  ['  a@b.com  ', null],
])('validateEmail(%j) -> %j', (input, expected) => {
  expect(validateEmail(input)).toBe(expected)
})

test.each([
  ['', 'Password is required'],
  ['secret', null],
])('validatePassword(%j) -> %j', (input, expected) => {
  expect(validatePassword(input)).toBe(expected)
})
