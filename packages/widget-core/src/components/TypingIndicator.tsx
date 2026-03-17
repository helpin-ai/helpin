import { FunctionComponent } from 'preact';

interface TypingIndicatorProps {
  label?: string;
  agentName?: string;
  agentAvatar?: string;
}

export const TypingIndicator: FunctionComponent<TypingIndicatorProps> = ({
  label = 'is typing...',
  agentName,
  agentAvatar,
}) => {
  return (
    <div className="helpin-typing-indicator">
      {agentAvatar ? (
        <img src={agentAvatar} alt={agentName || ''} className="helpin-typing-avatar" />
      ) : agentName ? (
        <span className="helpin-typing-avatar-placeholder">
          {agentName.charAt(0).toUpperCase()}
        </span>
      ) : null}
      <span className="helpin-typing-dots">
        <span></span>
        <span></span>
        <span></span>
      </span>
      <span className="helpin-typing-label">
        {agentName ? `${agentName} ${label}` : label}
      </span>
    </div>
  );
};
