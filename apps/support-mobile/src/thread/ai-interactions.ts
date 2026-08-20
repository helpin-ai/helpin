import type { SupportRunInteraction } from '@helpin-ai/support-core'

export interface MobileInteractionQuestion {
  id: string
  prompt: string
  secret: boolean
  allowOther: boolean
  schema: 'shared' | 'helpin'
  options: Array<{ value: string; label: string; description?: string; freetext?: boolean }>
}

function record(value: unknown): Record<string, unknown> | null {
  return value && typeof value === 'object' && !Array.isArray(value) ? value as Record<string, unknown> : null
}

export function interactionId(interaction: SupportRunInteraction) {
  return interaction.interaction_id || interaction.id || ''
}

export function parseInteractionQuestions(payload: Record<string, unknown>): MobileInteractionQuestion[] {
  return (Array.isArray(payload.questions) ? payload.questions : []).flatMap((raw) => {
    const question = record(raw)
    if (!question) return []
    const id = typeof question.id === 'string' ? question.id.trim() : ''
    const sharedPrompt = typeof question.question === 'string' ? question.question.trim() : ''
    const helpinPrompt = typeof question.text === 'string' ? question.text.trim() : ''
    const prompt = sharedPrompt || helpinPrompt
    if (!id || !prompt) return []
    const schema = sharedPrompt ? 'shared' as const : 'helpin' as const
    const options = (Array.isArray(question.options) ? question.options : []).flatMap((rawOption) => {
      const option = record(rawOption)
      if (!option) return []
      const label = typeof option.label === 'string' ? option.label.trim() : ''
      const value = schema === 'shared' ? label : typeof option.value === 'string' ? option.value.trim() : ''
      if (!label || !value) return []
      return [{
        value,
        label,
        description: typeof option.description === 'string' ? option.description.trim() : undefined,
        freetext: option.freetext === true,
      }]
    })
    return [{
      id, prompt, schema, options,
      secret: question.isSecret === true,
      allowOther: question.isOther === true,
    }]
  })
}

export function buildQuestionResponse(
  questions: MobileInteractionQuestion[],
  answers: Record<string, { value?: string; freetext?: string }>,
) {
  if (questions[0]?.schema === 'shared') {
    const result: Record<string, { answers: string[] }> = {}
    for (const question of questions) {
      const answer = answers[question.id]
      const value = !question.options.length || answer?.value === '__other__'
        ? answer?.freetext?.trim()
        : answer?.value?.trim()
      if (value) result[question.id] = { answers: [value] }
    }
    return { answers: result }
  }
  const entries = questions.flatMap((question) => {
    const answer = answers[question.id]
    const option = question.options.find((candidate) => candidate.value === answer?.value)
    if (!option) return []
    return [{
      question_id: question.id,
      question: question.prompt,
      selected_value: option.value,
      selected_label: option.label,
      freetext: option.freetext ? answer?.freetext?.trim() : undefined,
    }]
  })
  return {
    content: entries.map((entry) => entry.freetext
      ? `- ${entry.question_id}: ${entry.question} -> ${entry.selected_label} (${entry.freetext})`
      : `- ${entry.question_id}: ${entry.question} -> ${entry.selected_label}`).join('\n'),
    answers: entries,
  }
}

export function questionsAnswered(
  questions: MobileInteractionQuestion[],
  answers: Record<string, { value?: string; freetext?: string }>,
) {
  return questions.length > 0 && questions.every((question) => {
    const answer = answers[question.id]
    if (!question.options.length || answer?.value === '__other__') return !!answer?.freetext?.trim()
    const option = question.options.find((candidate) => candidate.value === answer?.value)
    return !!option && (!option.freetext || !!answer?.freetext?.trim())
  })
}
