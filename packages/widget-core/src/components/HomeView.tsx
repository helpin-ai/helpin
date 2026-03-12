import { FunctionComponent } from 'preact';
import { useState } from 'preact/hooks';
import type { WidgetConfig } from '../types';

interface HomeViewProps {
  config: WidgetConfig;
  onSendMessage: (content: string) => void;
  onNavigate: (view: 'messages' | 'help') => void;
  showPreChatForm: boolean;
  onPreChatSubmit: (data: { name: string; email: string }) => void;
}

const SEND_ICON = 'M3.4 20.4l17.45-7.48a1 1 0 000-1.84L3.4 3.6a.993.993 0 00-1.39.91L2 9.12c0 .5.37.93.87.99L17 12 2.87 13.88c-.5.07-.87.5-.87 1l.01 4.61c0 .71.73 1.2 1.39.91z';

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
                  <svg viewBox="0 0 24 24" width="18" height="18" fill="white">
                    <path d={SEND_ICON} />
                  </svg>
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
                  <svg viewBox="0 0 24 24" width="18" height="18" fill="white">
                    <path d={SEND_ICON} />
                  </svg>
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
              <svg viewBox="0 0 24 24" width="18" height="18" fill="white">
                <path d={SEND_ICON} />
              </svg>
            </button>
          </div>
        )}

        {/* Quick action cards */}
        <div className="helpin-home-actions">
          <button className="helpin-home-action" onClick={() => onNavigate('messages')}>
            <svg viewBox="0 0 24 24" width="18" height="18" fill="currentColor">
              <path d="M20 2H4c-1.1 0-2 .9-2 2v12c0 1.1.9 2 2 2h14l4 4V4c0-1.1-.9-2-2-2zm-2 12H6v-2h12v2zm0-3H6V9h12v2zm0-3H6V6h12v2z" />
            </svg>
            <div className="helpin-home-action-text">
              <span className="helpin-home-action-title">Send us a message</span>
              <span className="helpin-home-action-desc">We typically reply in a few minutes</span>
            </div>
            <svg viewBox="0 0 24 24" width="16" height="16" fill="currentColor" className="helpin-home-action-arrow">
              <path d="M8.59 16.59L13.17 12 8.59 7.41 10 6l6 6-6 6z" />
            </svg>
          </button>

          <button className="helpin-home-action" onClick={() => onNavigate('help')}>
            <svg viewBox="0 0 24 24" width="18" height="18" fill="currentColor">
              <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm1 17h-2v-2h2v2zm2.07-7.75l-.9.92C13.45 12.9 13 13.5 13 15h-2v-.5c0-1.1.45-2.1 1.17-2.83l1.24-1.26c.37-.36.59-.86.59-1.41 0-1.1-.9-2-2-2s-2 .9-2 2H8c0-2.21 1.79-4 4-4s4 1.79 4 4c0 .88-.36 1.68-.93 2.25z" />
            </svg>
            <div className="helpin-home-action-text">
              <span className="helpin-home-action-title">Help center</span>
              <span className="helpin-home-action-desc">Find answers to common questions</span>
            </div>
            <svg viewBox="0 0 24 24" width="16" height="16" fill="currentColor" className="helpin-home-action-arrow">
              <path d="M8.59 16.59L13.17 12 8.59 7.41 10 6l6 6-6 6z" />
            </svg>
          </button>
        </div>
      </div>
    </div>
  );
};
