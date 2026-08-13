"use client"

import * as React from "react"
import { Select as SelectPrimitive } from "radix-ui"

import { cn } from "@/lib/utils"
import { HugeiconsIcon } from "@hugeicons/react"
import { Tick02Icon, ArrowUp01Icon, ArrowDown01Icon } from "@hugeicons/core-free-icons"
import { ArrowUpDownIcon } from "@/lib/icons"
import { pickerTriggerVariants, type PickerTriggerVariant } from "@/components/ui/picker-trigger"

type SelectSize = "sm" | "default"
const SelectSizeContext = React.createContext<SelectSize>("default")

function Select({
  size = "default",
  ...props
}: React.ComponentProps<typeof SelectPrimitive.Root> & { size?: SelectSize }) {
  return (
    <SelectSizeContext.Provider value={size}>
      <SelectPrimitive.Root data-slot="select" {...props} />
    </SelectSizeContext.Provider>
  )
}

function SelectGroup({
  className,
  ...props
}: React.ComponentProps<typeof SelectPrimitive.Group>) {
  const size = React.useContext(SelectSizeContext)
  return (
    <SelectPrimitive.Group
      data-slot="select-group"
      className={cn(size === "sm" ? "scroll-my-0.5 p-0.5" : "scroll-my-1 p-1", className)}
      {...props}
    />
  )
}

function SelectValue({
  ...props
}: React.ComponentProps<typeof SelectPrimitive.Value>) {
  return <SelectPrimitive.Value data-slot="select-value" {...props} />
}

function SelectTrigger({
  className,
  size,
  variant = "default",
  children,
  ...props
}: React.ComponentProps<typeof SelectPrimitive.Trigger> & {
  size?: "sm" | "default"
  variant?: PickerTriggerVariant
}) {
  const ctxSize = React.useContext(SelectSizeContext)
  const resolvedSize = size ?? ctxSize
  return (
    <SelectPrimitive.Trigger
      data-slot="select-trigger"
      data-size={resolvedSize}
      data-variant={variant}
      className={cn(
        "flex w-fit items-center justify-between whitespace-nowrap aria-invalid:border-destructive aria-invalid:ring-3 aria-invalid:ring-destructive/20 data-placeholder:text-muted-foreground *:data-[slot=select-value]:line-clamp-1 *:data-[slot=select-value]:flex *:data-[slot=select-value]:items-center dark:aria-invalid:border-destructive/50 dark:aria-invalid:ring-destructive/40 [&_svg]:pointer-events-none [&_svg]:shrink-0",
        pickerTriggerVariants({ variant }),
        resolvedSize === "sm"
          ? "gap-1 px-2 py-1 text-xs data-[size=sm]:h-7 *:data-[slot=select-value]:gap-1 [&_svg:not([class*='size-'])]:size-3.5"
          : "gap-1.5 px-3 py-2 text-sm data-[size=default]:h-9 *:data-[slot=select-value]:gap-1.5 [&_svg:not([class*='size-'])]:size-4",
        className
      )}
      {...props}
    >
      {children}
      <SelectPrimitive.Icon asChild>
        <ArrowUpDownIcon className="pointer-events-none size-3.5 text-muted-foreground" />
      </SelectPrimitive.Icon>
    </SelectPrimitive.Trigger>
  )
}

function SelectContent({
  className,
  children,
  position = "item-aligned",
  align = "center",
  ...props
}: React.ComponentProps<typeof SelectPrimitive.Content>) {
  return (
    <SelectPrimitive.Portal>
      <SelectPrimitive.Content
        data-slot="select-content"
        data-align-trigger={position === "item-aligned"}
        className={cn(
                                  "z-50 max-h-(--radix-select-content-available-height) min-w-36 origin-(--radix-select-content-transform-origin) overflow-x-hidden overflow-y-auto rounded-xl border border-border bg-popover text-popover-foreground shadow-sm duration-100 data-[align-trigger=true]:animate-none data-[side=bottom]:slide-in-from-top-2 data-[side=left]:slide-in-from-right-2 data-[side=right]:slide-in-from-left-2 data-[side=top]:slide-in-from-bottom-2 data-open:animate-in data-open:fade-in-0 data-open:zoom-in-95 data-closed:animate-out data-closed:fade-out-0 data-closed:zoom-out-95",
                                  position === "popper" &&
                                    "data-[side=bottom]:translate-y-1 data-[side=left]:-translate-x-1 data-[side=right]:translate-x-1 data-[side=top]:-translate-y-1",
                                  className
                                )}
        position={position}
        align={align}
        {...props}
      >
        <SelectScrollUpButton />
        <SelectPrimitive.Viewport
          data-position={position}
          className={cn(
            "data-[position=popper]:h-(--radix-select-trigger-height) data-[position=popper]:w-full data-[position=popper]:min-w-(--radix-select-trigger-width)",
            position === "popper" && ""
          )}
        >
          {children}
        </SelectPrimitive.Viewport>
        <SelectScrollDownButton />
      </SelectPrimitive.Content>
    </SelectPrimitive.Portal>
  )
}

function SelectLabel({
  className,
  ...props
}: React.ComponentProps<typeof SelectPrimitive.Label>) {
  return (
    <SelectPrimitive.Label
      data-slot="select-label"
      className={cn("px-3 py-2.5 text-xs text-muted-foreground", className)}
      {...props}
    />
  )
}

function SelectItem({
  className,
  children,
  ...props
}: React.ComponentProps<typeof SelectPrimitive.Item>) {
  const size = React.useContext(SelectSizeContext)
  const isCompact = size === "sm"
  return (
    <SelectPrimitive.Item
      data-slot="select-item"
      className={cn(
        "relative flex w-full cursor-pointer items-center rounded-md outline-hidden select-none focus:bg-accent focus:text-accent-foreground data-disabled:pointer-events-none data-disabled:opacity-50 [&_svg]:pointer-events-none [&_svg]:shrink-0 *:[span]:last:flex *:[span]:last:items-center *:[span]:last:gap-2",
        isCompact
          ? "gap-1.5 py-1 pr-6 pl-1.5 text-xs [&_svg:not([class*='size-'])]:size-3.5"
          : "gap-2 py-1.5 pr-8 pl-2 text-sm [&_svg:not([class*='size-'])]:size-4",
        className
      )}
      {...props}
    >
      <span className={cn("pointer-events-none absolute flex items-center justify-center", isCompact ? "right-1.5 size-3.5" : "right-2 size-4")}>
        <SelectPrimitive.ItemIndicator>
          <HugeiconsIcon icon={Tick02Icon} strokeWidth={2} className="pointer-events-none" />
        </SelectPrimitive.ItemIndicator>
      </span>
      <SelectPrimitive.ItemText>{children}</SelectPrimitive.ItemText>
    </SelectPrimitive.Item>
  )
}

function SelectSeparator({
  className,
  ...props
}: React.ComponentProps<typeof SelectPrimitive.Separator>) {
  return (
    <SelectPrimitive.Separator
      data-slot="select-separator"
      className={cn(
        "pointer-events-none -mx-1.5 my-1.5 h-px bg-border",
        className
      )}
      {...props}
    />
  )
}

function SelectScrollUpButton({
  className,
  ...props
}: React.ComponentProps<typeof SelectPrimitive.ScrollUpButton>) {
  return (
    <SelectPrimitive.ScrollUpButton
      data-slot="select-scroll-up-button"
      className={cn(
        "z-10 flex cursor-default items-center justify-center bg-popover py-1 [&_svg:not([class*='size-'])]:size-4",
        className
      )}
      {...props}
    >
      <HugeiconsIcon icon={ArrowUp01Icon} strokeWidth={2} />
    </SelectPrimitive.ScrollUpButton>
  )
}

function SelectScrollDownButton({
  className,
  ...props
}: React.ComponentProps<typeof SelectPrimitive.ScrollDownButton>) {
  return (
    <SelectPrimitive.ScrollDownButton
      data-slot="select-scroll-down-button"
      className={cn(
        "z-10 flex cursor-default items-center justify-center bg-popover py-1 [&_svg:not([class*='size-'])]:size-4",
        className
      )}
      {...props}
    >
      <HugeiconsIcon icon={ArrowDown01Icon} strokeWidth={2} />
    </SelectPrimitive.ScrollDownButton>
  )
}

export {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectLabel,
  SelectScrollDownButton,
  SelectScrollUpButton,
  SelectSeparator,
  SelectTrigger,
  SelectValue,
}
