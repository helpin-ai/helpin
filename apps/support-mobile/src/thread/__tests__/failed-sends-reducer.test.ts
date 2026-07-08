import { failedSendsReducer, type FailedSend } from '../failed-sends-reducer'

test('add appends a failed send to the end of the list', () => {
  const first: FailedSend = { id: '1', content: 'First', mode: 'reply' }
  const second: FailedSend = { id: '2', content: 'Second', mode: 'note' }

  let state = failedSendsReducer([], { type: 'add', failedSend: first })
  state = failedSendsReducer(state, { type: 'add', failedSend: second })

  expect(state).toEqual([first, second])
})

test('remove drops only the matching id, preserving order of the rest', () => {
  const first: FailedSend = { id: '1', content: 'First', mode: 'reply' }
  const second: FailedSend = { id: '2', content: 'Second', mode: 'note' }
  const third: FailedSend = { id: '3', content: 'Third', mode: 'reply' }

  const state = failedSendsReducer([first, second, third], { type: 'remove', id: '2' })

  expect(state).toEqual([first, third])
})

test('remove is a no-op when the id is not present', () => {
  const first: FailedSend = { id: '1', content: 'First', mode: 'reply' }

  const state = failedSendsReducer([first], { type: 'remove', id: 'missing' })

  expect(state).toEqual([first])
})

test('does not mutate the input array', () => {
  const first: FailedSend = { id: '1', content: 'First', mode: 'reply' }
  const original = [first]

  failedSendsReducer(original, { type: 'add', failedSend: { id: '2', content: 'Second', mode: 'reply' } })

  expect(original).toEqual([first])
})
