import blackLogoUrl from './helpin-icon-black.svg'
import whiteLogoUrl from './helpin-icon-white.svg'

interface HelpinLogoProps {
  className?: string
}

export function HelpinLogo({ className = '' }: HelpinLogoProps) {
  return (
    <div
      aria-label="Helpin"
      className={`flex items-center justify-center gap-1 text-foreground ${className}`}
    >
      <img
        src={blackLogoUrl}
        alt=""
        aria-hidden="true"
        className="h-11 w-11 shrink-0 scale-125 dark:hidden"
      />
      <img
        src={whiteLogoUrl}
        alt=""
        aria-hidden="true"
        className="hidden h-11 w-11 shrink-0 scale-125 dark:block"
      />
      <span className="text-[30px] font-semibold tracking-[-0.04em]">Helpin</span>
    </div>
  )
}
