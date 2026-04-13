import type { ConversationStatus, ConversationPriority } from '@/lib/pmTypes';

export const STATUS_COLORS: Record<ConversationStatus, string> = {
  open: 'bg-blue-100 text-blue-700',
  waiting_on_customer: 'bg-purple-100 text-purple-700',
  resolved: 'bg-green-100 text-green-700',
  spam: 'bg-red-100 text-red-600',
};

export const STATUS_LABELS: Record<ConversationStatus, string> = {
  open: 'Open',
  waiting_on_customer: 'Waiting on Customer',
  resolved: 'Resolved',
  spam: 'Spam',
};

export const PRIORITY_COLORS: Record<ConversationPriority, string> = {
  low: 'bg-gray-100 text-gray-600',
  medium: 'bg-blue-100 text-blue-700',
  high: 'bg-amber-100 text-amber-700',
  urgent: 'bg-red-100 text-red-700',
};

export const PRIORITY_LABELS: Record<ConversationPriority, string> = {
  low: 'Low',
  medium: 'Medium',
  high: 'High',
  urgent: 'Urgent',
};
