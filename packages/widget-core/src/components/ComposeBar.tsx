import { FunctionComponent } from 'preact';
import { useState, useRef, useEffect } from 'preact/hooks';
import { PaperclipIcon, SmileIcon, SendIcon } from './icons';

interface ComposeBarProps {
  onSend: (content: string) => void;
  onTyping?: (content: string) => void;
  disabled?: boolean;
  placeholder?: string;
  showBranding?: boolean;
}

export const ComposeBar: FunctionComponent<ComposeBarProps> = ({
  onSend,
  onTyping,
  disabled = false,
  placeholder = 'Ask a question...',
  showBranding = true,
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

  const canSend = message.trim().length > 0 && !disabled;

  return (
    <div className="helpin-compose-wrapper">
      <form className="helpin-compose-bar" onSubmit={handleSubmit}>
        <textarea
          ref={textareaRef}
          className="helpin-compose-input"
          value={message}
          onInput={(e) => { const val = (e.target as HTMLTextAreaElement).value; setMessage(val); onTyping?.(val); }}
          onKeyDown={handleKeyDown}
          placeholder={placeholder}
          disabled={disabled}
          rows={1}
          aria-label={placeholder}
        />
        <div className="helpin-compose-actions">
          <div className="helpin-compose-tools">
            <button
              type="button"
              className="helpin-compose-tool-btn"
              aria-label="Attach file"
              tabIndex={0}
            >
              <PaperclipIcon size={20} />
            </button>
            <button
              type="button"
              className="helpin-compose-tool-btn"
              aria-label="Add emoji"
              tabIndex={0}
            >
              <SmileIcon size={20} />
            </button>
          </div>
          <button
            type="submit"
            className={`helpin-compose-send ${canSend ? 'helpin-compose-send--active' : ''}`}
            disabled={!canSend}
            aria-label="Send message"
          >
            <SendIcon size={16} />
          </button>
        </div>
      </form>
      {showBranding && (
        <div className="helpin-compose-footer">
          We run on{' '}
          <a href="https://helpin.ai" target="_blank" rel="noopener noreferrer" className="helpin-compose-footer-link">Helpin</a>
        </div>
      )}
    </div>
  );
};
