import * as React from "react"
import { cva, type VariantProps } from "class-variance-authority"
import { Tabs as TabsPrimitive } from "radix-ui"

import { cn } from "@/lib/utils"

function Tabs({
  className,
  orientation = "horizontal",
  ...props
}: React.ComponentProps<typeof TabsPrimitive.Root>) {
  return (
    <TabsPrimitive.Root
      data-slot="tabs"
      data-orientation={orientation}
      className={cn(
        "group/tabs flex gap-2 data-horizontal:flex-col",
        className
      )}
      {...props}
    />
  )
}

const tabsListVariants = cva(
  "group/tabs-list inline-flex w-fit items-center justify-center rounded-full p-1 text-muted-foreground group-data-horizontal/tabs:h-9 group-data-vertical/tabs:h-fit group-data-vertical/tabs:flex-col group-data-vertical/tabs:rounded-2xl data-[variant=line]:rounded-none data-[variant=quiet]:h-auto data-[variant=quiet]:rounded-none data-[variant=quiet]:border-b data-[variant=quiet]:border-quiet-divider-strong",
  {
    variants: {
      variant: {
        default: "bg-muted",
        line: "gap-1 bg-transparent",
        quiet: "justify-start gap-6 bg-transparent p-0",
      },
    },
    defaultVariants: {
      variant: "default",
    },
  }
)

function TabsList({
  className,
  variant = "default",
  ...props
}: React.ComponentProps<typeof TabsPrimitive.List> &
  VariantProps<typeof tabsListVariants>) {
  return (
    <TabsPrimitive.List
      data-slot="tabs-list"
      data-variant={variant}
      className={cn(tabsListVariants({ variant }), className)}
      {...props}
    />
  )
}

function TabsTrigger({
  className,
  ...props
}: React.ComponentProps<typeof TabsPrimitive.Trigger>) {
  return (
    <TabsPrimitive.Trigger
      data-slot="tabs-trigger"
      className={cn(
        "relative inline-flex h-[calc(100%-1px)] flex-1 items-center justify-center gap-2 rounded-full border border-transparent! px-3 py-1 text-sm font-medium whitespace-nowrap text-foreground/60 transition-all group-data-vertical/tabs:w-full group-data-vertical/tabs:justify-start group-data-vertical/tabs:rounded-2xl group-data-vertical/tabs:px-3 group-data-vertical/tabs:py-1.5 hover:text-foreground focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 focus-visible:outline-1 focus-visible:outline-ring disabled:pointer-events-none disabled:opacity-50 has-data-[icon=inline-end]:pr-2 has-data-[icon=inline-start]:pl-2 dark:text-muted-foreground dark:hover:text-foreground [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4",
        "group-data-[variant=line]/tabs-list:bg-transparent group-data-[variant=line]/tabs-list:data-active:bg-transparent dark:group-data-[variant=line]/tabs-list:data-active:border-transparent dark:group-data-[variant=line]/tabs-list:data-active:bg-transparent",
        "group-data-[variant=quiet]/tabs-list:h-auto group-data-[variant=quiet]/tabs-list:flex-none group-data-[variant=quiet]/tabs-list:rounded-none group-data-[variant=quiet]/tabs-list:border-x-0! group-data-[variant=quiet]/tabs-list:border-t-0! group-data-[variant=quiet]/tabs-list:border-b-0! group-data-[variant=quiet]/tabs-list:bg-transparent group-data-[variant=quiet]/tabs-list:px-0.5 group-data-[variant=quiet]/tabs-list:py-[11px] group-data-[variant=quiet]/tabs-list:text-[14px] group-data-[variant=quiet]/tabs-list:font-medium group-data-[variant=quiet]/tabs-list:text-quiet-text-tertiary group-data-[variant=quiet]/tabs-list:hover:text-quiet-text-primary group-data-[variant=quiet]/tabs-list:focus-visible:border-quiet-text-primary! group-data-[variant=quiet]/tabs-list:focus-visible:ring-0 group-data-[variant=quiet]/tabs-list:focus-visible:outline-none group-data-[variant=quiet]/tabs-list:data-active:bg-transparent group-data-[variant=quiet]/tabs-list:data-active:font-semibold group-data-[variant=quiet]/tabs-list:data-active:text-quiet-text-primary dark:group-data-[variant=quiet]/tabs-list:data-active:border-transparent dark:group-data-[variant=quiet]/tabs-list:data-active:bg-transparent",
        "data-active:bg-background data-active:text-foreground dark:data-active:border-input dark:data-active:bg-input/30 dark:data-active:text-foreground",
        "after:absolute after:bg-foreground after:opacity-0 after:transition-opacity group-data-horizontal/tabs:after:inset-x-0 group-data-horizontal/tabs:after:bottom-[-5px] group-data-horizontal/tabs:after:h-0.5 group-data-vertical/tabs:after:inset-y-0 group-data-vertical/tabs:after:-right-1 group-data-vertical/tabs:after:w-0.5 group-data-[variant=line]/tabs-list:data-active:after:opacity-100 group-data-[variant=quiet]/tabs-list:after:bottom-0 group-data-[variant=quiet]/tabs-list:after:bg-quiet-text-primary group-data-[variant=quiet]/tabs-list:data-active:after:opacity-100",
        className
      )}
      {...props}
    />
  )
}

function TabsContent({
  className,
  ...props
}: React.ComponentProps<typeof TabsPrimitive.Content>) {
  return (
    <TabsPrimitive.Content
      data-slot="tabs-content"
      className={cn("flex-1 text-sm outline-none", className)}
      {...props}
    />
  )
}

export { Tabs, TabsList, TabsTrigger, TabsContent, tabsListVariants }
