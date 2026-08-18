import { cva } from "class-variance-authority"

const inputVariants = cva(
  "w-full min-w-0 text-base outline-none disabled:pointer-events-none disabled:cursor-not-allowed disabled:opacity-50 md:text-sm",
  {
    variants: {
      variant: {
        default:
          "h-9 rounded-3xl border border-transparent bg-input/50 px-3 py-1 transition-[color,box-shadow,background-color] file:inline-flex file:h-7 file:border-0 file:bg-transparent file:text-sm file:font-medium file:text-foreground placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/30 aria-invalid:border-destructive aria-invalid:ring-3 aria-invalid:ring-destructive/20 dark:aria-invalid:border-destructive/50 dark:aria-invalid:ring-destructive/40",
        plain:
          "h-auto rounded-none border-0 bg-transparent p-0 shadow-none placeholder:text-muted-foreground focus-visible:ring-0",
      },
    },
    defaultVariants: {
      variant: "default",
    },
  },
)

export { inputVariants }
