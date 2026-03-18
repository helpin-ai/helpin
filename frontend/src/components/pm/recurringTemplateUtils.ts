import type { RecurringTemplateConfig } from '@/lib/pmTypes';

const weekdayNames = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'];

export function formatRecurringRuleSummary(config: RecurringTemplateConfig) {
  if (config.schedule_type === 'completion') {
    if (config.completion_event === 'completed') {
      return 'Create next when marked completed';
    }
    return 'Create next when moved to done';
  }

  const every = config.interval && config.interval > 1 ? `Every ${config.interval}` : 'Every';

  switch (config.frequency) {
    case 'daily':
      return `${every} day${config.interval && config.interval > 1 ? 's' : ''}`;
    case 'monthly':
      return `${every} month${config.interval && config.interval > 1 ? 's' : ''}${config.day_of_month ? ` on day ${config.day_of_month}` : ''}`;
    case 'yearly':
      return `${every} year${config.interval && config.interval > 1 ? 's' : ''}${config.day_of_month ? ` on day ${config.day_of_month}` : ''}`;
    case 'weekly':
    default: {
      const days = (config.weekdays ?? []).map((weekday) => weekdayNames[weekday] ?? '').filter(Boolean).join(', ');
      return `${every} week${config.interval && config.interval > 1 ? 's' : ''}${days ? ` on ${days}` : ''}`;
    }
  }
}
