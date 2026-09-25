import { defaultStageColor } from '@/lib/crmStageColors';
import { StateSelectContent } from '@/components/design-system/state-select-content';
import { SidebarPopoverSelect } from '@/components/pm/SidebarPopoverSelect';
import type { CRMPipelineStage } from '@/lib/crmTypes';
export function DealStageContent({ stage }: { stage: CRMPipelineStage }) {
  return (
    <StateSelectContent
      label={stage.name}
      color={stage.color || defaultStageColor(stage.stage_type, stage.position)}
    />
  );
}
export function DealStageSelect({
  stages,
  value,
  onChange,
  disabled,
  underline = false,
}: {
  stages: CRMPipelineStage[];
  value: string;
  onChange: (id: string) => void;
  disabled?: boolean;
  underline?: boolean;
}) {
  const selected = stages.find((s) => s.id === value);
  return (
    <SidebarPopoverSelect
      triggerLabel="Stage"
      value={value}
      options={stages.map((s) => ({ value: s.id, label: s.name }))}
      onChange={(id) => {
        if (id !== value) onChange(id);
      }}
      disabled={disabled}
      searchPlaceholder="Search stages..."
      triggerVariant={underline ? 'underline' : undefined}
      triggerClassName={underline ? 'w-full' : undefined}
      showChevron={underline}
      renderTrigger={() =>
        selected ? (
          <DealStageContent stage={selected} />
        ) : (
          <span>Select stage</span>
        )
      }
      renderOption={(id) => {
        const stage = stages.find((s) => s.id === id);
        return stage ? <DealStageContent stage={stage} /> : null;
      }}
    />
  );
}
