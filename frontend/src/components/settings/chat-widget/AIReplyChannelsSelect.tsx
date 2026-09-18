import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/design-system/quiet-dropdown-select';

import { getAIReplyChannels, type AIReplyChannels } from './responseModes';
export { getAIReplyChannels, type AIReplyChannels } from './responseModes';

export function AIReplyChannelsSelect({ value, onChange }: { value: AIReplyChannels; onChange: (value: AIReplyChannels) => void }) {
  return (
    <Select value={value} onValueChange={(next) => onChange(getAIReplyChannels(next))}>
      <SelectTrigger id="ai-reply-channels" aria-label="Reply channels" variant="underline" className="w-full">
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        <SelectItem value="chat">Chat</SelectItem>
        <SelectItem value="email">Email</SelectItem>
        <SelectItem value="both">Chat and email</SelectItem>
      </SelectContent>
    </Select>
  );
}
