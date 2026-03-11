import { FunctionComponent } from 'preact';
import { useState, useRef, useEffect } from 'preact/hooks';

interface ComposeBarProps {
  onSend: (content: string) => void;
  disabled?: boolean;
  placeholder?: string;
}

export const ComposeBar: FunctionComponent<ComposeBarProps> = ({
  onSend,
  disabled = false,
  placeholder = 'Type a message...',
}) => {
  const [message, setMessage] = useState('');
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  useEffect(() => {
    if (textareaRef.current) {
      textareaRef.current.style.height = 'auto';
      textareaRef.current.style.height = `${Math.min(textareaRef.current.scrollHeight, 120)}px`;
    }
  }, [message]);

  const handleSubmit = (e?: Event) => {
    e?.preventDefault();
    if (message.trim() && !disabled) {
      onSend(message.trim());
      setMessage('');
    }
  };

  const handleKeyDown = (e: KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSubmit();
    }
  };

  return (
    <form className="helpin-compose-bar" onSubmit={handleSubmit}>
      <textarea
        ref={textareaRef}
        className="helpin-compose-input"
        value={message}
        onInput={(e) => setMessage((e.target as HTMLTextAreaElement).value)}
        onKeyDown={handleKeyDown}
        placeholder={placeholder}
        disabled={disabled}
        rows={1}
        aria-label={placeholder}
      />
      <button
        type="submit"
        className="helpin-compose-send"
        disabled={disabled || !message.trim()}
        aria-label="Send message"
      >
        <svg viewBox="0 0 24 24" width="20" height="20" fill="currentColor">
          <path d="M2.01 21L23 12 2.01 3 2 10l15 2-15 2z" />
        </svg>
      </button>
    </form>
  );
};
