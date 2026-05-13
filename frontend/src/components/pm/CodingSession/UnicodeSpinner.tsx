import { useEffect, useState, type HTMLAttributes } from 'react';
import spinners, { type BrailleSpinnerName } from 'unicode-animations';
import { cn } from '@/lib/utils';

export type SpinnerName = BrailleSpinnerName;

export function UnicodeSpinner({
  name = 'braille',
  className,
  children,
  ...props
}: {
  name?: SpinnerName;
  className?: string;
  children?: React.ReactNode;
} & HTMLAttributes<HTMLSpanElement>) {
  const [frame, setFrame] = useState(0);
  const spinner = spinners[name];

  useEffect(() => {
    const timer = setInterval(
      () => setFrame((f) => (f + 1) % spinner.frames.length),
      spinner.interval,
    );
    return () => clearInterval(timer);
  }, [name, spinner.frames.length, spinner.interval]);

  return (
    <span {...props} className={cn('inline-flex items-center gap-1 font-mono', className)}>
      <span className="inline-block text-center">{spinner.frames[frame]}</span>
      {children}
    </span>
  );
}
