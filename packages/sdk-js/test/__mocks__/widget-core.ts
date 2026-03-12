// Mock for @helpin/widget-core in test environment
import { vi } from 'vitest';

export const mountWidget = vi.fn((_container: HTMLElement, _options: any): void => {});
export const unmountWidget = vi.fn((_container: HTMLElement): void => {});
