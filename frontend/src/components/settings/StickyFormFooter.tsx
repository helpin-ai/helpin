import type { ComponentProps } from 'react';
import { SettingsSaveBar } from './SettingsSaveBar';

/** Compatibility wrapper for settings forms using the previous footer API. */
export function StickyFormFooter(props: ComponentProps<typeof SettingsSaveBar>) {
  return <SettingsSaveBar {...props} />;
}
