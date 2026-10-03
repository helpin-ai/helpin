'use client';

import { WorkScene } from '../../_components/WorkScene';

// Both placements show the same coverage-gap → sourced article proposal flow.
export function SupportKnowledge({ variant = 'support' }: { variant?: 'support' | 'knowledge' }) {
  return <div data-knowledge-context={variant}><WorkScene variant="coverage" /></div>;
}
