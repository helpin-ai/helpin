import { useQuery } from '@tanstack/react-query';
import { VoiceFeedback, VoiceMicrophone } from '@/components/voice/VoiceInput';
import { queryKeys } from '@/lib/queryKeys';
import { appendVoiceTranscript } from '@/lib/voiceRecording';
import { voiceInputAvailable } from '@/lib/services/voiceInputService';
import { useVoiceInput } from './useVoiceInput';

export function useVoiceComposer({ workspaceId, identity, enabled, value, onChange, onReady }: {
  workspaceId?: string;
  identity: string;
  enabled: boolean;
  value: string;
  onChange: (value: string) => void;
  onReady?: () => void;
}) {
  const availability = useQuery({
    queryKey: queryKeys.voiceInput(workspaceId ?? ''),
    queryFn: () => voiceInputAvailable(workspaceId!),
    enabled: Boolean(workspaceId) && enabled,
    staleTime: 60_000,
    retry: false,
  });
  const voice = useVoiceInput({
    workspaceId, identity, enabled,
    onTranscript: text => { onChange(appendVoiceTranscript(value, text)); onReady?.(); },
  });
  return {
    busy: voice.busy,
    cancel: voice.cancel,
    microphone: availability.data ? <VoiceMicrophone enabled={enabled && !voice.busy} onStart={() => void voice.start()} /> : null,
    feedback: <VoiceFeedback voice={voice} />,
  };
}
