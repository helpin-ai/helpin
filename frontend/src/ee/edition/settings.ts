import type { IconComponent, SettingsSectionMeta } from '@/lib/settingsSections';

export function billingSettingsSections(_icon: IconComponent): SettingsSectionMeta[] {
  return [{ id: 'billing', label: 'Billing', description: 'Manage this workspace plan, AI usage, payment methods, and invoices.', icon: _icon, group: 'Workspace' }];
}
