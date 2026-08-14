import { cva, type VariantProps } from "class-variance-authority"

const pickerTriggerVariants = cva(
  "outline-none transition-[color,background-color,border-color,box-shadow] disabled:cursor-not-allowed disabled:opacity-50",
  {
    variants: {
      variant: {
        default:
          "rounded-3xl border border-transparent bg-input/50 focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/30",
        ghost:
          "rounded-md border border-transparent bg-transparent hover:bg-muted focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/30",
        underline:
          "rounded-none border-x-0 border-t-0 border-b border-border bg-transparent hover:border-foreground focus-visible:border-foreground focus-visible:ring-0",
      },
    },
    defaultVariants: {
      variant: "default",
    },
  },
)

type PickerTriggerVariant = NonNullable<VariantProps<typeof pickerTriggerVariants>["variant"]>

export { pickerTriggerVariants, type PickerTriggerVariant }
