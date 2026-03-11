import { FunctionComponent } from 'preact';

interface TypingIndicatorProps {
  label?: string;
}

export const TypingIndicator: FunctionComponent<TypingIndicatorProps> = ({
  label = 'is typing...',
}) => {
  return (
    <div className="helpin-typing-indicator">
      <span className="helpin-typing-dots">
        <span></span>
        <span></span>
        <span></span>
      </span>
      <span className="helpin-typing-label">{label}</span>
    </div>
  );
};
