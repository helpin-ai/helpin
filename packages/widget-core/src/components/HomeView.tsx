import { FunctionComponent } from 'preact';
import { useState } from 'preact/hooks';
import type { WidgetConfig } from '../types';
import { MessageSquareIcon, CircleHelpIcon, ChevronRightIcon, ArrowRightIcon } from './icons';

interface HomeViewProps {
  config: WidgetConfig;
  onSendMessage: (content: string) => void;
  onNavigate: (view: 'conversation' | 'messages' | 'help') => void;
}

export const HomeView: FunctionComponent<HomeViewProps> = ({
  config,
  onSendMessage,
  onNavigate,
}) => {
  const [query, setQuery] = useState('');
  const brandColor = config.branding?.primaryColor || '#6366f1';
  const logoUrl = config.branding?.logoUrl;
  const welcomeMessage = config.branding?.welcomeMessage || 'How can we help?';
  const workspaceName = config.workspaceName || 'Support';
  const availability = config.availability;
  const statusText = availability?.statusText || 'Online now';
  const replyTimeText = availability?.replyTimeText || 'We typically reply in a few minutes';

  const handleSend = () => {
    const trimmed = query.trim();
    if (!trimmed) return;
    onSendMessage(trimmed);
    setQuery('');
  };

  const handleKeyDown = (e: KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  };

  return (
    <div className="helpin-home-view">
      {/* Gradient header area */}
      <div className="helpin-home-header" style={{ background: `linear-gradient(135deg, ${brandColor}, ${brandColor}88)` }}>
        {logoUrl ? (
          <img src={logoUrl} alt={workspaceName} className="helpin-home-logo" />
        ) : (
          <div className="helpin-home-logo-placeholder" style={{ backgroundColor: '#ffffff' }}>
            <span>{workspaceName.charAt(0).toUpperCase()}</span>
          </div>
        )}
      </div>

      {/* Welcome content */}
      <div className="helpin-home-content">
        <h2 className="helpin-home-welcome">{welcomeMessage}</h2>
        <p className="helpin-home-status">{statusText}</p>

        <div className="helpin-home-search">
          <input
            type="text"
            className="helpin-home-input"
            placeholder="Ask me anything..."
            value={query}
            onInput={(e) => setQuery((e.target as HTMLInputElement).value)}
            onKeyDown={handleKeyDown}
          />
          <button
            className="helpin-home-send"
            onClick={handleSend}
            style={{ backgroundColor: brandColor }}
            disabled={!query.trim()}
          >
            <ArrowRightIcon size={18} color="white" />
          </button>
        </div>

        {/* Quick action cards */}
        <div className="helpin-home-actions">
          <button className="helpin-home-action" onClick={() => onNavigate('conversation')}>
            <MessageSquareIcon size={18} />
            <div className="helpin-home-action-text">
              <span className="helpin-home-action-title">Send us a message</span>
              <span className="helpin-home-action-desc">{replyTimeText}</span>
            </div>
            <ChevronRightIcon size={16} class="helpin-home-action-arrow" />
          </button>

          <button className="helpin-home-action" onClick={() => onNavigate('help')}>
            <CircleHelpIcon size={18} />
            <div className="helpin-home-action-text">
              <span className="helpin-home-action-title">Help center</span>
              <span className="helpin-home-action-desc">Find answers to common questions</span>
            </div>
            <ChevronRightIcon size={16} class="helpin-home-action-arrow" />
          </button>
        </div>
      </div>
    </div>
  );
};
