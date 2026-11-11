import { toSupportSystemEventSegments } from '@/components/support/supportSystemEvent'

const bolds = (segments: { text: string; bold?: boolean }[]) => segments.filter((s) => s.bold).map((s) => s.text)
const text = (segments: { text: string }[]) => segments.map((s) => s.text).join('')

test('teammate_joined bolds the actor name', () => {
  const segs = toSupportSystemEventSegments('teammate_joined', 'Waqar joined the conversation.', { extended: true })
  expect(bolds(segs)).toEqual(['Waqar'])
  expect(text(segs)).toBe('Waqar joined the conversation.')
})

test('multi-word actor names are fully emphasized', () => {
  const segs = toSupportSystemEventSegments('teammate_joined', 'Waqar Ahmed joined the conversation.', { extended: true })
  expect(bolds(segs)).toEqual(['Waqar Ahmed'])
})

test('mailbox_moved bolds the actor and the inbox name, and strips the quotes', () => {
  const segs = toSupportSystemEventSegments('mailbox_moved', "Waqar moved to inbox 'Sales'.", { extended: true })
  expect(bolds(segs)).toEqual(['Waqar', 'Sales'])
  expect(text(segs)).toBe('Waqar moved to inbox Sales.')
})

test('assigned bolds both the actor and the assignee', () => {
  const segs = toSupportSystemEventSegments('assigned', 'Azhar assigned this conversation to Jarek', { extended: true })
  expect(bolds(segs)).toEqual(['Azhar', 'Jarek'])
})

test('took bolds the actor', () => {
  expect(bolds(toSupportSystemEventSegments('took', 'Waqar took this conversation.', { extended: true }))).toEqual(['Waqar'])
})

test('triage_routed bolds the inbox name but NOT "Routing rule"', () => {
  const segs = toSupportSystemEventSegments('triage_routed', "Routing rule moved to inbox 'Sales'.", { extended: true })
  expect(bolds(segs)).toEqual(['Sales'])
  expect(text(segs)).toBe('Routing rule moved to inbox Sales.')
})

test('tag_added bolds the tag name', () => {
  expect(bolds(toSupportSystemEventSegments('tag_added', 'added tag VIP.', { extended: true }))).toEqual(['VIP'])
})

test('removed tags and Cc addresses are marked removed; added ones are not', () => {
  expect(toSupportSystemEventSegments('tag_removed', 'removed tag VIP.', { extended: true })).toContainEqual({ text: 'VIP', bold: true, removed: true })
  expect(toSupportSystemEventSegments('tag_added', 'added tag VIP.', { extended: true })).toContainEqual({ text: 'VIP', bold: true })
  const cc = toSupportSystemEventSegments('email_recipients_updated', 'Ada added new@example.com to Cc. Ada removed old@example.com from Cc.', { extended: true })
  expect(cc.filter((s) => s.removed).map((s) => s.text)).toEqual(['old@example.com'])
  expect(bolds(cc)).toEqual(['new@example.com', 'old@example.com'])
})

test('task_created bolds the task key and name', () => {
  const segs = toSupportSystemEventSegments('task_created', 'Ada created task #HLP-12: Fix the widget.', { extended: true })
  expect(bolds(segs)).toEqual(['Ada', '#HLP-12', 'Fix the widget'])
})

test('unknown event type returns a single plain segment', () => {
  expect(toSupportSystemEventSegments('mystery', 'something happened.', { extended: true })).toEqual([{ text: 'something happened.' }])
})
