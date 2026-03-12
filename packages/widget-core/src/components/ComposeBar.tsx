import { FunctionComponent } from 'preact';
import { useState, useRef, useEffect } from 'preact/hooks';

interface ComposeBarProps {
  onSend: (content: string) => void;
  disabled?: boolean;
  placeholder?: string;
}

const ICON_ATTACH =
  'M16.5 6v11.5a4 4 0 0 1-8 0V5a2.5 2.5 0 0 1 5 0v10.5a1 1 0 0 1-2 0V6h-1.5v9.5a2.5 2.5 0 0 0 5 0V5a4 4 0 0 0-8 0v12.5a5.5 5.5 0 0 0 11 0V6z';

const ICON_EMOJI =
  'M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm0 18c-4.41 0-8-3.59-8-8s3.59-8 8-8 8 3.59 8 8-3.59 8-8 8zm-4-8c.79 0 1.5-.71 1.5-1.5S8.79 9 8 9s-1.5.71-1.5 1.5S7.21 12 8 12zm8 0c.79 0 1.5-.71 1.5-1.5S16.79 9 16 9s-1.5.71-1.5 1.5.71 1.5 1.5 1.5zm-4 5.5c2.33 0 4.31-1.46 5.11-3.5H6.89c.8 2.04 2.78 3.5 5.11 3.5z';

const ICON_SEND_ARROW =
  'M12 4l-1.41 1.41L16.17 11H4v2h12.17l-5.58 5.59L12 20l8-8z';

export const ComposeBar: FunctionComponent<ComposeBarProps> = ({
  onSend,
  disabled = false,
  placeholder = 'Ask a question...',
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
          onInput={(e) => setMessage((e.target as HTMLTextAreaElement).value)}
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
              <svg viewBox="0 0 24 24" width="20" height="20" fill="currentColor">
                <path d={ICON_ATTACH} />
              </svg>
            </button>
            <button
              type="button"
              className="helpin-compose-tool-btn"
              aria-label="Add emoji"
              tabIndex={0}
            >
              <svg viewBox="0 0 24 24" width="20" height="20" fill="currentColor">
                <path d={ICON_EMOJI} />
              </svg>
            </button>
          </div>
          <button
            type="submit"
            className={`helpin-compose-send ${canSend ? 'helpin-compose-send--active' : ''}`}
            disabled={!canSend}
            aria-label="Send message"
          >
            <svg viewBox="0 0 24 24" width="16" height="16" fill="currentColor">
              <path d={ICON_SEND_ARROW} />
            </svg>
          </button>
        </div>
      </form>
      <div className="helpin-compose-footer">
        By chatting with us, you agree to our{' '}
        <a href="#" className="helpin-compose-footer-link">Privacy Policy</a>
      </div>
    </div>
  );
};
