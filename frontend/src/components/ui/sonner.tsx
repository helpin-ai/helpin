"use client"

import { useTheme } from "next-themes"
import { Toaster as Sonner, type ToasterProps } from "sonner"
import { HugeiconsIcon } from "@hugeicons/react"
import { CheckmarkCircle02Icon, InformationCircleIcon, Alert02Icon, MultiplicationSignCircleIcon, Loading03Icon } from "@hugeicons/core-free-icons"

const Toaster = ({ ...props }: ToasterProps) => {
  const { theme = "system" } = useTheme()

  return (
    <Sonner
      theme={theme as ToasterProps["theme"]}
      closeButton
      duration={6000}
      className="toaster group"
      icons={{
        success: (
          <HugeiconsIcon icon={CheckmarkCircle02Icon} strokeWidth={2} className="size-4" />
        ),
        info: (
          <HugeiconsIcon icon={InformationCircleIcon} strokeWidth={2} className="size-4" />
        ),
        warning: (
          <HugeiconsIcon icon={Alert02Icon} strokeWidth={2} className="size-4" />
        ),
        error: (
          <HugeiconsIcon icon={MultiplicationSignCircleIcon} strokeWidth={2} className="size-4" />
        ),
        loading: (
          <HugeiconsIcon icon={Loading03Icon} strokeWidth={2} className="size-4 animate-spin" />
        ),
      }}
      style={
        {
          "--normal-bg": "var(--popover)",
          "--normal-text": "var(--popover-foreground)",
          "--normal-border": "var(--border)",
          "--border-radius": "var(--radius)",
        } as React.CSSProperties
      }
      toastOptions={{
        classNames: {
          toast: "cn-toast group/toast !items-start !gap-3 !border-border/80 !border-l-[3px] !bg-popover/95 !px-4 !py-3 !shadow-lg backdrop-blur data-[type=success]:!border-l-emerald-500 data-[type=info]:!border-l-sky-500 data-[type=warning]:!border-l-amber-500 data-[type=error]:!border-l-red-500",
          icon: "mt-0.5 !h-5 !w-5 rounded-full p-0.5 group-data-[type=success]/toast:bg-emerald-50 group-data-[type=success]/toast:text-emerald-600 group-data-[type=info]/toast:bg-sky-50 group-data-[type=info]/toast:text-sky-600 group-data-[type=warning]/toast:bg-amber-50 group-data-[type=warning]/toast:text-amber-600 group-data-[type=error]/toast:bg-red-50 group-data-[type=error]/toast:text-red-600 dark:group-data-[type=success]/toast:bg-emerald-950/60 dark:group-data-[type=success]/toast:text-emerald-300 dark:group-data-[type=info]/toast:bg-sky-950/60 dark:group-data-[type=info]/toast:text-sky-300 dark:group-data-[type=warning]/toast:bg-amber-950/60 dark:group-data-[type=warning]/toast:text-amber-300 dark:group-data-[type=error]/toast:bg-red-950/60 dark:group-data-[type=error]/toast:text-red-300",
          content: "min-w-0 gap-1",
          title: "text-sm font-medium leading-5 text-popover-foreground",
          description: "text-xs leading-5 text-muted-foreground",
          closeButton: "opacity-0 transition-opacity group-hover/toast:opacity-100 group-focus-within/toast:opacity-100",
        },
      }}
      {...props}
    />
  )
}

export { Toaster }
