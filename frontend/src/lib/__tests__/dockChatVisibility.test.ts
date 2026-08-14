import { describe, expect, it } from 'vitest';
import { dockChatModuleForContext } from '@/lib/dockTypes';
import type { CommandBarPageContext } from '@/lib/pmTypes';

const context = (entity_type: CommandBarPageContext['entity_type']): CommandBarPageContext => ({
  entity_type,
  entity_id: 'entity-1',
  display_title: 'Example',
});

describe('dockChatModuleForContext', () => {
  it.each([
    ['support_conversation', 'support'],
    ['crm_contact', 'crm'],
    ['crm_deal', 'crm'],
    ['task', 'pm'],
    ['epic', 'pm'],
    ['repository', 'pm'],
    ['document', 'docs'],
    ['workspace', null],
  ] as const)('maps %s to %s', (entityType, expectedModule) => {
    expect(dockChatModuleForContext(context(entityType))).toBe(expectedModule);
  });
});
