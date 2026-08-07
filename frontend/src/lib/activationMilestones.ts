export const ACTIVATION_MODULES = ['pm', 'docs', 'support', 'crm', 'automation'] as const;

export type ActivationModule = typeof ACTIVATION_MODULES[number];

export const MODULE_ACTIVATION_MILESTONES: Record<ActivationModule, {
  firstValue: readonly string[];
  repeatValue: readonly string[];
}> = {
  pm: {
    firstValue: ['first_task_created'],
    repeatValue: ['three_tasks_updated'],
  },
  docs: {
    firstValue: ['first_document_created', 'first_article_published'],
    repeatValue: ['three_documents_created_or_edited'],
  },
  support: {
    firstValue: ['first_reply_sent'],
    repeatValue: ['three_replies_sent', 'first_conversation_resolved'],
  },
  crm: {
    firstValue: ['first_contact_created', 'first_deal_created'],
    repeatValue: ['three_crm_records_updated'],
  },
  automation: {
    firstValue: ['first_automation_enabled', 'first_automation_executed'],
    repeatValue: ['three_automation_runs_succeeded'],
  },
};

export function isActivationModule(value: string): value is ActivationModule {
  return (ACTIVATION_MODULES as readonly string[]).includes(value);
}
