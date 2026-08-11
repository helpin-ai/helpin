import type { TaskUpdateEntry } from '@/lib/pmTypes';

function sentenceCase(value: string) {
  return value ? `${value[0].toUpperCase()}${value.slice(1)}` : value;
}

function readableActorName(entry: TaskUpdateEntry) {
  return entry.actor?.full_name?.trim() || entry.actor?.email?.trim() || '';
}

function readableAgentName(entry: TaskUpdateEntry) {
  const metadata = entry.activity?.metadata;
  const metadataName = typeof metadata?.agent_name === 'string' ? metadata.agent_name.trim() : '';
  return metadataName || entry.agent_name?.trim() || 'agent';
}

export function taskUpdateEventLabel(entry: TaskUpdateEntry) {
  if (entry.kind === 'agent_run' && entry.agent_run) {
    const name = entry.agent_name || 'Agent';
    return `${name} ${entry.agent_run.status.replace(/_/g, ' ')}`;
  }
  if (entry.kind === 'git' && entry.git_link) {
    if (entry.git_link.pr_number) return `Pull request #${entry.git_link.pr_number} · ${entry.git_link.pr_status ?? 'linked'}`;
    if (entry.git_link.commit_sha) return `Commit ${entry.git_link.commit_sha.slice(0, 7)} linked`;
    return `Branch ${entry.git_link.branch ?? ''} linked`;
  }

  const activity = entry.activity;
  if (!activity) return 'Task updated';

  if (activity.field_name === 'agent_run' && activity.new_value === 'note_added') {
    const actorName = readableActorName(entry);
    const agentName = readableAgentName(entry);
    const noteSnippet = typeof activity.metadata?.note_snippet === 'string'
      ? activity.metadata.note_snippet.trim()
      : '';
    const action = actorName
      ? `${actorName} added a note to the ${agentName} run`
      : `A note was added to the ${agentName} run`;
    return noteSnippet ? `${action} · “${noteSnippet}”` : action;
  }

  if (activity.field_name && activity.new_value) {
    return sentenceCase(`${activity.field_name.replace(/_/g, ' ')} changed${activity.old_value ? ` · ${activity.old_value} → ${activity.new_value}` : ` to ${activity.new_value}`}`);
  }
  return sentenceCase(activity.action.replace(/_/g, ' '));
}
