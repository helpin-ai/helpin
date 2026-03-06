import type { EstimateScale } from './types';

export interface EstimateOption {
  value: number;
  label: string;
}

const SCALES: Record<EstimateScale, { base: EstimateOption[]; extended: EstimateOption[] }> = {
  exponential: {
    base: [
      { value: 1, label: '1' },
      { value: 2, label: '2' },
      { value: 4, label: '4' },
      { value: 8, label: '8' },
      { value: 16, label: '16' },
    ],
    extended: [
      { value: 32, label: '32' },
      { value: 64, label: '64' },
    ],
  },
  fibonacci: {
    base: [
      { value: 1, label: '1' },
      { value: 2, label: '2' },
      { value: 3, label: '3' },
      { value: 5, label: '5' },
      { value: 8, label: '8' },
    ],
    extended: [
      { value: 13, label: '13' },
      { value: 21, label: '21' },
    ],
  },
  linear: {
    base: [
      { value: 1, label: '1' },
      { value: 2, label: '2' },
      { value: 3, label: '3' },
      { value: 4, label: '4' },
      { value: 5, label: '5' },
    ],
    extended: [
      { value: 6, label: '6' },
      { value: 7, label: '7' },
    ],
  },
  tshirt: {
    base: [
      { value: 1, label: 'XS' },
      { value: 2, label: 'S' },
      { value: 3, label: 'M' },
      { value: 5, label: 'L' },
      { value: 8, label: 'XL' },
    ],
    extended: [
      { value: 13, label: 'XXL' },
      { value: 21, label: 'XXXL' },
    ],
  },
  hours: {
    base: [
      { value: 1, label: '1h' },
      { value: 2, label: '2h' },
      { value: 4, label: '4h' },
      { value: 8, label: '8h' },
      { value: 16, label: '16h' },
    ],
    extended: [
      { value: 24, label: '24h' },
      { value: 40, label: '40h' },
    ],
  },
};

export function getEstimateOptions(scale: EstimateScale, extended: boolean, allowZero: boolean): EstimateOption[] {
  const def = SCALES[scale];
  const options: EstimateOption[] = [];
  if (allowZero) {
    options.push({ value: 0, label: scale === 'tshirt' ? '0' : '0' });
  }
  options.push(...def.base);
  if (extended) {
    options.push(...def.extended);
  }
  return options;
}

export function formatEstimateValue(value: number | undefined | null, scale: EstimateScale): string {
  if (value == null) return 'None';

  if (scale === 'tshirt') {
    const all = [...SCALES.tshirt.base, ...SCALES.tshirt.extended];
    const match = all.find((o) => o.value === value);
    return match ? match.label : `${value}`;
  }

  if (scale === 'hours') {
    return `${value}h`;
  }

  return `${value} pts`;
}

export const SCALE_LABELS: Record<EstimateScale, string> = {
  exponential: 'Exponential',
  fibonacci: 'Fibonacci',
  linear: 'Linear',
  tshirt: 'T-Shirt',
  hours: 'Hours',
};

export const SCALE_DESCRIPTIONS: Record<EstimateScale, string> = {
  exponential: '1, 2, 4, 8, 16',
  fibonacci: '1, 2, 3, 5, 8',
  linear: '1, 2, 3, 4, 5',
  tshirt: 'XS, S, M, L, XL',
  hours: '1h, 2h, 4h, 8h, 16h',
};
