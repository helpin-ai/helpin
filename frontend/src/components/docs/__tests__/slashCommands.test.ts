import { describe, expect, it } from 'vitest'

import { slashCommands } from '../slash-commands'

describe('docs slash commands', () => {
  it('exposes direct entity embed commands while keeping the generic entity fallback', () => {
    expect(slashCommands.find((command) => command.title === 'Task')?.entityType).toBe('task')
    expect(slashCommands.find((command) => command.title === 'Epic')?.entityType).toBe('epic')
    expect(slashCommands.find((command) => command.title === 'Deal')?.entityType).toBe('deal')
    expect(slashCommands.find((command) => command.title === 'Contact')?.entityType).toBe('contact')
    expect(slashCommands.find((command) => command.title === 'Company')?.entityType).toBe('company')
    expect(slashCommands.find((command) => command.title === 'Conversation')?.entityType).toBe('support_conversation')

    const genericEntity = slashCommands.find((command) => command.title === 'Entity')
    expect(genericEntity).toBeTruthy()
    expect(genericEntity?.entityType).toBeUndefined()
  })
})
