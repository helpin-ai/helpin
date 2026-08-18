import type { ComponentProps, ElementType, ReactNode } from "react";

import { Button } from "@/components/ui/button";
import { DialogContent } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { QuickTooltip } from "@/components/ui/quick-tooltip";
import { Cancel01Icon } from "@/lib/icons";
import { cn } from "@/lib/utils";

export function CreateEntityDialogContent({
  className,
  children,
}: {
  className?: string;
  children: ReactNode;
}) {
  return (
    <DialogContent
      variant="flush"
      className={cn(
        "w-[1080px] max-w-[calc(100vw-2rem)] overflow-visible sm:max-w-[calc(100vw-2rem)]",
        className,
      )}
      showCloseButton={false}
    >
      {children}
    </DialogContent>
  );
}

export function CreateEntityModalFrame({
  className,
  children,
}: {
  className?: string;
  children: ReactNode;
}) {
  return (
    <div className={cn("flex h-[85vh] max-h-[960px] flex-col", className)}>
      {children}
    </div>
  );
}

export function CreateEntityModalHeader({
  title,
  onClose,
  children,
}: {
  title: ReactNode;
  onClose: () => void;
  children?: ReactNode;
}) {
  return (
    <div className="flex items-center justify-between border-b border-border/60 px-5 py-[15px]">
      <div className="flex min-w-0 items-center gap-3">
        <span className="text-[17px] font-semibold tracking-[-0.01em]">
          {title}
        </span>
        {children}
      </div>
      <Button
        variant="ghost"
        size="icon"
        className="h-8 w-8 shrink-0 rounded-md text-muted-foreground"
        onClick={onClose}
      >
        <Cancel01Icon className="h-4 w-4" />
        <span className="sr-only">Close</span>
      </Button>
    </div>
  );
}

export function CreateEntityModalBody({ children }: { children: ReactNode }) {
  return (
    <div className="grid min-h-0 flex-1 grid-cols-1 overflow-y-auto md:grid-cols-[minmax(0,1fr)_300px] md:overflow-hidden">
      {children}
    </div>
  );
}

export function CreateEntityModalMain({
  className,
  children,
}: {
  className?: string;
  children: ReactNode;
}) {
  return (
    <div className={cn("min-h-0 md:overflow-y-auto", className)}>
      {children}
    </div>
  );
}

export function CreateEntityModalSidebar({
  className,
  description,
  children,
}: {
  className?: string;
  description?: ReactNode;
  children: ReactNode;
}) {
  return (
    <aside
      className={cn(
        "min-h-0 overflow-y-auto border-t border-border/50 px-5 py-4 md:border-t-0 md:border-l",
        className,
      )}
    >
      {description ? (
        <p className="mb-4 text-ui text-muted-foreground">{description}</p>
      ) : null}
      {children}
    </aside>
  );
}

export function CreateEntityTitleInput({
  className,
  ...props
}: ComponentProps<typeof Input>) {
  return (
    <div className="border-b border-border/60 px-6 py-[17px] transition-colors focus-within:border-foreground/70">
      <Input
        variant="plain"
        className={cn(
          "text-xl font-medium tracking-[-0.015em] md:text-xl",
          className,
        )}
        {...props}
      />
    </div>
  );
}

export function CreateEntityMetadataGrid({
  className,
  children,
}: {
  className?: string;
  children: ReactNode;
}) {
  return (
    <div
      className={cn(
        "grid grid-cols-[16px_80px_1fr] items-center gap-x-3 gap-y-3",
        className,
      )}
    >
      {children}
    </div>
  );
}

export function CreateEntityMetadataRow({
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
  const labelNode = (
    <span className="self-center text-ui text-muted-foreground">{label}</span>
  );

  return (
    <>
      <Icon className="h-3.5 w-3.5 shrink-0 self-center text-muted-foreground" />
      {tooltip ? (
        <QuickTooltip label={tooltip} side="left">
          {labelNode}
        </QuickTooltip>
      ) : (
        labelNode
      )}
      <div className="min-w-0 self-center text-ui [&_button]:text-ui">
        {children}
      </div>
    </>
  );
}

export function CreateEntityModalFooter({
  className,
  children,
}: {
  className?: string;
  children: ReactNode;
}) {
  return (
    <div
      className={cn(
        "flex flex-wrap items-center justify-end gap-3 border-t border-border/60 px-5 py-3.5",
        className,
      )}
    >
      {children}
    </div>
  );
}
