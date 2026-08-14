import { renderToStaticMarkup } from 'react-dom/server';
import { describe, expect, it } from 'vitest';

import { UpdateActivityRow } from '@/components/pm/UpdateActivityRow';

describe('UpdateActivityRow', () => {
  it('keeps the sentence quiet and emphasizes only meaningful values', () => {
    const markup = renderToStaticMarkup(
      <UpdateActivityRow
        label="Amad moved this epic from To Do to In Progress"
        occurredAt="2026-08-14T12:00:00Z"
        emphasizedValues={['Amad', 'To Do', 'In Progress']}
        humanActor={{
          email: 'amad@example.com',
          full_name: 'Amad',
          avatar_style: 'personas',
          avatar_seed: 'amad-seed',
          avatar_background_mode: 'color',
          avatar_background_color: '#f59e0b',
        }}
      />,
    );

    expect(markup).toContain('min-w-0 flex-1 truncate text-foreground/70');
    expect(markup.match(/font-semibold text-foreground\/90/g)).toHaveLength(3);
    expect(markup).toContain('>AM</span>');
    expect(markup).not.toContain('block truncate font-medium');
  });

  it('uses an agent avatar and keeps details inline', () => {
    const markup = renderToStaticMarkup(
      <UpdateActivityRow
        label="Forge completed an automated run"
        occurredAt="2026-08-14T12:00:00Z"
        emphasizedValues={['Forge']}
        detail="Heartbeat timeout"
        agent={{ name: 'Forge', presetKey: 'code_builder' }}
      />,
    );

    const text = markup.replace(/<[^>]+>/g, '');
    expect(text).toContain('Forge completed an automated run');
    expect(text).toContain(' — Heartbeat timeout');
    expect(markup).toContain('h-5 w-5 rounded-none border-0 bg-transparent shadow-none');
  });
});
