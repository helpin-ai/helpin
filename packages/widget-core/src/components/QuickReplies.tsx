import { h, FunctionComponent } from 'preact';

interface QuickRepliesProps {
  replies: string[];
  onSelect: (reply: string) => void;
}

export const QuickReplies: FunctionComponent<QuickRepliesProps> = ({
  replies,
  onSelect,
}) => {
  return (
    <div className="helpin-quick-replies">
      {replies.map((reply, idx) => (
        <button
          key={idx}
          className="helpin-quick-reply"
          onClick={() => onSelect(reply)}
        >
          {reply}
        </button>
      ))}
    </div>
  );
};
