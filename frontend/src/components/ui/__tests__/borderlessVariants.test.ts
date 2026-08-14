import { describe, expect, it } from 'vitest';

import { pickerTriggerVariants } from '@/components/ui/picker-trigger';
import { inputVariants } from '@/components/ui/input-variants';
import { getOptionalSectionActionClass } from '@/components/pm/optionalSectionActionPill';

describe('borderless control variants', () => {
  it('keeps the existing filled picker treatment as the default', () => {
    expect(pickerTriggerVariants()).toContain('bg-input/50');
    expect(pickerTriggerVariants()).toContain('rounded-3xl');
  });

  it('provides ghost and underline picker treatments without boxed chrome', () => {
    const ghost = pickerTriggerVariants({ variant: 'ghost' });
    const underline = pickerTriggerVariants({ variant: 'underline' });

    expect(ghost).toContain('bg-transparent');
    expect(underline).toContain('border-b');
    expect(underline).toContain('focus-visible:border-foreground');
    expect(underline).not.toContain('rounded-3xl');
  });

  it('provides a plain input that delegates focus emphasis to its divider row', () => {
    const plain = inputVariants({ variant: 'plain' });

    expect(plain).toContain('bg-transparent');
    expect(plain).toContain('border-0');
    expect(plain).toContain('focus-visible:ring-0');
  });

  it('renders task attachment actions as 13px borderless controls', () => {
    const borderless = getOptionalSectionActionClass('available', 'borderless');

    expect(borderless).toContain('text-[13px]');
    expect(borderless).toContain('border-0');
    expect(borderless).toContain('p-0');
    expect(borderless).not.toContain('rounded-full');
  });
});
