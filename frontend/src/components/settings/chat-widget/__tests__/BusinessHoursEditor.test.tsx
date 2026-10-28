// @vitest-environment jsdom
import React, { act, useState } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { TooltipProvider } from '@/components/ui/tooltip';
import { BusinessHoursEditor } from '../BusinessHoursEditor';
import { normalizeBusinessHoursSchedule } from '../utils';
import type { BusinessHoursDay } from '@/lib/pmTypes';

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

describe('business hours editor', () => {
  let root: Root;
  let container: HTMLDivElement;
  let latest: Record<string, BusinessHoursDay> = {};

  function Harness() {
    const [schedule, setSchedule] = useState(() => normalizeBusinessHoursSchedule({
      mon: { start: '09:00', end: '17:00', enabled: true },
    }));
    latest = schedule;
    return <BusinessHoursEditor schedule={normalizeBusinessHoursSchedule(schedule)} onChange={setSchedule} />;
  }

  beforeEach(() => {
    globalThis.ResizeObserver ??= class { observe() {} unobserve() {} disconnect() {} } as unknown as typeof ResizeObserver;
    Element.prototype.scrollIntoView ??= function scrollIntoView() {};
    container = document.createElement('div');
    document.body.appendChild(container);
    root = createRoot(container);
    act(() => root.render(<TooltipProvider><Harness /></TooltipProvider>));
  });
  afterEach(() => {
    act(() => root.unmount());
    container.remove();
    document.body.innerHTML = '';
  });

  const button = (label: string) => {
    const element = document.querySelector<HTMLButtonElement>(`button[aria-label="${label}"]`);
    if (!element) throw new Error(`missing button ${label}`);
    return element;
  };
  const click = (element: HTMLElement) => act(() => {
    element.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true }));
    element.click();
  });

  it('shows closed days as unavailable', () => {
    expect(container.textContent).toContain('Unavailable');
    expect(button('Monday start time: 9:00am')).toBeTruthy();
  });

  it('adds and removes time ranges', () => {
    click(button('Add a time range on Monday'));
    expect(latest.mon.slots).toEqual([{ start: '09:00', end: '17:00' }, { start: '17:00', end: '18:00' }]);

    click(button('Remove Monday 9:00am to 5:00pm'));
    expect(latest.mon.slots).toEqual([{ start: '17:00', end: '18:00' }]);
    expect(latest.mon.start).toBe('17:00');

    click(button('Remove Monday 5:00pm to 6:00pm'));
    expect(latest.mon.enabled).toBe(false);
  });

  it('opens a closed day from its add button', () => {
    click(button('Add a time range on Sunday'));
    expect(latest.sun.enabled).toBe(true);
    expect(button('Sunday start time: 9:00am')).toBeTruthy();
  });

  it('picks times from the list, offering only end times after the start', () => {
    const optionNamed = (name: string) =>
      Array.from(document.querySelectorAll<HTMLElement>('[cmdk-item]')).find((item) => item.textContent === name);

    click(button('Monday start time: 9:00am'));
    click(optionNamed('10:30am')!);
    expect(latest.mon.slots?.[0]).toEqual({ start: '10:30', end: '17:00' });

    click(button('Monday end time: 5:00pm'));
    expect(optionNamed('10:30am')).toBeUndefined();
    expect(optionNamed('10:45am')).toBeTruthy();
    expect(optionNamed('12:00am')).toBeTruthy();
  });

  it('moves the end along when the start passes it', () => {
    click(button('Monday start time: 9:00am'));
    click(Array.from(document.querySelectorAll<HTMLElement>('[cmdk-item]')).find((item) => item.textContent === '6:00pm')!);
    expect(latest.mon.slots?.[0]).toEqual({ start: '18:00', end: '19:00' });
  });

  it('copies a day to the days picked in the copy menu', () => {
    click(button('Copy Monday times to other days'));
    const option = (name: string) => {
      const element = Array.from(document.querySelectorAll<HTMLElement>('[cmdk-item]')).find((item) => item.textContent === name);
      if (!element) throw new Error(`missing option ${name}`);
      return element;
    };
    expect(option('Monday').getAttribute('data-disabled')).toBe('true');
    click(option('Tuesday'));
    click(option('Saturday'));
    const apply = Array.from(document.querySelectorAll<HTMLButtonElement>('button')).find((element) => element.textContent === 'Apply');
    click(apply!);

    expect(latest.tue).toEqual(latest.mon);
    expect(latest.sat.enabled).toBe(true);
    expect(latest.wed.enabled).toBe(false);
  });

  it('selects every other day with select all', () => {
    click(button('Copy Monday times to other days'));
    const selectAll = Array.from(document.querySelectorAll<HTMLElement>('[cmdk-item]')).find((item) => item.textContent === 'Select all');
    click(selectAll!);
    const apply = Array.from(document.querySelectorAll<HTMLButtonElement>('button')).find((element) => element.textContent === 'Apply');
    click(apply!);

    for (const key of ['tue', 'wed', 'thu', 'fri', 'sat', 'sun']) {
      expect(latest[key].enabled).toBe(true);
      expect(latest[key].slots).toEqual([{ start: '09:00', end: '17:00' }]);
    }
  });
});
