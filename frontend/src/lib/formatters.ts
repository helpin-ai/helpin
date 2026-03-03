// Currency and number formatting utilities

export function formatCurrencyUSD(amount: number): string {
  try {
    return new Intl.NumberFormat('en-US', {
      style: 'currency',
      currency: 'USD',
      minimumFractionDigits: 0,
      maximumFractionDigits: 0,
    }).format(amount);
  } catch (error) {
    // Fallback if Intl.NumberFormat fails
    return `$${Math.round(amount).toLocaleString()}`;
  }
}

export function formatPercentage(value: number): string {
  return `${Math.round(value)}%`;
}

export function formatCount(value: number): string {
  return Math.round(value).toLocaleString();
}

export function formatGoalValue(value: number, unit: 'USD' | '%' | 'count'): string {
  switch (unit) {
    case 'USD':
      return formatCurrencyUSD(value);
    case '%':
      return formatPercentage(value);
    case 'count':
      return formatCount(value);
    default:
      return value.toString();
  }
}