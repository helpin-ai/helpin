export function formatAIUsagePercent(value: number): string {
  if (!Number.isFinite(value) || value <= 0) return '0%';
  if (value < 0.01) return '<0.01%';
  const rounded = Math.round(value * 10) / 10;
  return `${Number.isInteger(rounded) ? rounded.toFixed(0) : rounded.toFixed(1)}%`;
}

export function calculateAIUsagePercent(used: number, allowance: number): number {
  if (!Number.isFinite(used) || !Number.isFinite(allowance) || used <= 0 || allowance <= 0) return 0;
  return (used / allowance) * 100;
}
