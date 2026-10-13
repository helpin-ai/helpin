// Mock for @helpin-ai/widget-core in test environment
import { vi } from 'vitest';
export { SYSTEM_EVENT_TYPES } from '@helpin-ai/shared';

export const mountWidget = vi.fn((_container: HTMLElement, _options: any): void => {});
export const unmountWidget = vi.fn((_container: HTMLElement): void => {});
