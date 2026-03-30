import { PipelineSettings } from '@/components/crm/PipelineSettings';
import { SettingsPageFrame } from './SettingsPageFrame';

export function CRMPipelinesSettingsPage() {
  return (
    <SettingsPageFrame section="crm-pipelines">
      {() => <PipelineSettings />}
    </SettingsPageFrame>
  );
}
