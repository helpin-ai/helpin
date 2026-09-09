import type { ElementType, ReactNode } from 'react';
import { QuickTooltip } from '@/components/ui/quick-tooltip';

export function DetailMetadataRow({
  icon: Icon,
  label,
  tooltip,
  children,
}: {
  icon: ElementType;
  label: string;
  tooltip?: string;
  children: ReactNode;
}) {
  const labelNode = <span className="mt-0.5 text-[12px] text-muted-foreground">{label}</span>;

  return (
    <>
      <Icon className="mt-0.5 h-3.5 w-3.5 shrink-0 text-muted-foreground" />
      {tooltip ? (
        <QuickTooltip label={tooltip} side="left">
          {labelNode}
        </QuickTooltip>
      ) : labelNode}
      <div className="min-w-0 text-[12px]">{children}</div>
    </>
  );
}

