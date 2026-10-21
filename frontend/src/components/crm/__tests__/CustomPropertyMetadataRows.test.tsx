// @vitest-environment jsdom

import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';

import { customPropertyLabel, customPropertyRows } from '@/lib/crmCustomProperties';
import { CustomPropertyMetadataRows } from '../CustomPropertyMetadataRows';

describe('customPropertyRows', () => {
  it('formats values, sorts keys, and skips reserved, empty, and nested values', () => {
    const rows = customPropertyRows({
      skip_reason: '',
      fit: 'yes',
      support_tools: ['zendesk', 'intercom'],
      employee_estimate: 1200,
      is_b2b: true,
      agent_enrichment: { domain: {} },
      crawl: { pages: 3 },
      help_center_url: 'https://help.example.com',
      old_value: null,
    });

    expect(rows.map((row) => [row.key, row.value])).toEqual([
      ['employee_estimate', (1200).toLocaleString()],
      ['fit', 'yes'],
      ['help_center_url', 'https://help.example.com'],
      ['is_b2b', 'Yes'],
      ['support_tools', 'zendesk, intercom'],
    ]);
    expect(rows.find((row) => row.key === 'help_center_url')?.href).toBe('https://help.example.com/');
    expect(rows.find((row) => row.key === 'fit')?.href).toBeUndefined();
  });

  it('does not link non-http URLs', () => {
    expect(customPropertyRows({ link: 'javascript:alert(1)' })[0].href).toBeUndefined();
  });

  it('labels snake_case keys in sentence case', () => {
    expect(customPropertyLabel('first_line_confidence')).toBe('First line confidence');
  });
});

describe('CustomPropertyMetadataRows', () => {
  let container: HTMLDivElement;
  let root: Root;

  beforeEach(() => {
    (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
    container = document.createElement('div');
    document.body.appendChild(container);
    root = createRoot(container);
  });

  afterEach(() => {
    act(() => root.unmount());
    container.remove();
  });

  it('renders one grid row per property with a safe external link', () => {
    act(() => root.render(<div className="grid"><CustomPropertyMetadataRows properties={{ fit: 'yes', help_center_url: 'https://help.example.com' }} /></div>));

    expect(container.querySelectorAll('[data-custom-property]')).toHaveLength(2);
    expect(container.textContent).toContain('Fit');
    expect(container.textContent).toContain('Help center url');
    const link = container.querySelector('a');
    expect(link?.getAttribute('href')).toBe('https://help.example.com/');
    expect(link?.getAttribute('rel')).toBe('noopener noreferrer');
  });

  it('renders nothing without properties', () => {
    act(() => root.render(<div className="grid"><CustomPropertyMetadataRows properties={{}} /></div>));
    expect(container.querySelector('.grid')?.childElementCount).toBe(0);
  });
});
