import { FunctionComponent } from 'preact';
import { useState } from 'preact/hooks';
import type { WidgetConfig } from '../types';
import { MessageSquareIcon, CircleHelpIcon, ChevronRightIcon, ArrowRightIcon } from './icons';

interface HomeViewProps {
  config: WidgetConfig;
  onSendMessage: (content: string) => void;
  onNavigate: (view: 'conversation' | 'messages' | 'help') => void;
  showPreChatForm: boolean;
  onPreChatSubmit: (data: { name: string; email: string }) => void;
}

export const HomeView: FunctionComponent<HomeViewProps> = ({
  config,
  onSendMessage,
  onNavigate,
  showPreChatForm,
  onPreChatSubmit,
}) => {
  const [query, setQuery] = useState('');
  const [email, setEmail] = useState('');
  const [name, setName] = useState('');
  const [step, setStep] = useState<'email' | 'name'>('email');
  const brandColor = config.branding?.primaryColor || '#6366f1';
  const logoUrl = config.branding?.logoUrl;
  const welcomeMessage = config.branding?.welcomeMessage || 'How can we help?';
  const workspaceName = config.workspaceName || 'Support';

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

  const handleEmailSubmit = (e: Event) => {
    e.preventDefault();
    if (!email.trim()) return;
    if (config.features?.preChatForm) {
      // If name is also required, go to name step
      setStep('name');
    } else {
      onPreChatSubmit({ name: '', email: email.trim() });
    }
  };

  const handleNameSubmit = (e: Event) => {
    e.preventDefault();
    onPreChatSubmit({ name: name.trim(), email: email.trim() });
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

        {showPreChatForm ? (
          <div className="helpin-home-prechat">
            {step === 'email' ? (
              <form onSubmit={handleEmailSubmit} className="helpin-home-form">
                <input
                  type="email"
                  className="helpin-home-input"
                  placeholder="Enter your email to get started..."
                  value={email}
                  onInput={(e) => setEmail((e.target as HTMLInputElement).value)}
                  required
                />
                <button
                  type="submit"
                  className="helpin-home-send"
                  style={{ backgroundColor: brandColor }}
                  disabled={!email.trim()}
                >
                  <ArrowRightIcon size={18} color="white" />
                </button>
              </form>
            ) : (
              <form onSubmit={handleNameSubmit} className="helpin-home-form">
                <input
                  type="text"
                  className="helpin-home-input"
                  placeholder="What's your name?"
                  value={name}
                  onInput={(e) => setName((e.target as HTMLInputElement).value)}
                />
                <button
                  type="submit"
                  className="helpin-home-send"
                  style={{ backgroundColor: brandColor }}
                >
                  <ArrowRightIcon size={18} color="white" />
                </button>
              </form>
            )}
          </div>
        ) : (
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
        )}

        {/* Quick action cards */}
        <div className="helpin-home-actions">
          <button className="helpin-home-action" onClick={() => onNavigate('conversation')}>
            <MessageSquareIcon size={18} />
            <div className="helpin-home-action-text">
              <span className="helpin-home-action-title">Send us a message</span>
              <span className="helpin-home-action-desc">We typically reply in a few minutes</span>
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
