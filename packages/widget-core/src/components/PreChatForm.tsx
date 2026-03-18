import { FunctionComponent } from 'preact';
import { useState, useRef, useEffect } from 'preact/hooks';
import type { WidgetConfig } from '../types';
import { ChevronRightIcon } from './icons';

interface PreChatFormProps {
  config: WidgetConfig;
  onSubmit: (data: { name: string; email: string }) => void;
}

export const PreChatForm: FunctionComponent<PreChatFormProps> = ({
  config,
  onSubmit,
}) => {
  const [email, setEmail] = useState('');
  const [name, setName] = useState('');
  const [step, setStep] = useState<'email' | 'name' | 'done'>('email');
  const formRef = useRef<HTMLDivElement>(null);
  const brandColor = config.branding?.primaryColor || '#6366f1';
  const workspaceName = config.workspaceName || 'Support';
  const logoUrl = config.branding?.logoUrl;

  useEffect(() => {
    if (typeof formRef.current?.scrollIntoView === 'function') {
      formRef.current.scrollIntoView({ behavior: 'smooth', block: 'end' });
    }
  }, [step]);

  const requireName = config.features?.requireName !== false;

  const handleEmailSubmit = (e: Event) => {
    e.preventDefault();
    if (!email.trim()) return;
    if (requireName) {
      setStep('name');
    } else {
      onSubmit({ name: '', email: email.trim() });
      setStep('done');
    }
  };

  const handleNameSubmit = (e: Event) => {
    e.preventDefault();
    onSubmit({ name: name.trim(), email: email.trim() });
    setStep('done');
  };

  if (step === 'done') return null;

  return (
    <div className="helpin-inline-prechat" ref={formRef}>
      <div className="helpin-message-row helpin-message-row--agent">
        <div className="helpin-message-agent-header">
          {logoUrl ? (
            <img src={logoUrl} alt={workspaceName} className="helpin-message-avatar" />
          ) : (
            <span className="helpin-message-avatar-placeholder">
              {workspaceName.charAt(0).toUpperCase()}
            </span>
          )}
          <span className="helpin-message-agent-name">{workspaceName}</span>
        </div>
        <div className="helpin-message-agent-bubble-wrap">
          <div className="helpin-message-bubble helpin-message--agent">
            {step === 'email' && (
              <>
                <div className="helpin-message-content">
                  Please enter your email address so we can get back to you by email if needed.
                </div>
                <form onSubmit={handleEmailSubmit} className="helpin-inline-prechat-form">
                  <label className="helpin-sr-only" htmlFor="helpin-prechat-email">Email address</label>
                  <input
                    id="helpin-prechat-email"
                    type="email"
                    className="helpin-inline-prechat-input"
                    placeholder="Your e-mail"
                    value={email}
                    onInput={(e) => setEmail((e.target as HTMLInputElement).value)}
                    required
                    autoFocus
                  />
                  <button
                    type="submit"
                    className="helpin-inline-prechat-btn"
                    style={{ backgroundColor: brandColor }}
                    disabled={!email.trim()}
                    aria-label="Submit email"
                  >
                    <ChevronRightIcon size={16} color="white" />
                  </button>
                </form>
              </>
            )}
            {step === 'name' && (
              <>
                <div className="helpin-message-content">
                  Thanks! What's your name?
                </div>
                <form onSubmit={handleNameSubmit} className="helpin-inline-prechat-form">
                  <label className="helpin-sr-only" htmlFor="helpin-prechat-name">Your name</label>
                  <input
                    id="helpin-prechat-name"
                    type="text"
                    className="helpin-inline-prechat-input"
                    placeholder="Your name"
                    value={name}
                    onInput={(e) => setName((e.target as HTMLInputElement).value)}
                    autoFocus
                  />
                  <button
                    type="submit"
                    className="helpin-inline-prechat-btn"
                    style={{ backgroundColor: brandColor }}
                    aria-label="Submit name"
                  >
                    <ChevronRightIcon size={16} color="white" />
                  </button>
                </form>
              </>
            )}
          </div>
        </div>
      </div>
    </div>
  );
};
