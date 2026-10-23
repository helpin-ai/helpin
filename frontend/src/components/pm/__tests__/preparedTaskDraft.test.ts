import { expect, it } from 'vitest';
import { mergePreparedTaskDraft } from '../preparedTaskDraft';

it('fills untouched fields while preserving user edits, including cleared fields', () => {
  const form = { name: '', description: 'My wording', task_type: 'chore', priority: 'low', team_id: 'marketing' };
  const result = mergePreparedTaskDraft(form, { name: 'AI title', description: 'AI body', task_type: 'feature', priority: 'medium' }, { name: true, description: true, priority: true });
  expect(result).toEqual({ ...form, task_type: 'feature' });
});
it('applies a complete draft without changing the chosen team', () => {
  expect(mergePreparedTaskDraft({ name: '', description: '', task_type: 'chore', priority: 'none', team_id: 'marketing' }, { name: 'Campaign brief', description: 'Publish a short guide.', task_type: 'chore', priority: 'medium' }, {})).toEqual({ name: 'Campaign brief', description: 'Publish a short guide.', task_type: 'chore', priority: 'medium', team_id: 'marketing' });
});
