import { classifyPasskeyError } from '../passkey-service'

test.each([
  ['SecurityError', 'not available for this app origin', false],
  ['NotSupportedError', 'not supported on this device', false],
  ['AbortError', 'cancelled', true],
] as const)('classifies %s passkey failures', (name, message, cancelled) => {
  const result = classifyPasskeyError(new DOMException('', name))
  expect(result.message).toContain(message)
  expect(result.cancelled).toBe(cancelled)
})
