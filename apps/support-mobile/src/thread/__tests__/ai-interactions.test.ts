import { buildQuestionResponse, parseInteractionQuestions, questionsAnswered } from '../ai-interactions'

test('builds the shared request_user_input response contract', () => {
  const questions = parseInteractionQuestions({ questions: [{ id: 'tone', question: 'Choose tone', options: [{ label: 'Friendly', description: 'Warm' }], isOther: true }] })
  expect(questions[0]).toMatchObject({ id: 'tone', schema: 'shared', options: [{ value: 'Friendly', label: 'Friendly' }] })
  expect(questionsAnswered(questions, { tone: { value: 'Friendly' } })).toBe(true)
  expect(buildQuestionResponse(questions, { tone: { value: 'Friendly' } })).toEqual({ answers: { tone: { answers: ['Friendly'] } } })
})

test('builds the Helpin request_user_input response contract with freetext', () => {
  const questions = parseInteractionQuestions({ questions: [{ id: 'scope', text: 'Choose scope', options: [{ value: 'other', label: 'Other', freetext: true }] }] })
  const answers = { scope: { value: 'other', freetext: 'Billing only' } }
  expect(questionsAnswered(questions, answers)).toBe(true)
  expect(buildQuestionResponse(questions, answers)).toEqual({
    content: '- scope: Choose scope -> Other (Billing only)',
    answers: [{ question_id: 'scope', question: 'Choose scope', selected_value: 'other', selected_label: 'Other', freetext: 'Billing only' }],
  })
})
