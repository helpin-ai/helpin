export type UpgradeRequiredKind =
  | 'custom_agents'
  | 'automation_flows'
  | 'agent_scheduling'
  | 'ai_usage'
  | 'workspace_locked'
  | 'contacts_limit'
  | 'documents_limit'

export interface UpgradeRequiredReason {
  kind: UpgradeRequiredKind
  title: string
  message: string
  primaryBenefit: string
}

const GROWTH_TITLE = 'Upgrade to the Growth plan'

export function getUpgradeRequiredReason(error: unknown): UpgradeRequiredReason | null {
  const message = errorMessage(error).toLowerCase()
  if (!message) return null

  if (message.includes('custom ai agents') && message.includes('growth')) {
    return {
      kind: 'custom_agents',
      title: GROWTH_TITLE,
      message: 'Custom agents are available on the Growth plan.',
      primaryBenefit: 'Custom AI agents',
    }
  }
  if (message.includes('automation flows') && message.includes('growth')) {
    return {
      kind: 'automation_flows',
      title: GROWTH_TITLE,
      message: 'Automation flows are available on the Growth plan.',
      primaryBenefit: 'Automation flows',
    }
  }
  if (message.includes('scheduled agents') || message.includes('agent scheduling')) {
    return {
      kind: 'agent_scheduling',
      title: GROWTH_TITLE,
      message: 'Scheduled agents and cron flows are available on the Growth plan.',
      primaryBenefit: 'Scheduled agents and cron',
    }
  }
  if (message.includes('ai usage exhausted') || message.includes('extra ai usage is not available')) {
    return {
      kind: 'ai_usage',
      title: 'Upgrade to continue',
      message: 'AI usage for this workspace is exhausted.',
      primaryBenefit: 'Larger included AI usage allowance',
    }
  }
  if (message.includes('workspace is locked')) {
    return {
      kind: 'workspace_locked',
      title: 'Reactivate workspace',
      message: 'This workspace is locked until billing is reactivated.',
      primaryBenefit: 'Reactivate workspace access',
    }
  }
  if (message.includes('starter includes up to 5,000 contacts')) {
    return {
      kind: 'contacts_limit',
      title: GROWTH_TITLE,
      message: 'The Starter plan includes up to 5,000 CRM contacts.',
      primaryBenefit: 'Unlimited CRM contacts',
    }
  }
  if (message.includes('starter includes up to 500 documents')) {
    return {
      kind: 'documents_limit',
      title: GROWTH_TITLE,
      message: 'The Starter plan includes up to 500 documents.',
      primaryBenefit: 'Unlimited documents',
    }
  }
  return null
}

function errorMessage(error: unknown): string {
  if (!error) return ''
  if (typeof error === 'string') return error
  if (error instanceof Error) return error.message
  if (typeof error === 'object' && 'error' in error && typeof error.error === 'string') return error.error
  if (typeof error === 'object' && 'message' in error && typeof error.message === 'string') return error.message
  return ''
}
