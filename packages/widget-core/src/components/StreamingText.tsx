import { h, FunctionComponent } from 'preact';
import { useState, useEffect } from 'preact/hooks';

interface StreamingTextProps {
  text: string;
  isStreaming: boolean;
  onComplete?: () => void;
}

export const StreamingText: FunctionComponent<StreamingTextProps> = ({
  text,
  isStreaming,
  onComplete,
}) => {
  const [displayedText, setDisplayedText] = useState('');

  useEffect(() => {
    if (isStreaming && displayedText.length < text.length) {
      const timeout = setTimeout(() => {
        setDisplayedText(text.slice(0, displayedText.length + 1));
      }, 30);
      return () => clearTimeout(timeout);
    } else if (!isStreaming && text !== displayedText) {
      setDisplayedText(text);
      onComplete?.();
    }
  }, [text, isStreaming, displayedText, onComplete]);

  return (
    <div className="helpin-streaming-text">
      <span>{displayedText}</span>
      {isStreaming && <span className="helpin-cursor">▊</span>}
    </div>
  );
};
