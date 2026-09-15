import type { ReactNode } from 'react';
import { SettingsSection } from '@/components/settings/SettingsSection';

/** Shares settings section styling while preserving drafts and search deep links. */
export function CRMEmailSettingsSection({ title, children, optionId }: {
  title: string;
  optionId?: string;
  children: ReactNode;
}) {
  return (
    <SettingsSection title={title} optionId={optionId}>
      <div className="py-3">{children}</div>
    </SettingsSection>
  );
}
