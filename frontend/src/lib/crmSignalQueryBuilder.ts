import type { QueryBuilderFieldDefinition } from '@/lib/queryBuilder';

interface SignalFilterOption {
  id: string;
  label: string;
}

const signalDomains = [
  { value: 'conversation', label: 'Conversation' },
  { value: 'web_behavior', label: 'Web behavior' },
  { value: 'product_usage', label: 'Product usage' },
  { value: 'support', label: 'Support' },
  { value: 'delivery', label: 'Delivery' },
  { value: 'relationship', label: 'Relationship' },
  { value: 'market', label: 'Market' },
];

export function buildCRMSignalQueryFields(
  owners: SignalFilterOption[],
  accounts: SignalFilterOption[],
): QueryBuilderFieldDefinition[] {
  return [
    { field: 'owner_member_id', label: 'Owner', type: 'member', options: owners.map(({ id, label }) => ({ value: id, label })) },
    { field: 'account_id', label: 'Account', type: 'enum', options: accounts.map(({ id, label }) => ({ value: id, label })) },
    { field: 'domain', label: 'Evidence domain', type: 'enum', options: signalDomains },
    { field: 'polarity', label: 'Direction', type: 'enum', options: [
      { value: 'positive', label: 'Momentum' },
      { value: 'negative', label: 'Risk' },
      { value: 'neutral', label: 'Neutral' },
    ] },
    { field: 'trust', label: 'Identity trust', type: 'enum', options: [
      { value: 'verified', label: 'Verified' },
      { value: 'probabilistic', label: 'Probabilistic' },
      { value: 'untrusted', label: 'Untrusted' },
      { value: 'unknown', label: 'Unknown' },
    ] },
    { field: 'signal_type', label: 'Signal type', type: 'enum', options: [
      { value: 'buying_intent', label: 'Buying intent' },
      { value: 'objection', label: 'Objection' },
      { value: 'competitor_mention', label: 'Competitor mention' },
      { value: 'budget_signal', label: 'Budget signal' },
      { value: 'timeline_signal', label: 'Timeline signal' },
      { value: 'champion_signal', label: 'Champion signal' },
      { value: 'risk_signal', label: 'Risk signal' },
    ] },
    { field: 'detected_at', label: 'Detected at', type: 'date' },
  ];
}
