import { FunctionComponent } from 'preact';
import { useState, useRef, useEffect } from 'preact/hooks';
import type { WidgetConfig } from '../types';
import { ChevronRightIcon, MailIcon, PhoneIcon } from './icons';

interface PreChatFormProps {
  config: WidgetConfig;
  onSubmit: (data: { phone: string; email: string }) => void;
}

export const PreChatForm: FunctionComponent<PreChatFormProps> = ({
  config,
  onSubmit,
}) => {
  const [email, setEmail] = useState('');
  const [phone, setPhone] = useState('');
  const [step, setStep] = useState<'email' | 'phone' | 'done'>('email');
  const formRef = useRef<HTMLDivElement>(null);
  const brandColor = config.branding?.primaryColor || '#6366f1';

  useEffect(() => {
    if (typeof formRef.current?.scrollIntoView === 'function') {
      formRef.current.scrollIntoView({ behavior: 'smooth', block: 'end' });
    }
  }, [step]);

  const requirePhone = config.features?.requirePhone !== false;

  const handleEmailSubmit = (e: Event) => {
    e.preventDefault();
    if (!email.trim()) return;
    if (requirePhone) {
      setStep('phone');
    } else {
      onSubmit({ phone: '', email: email.trim() });
      setStep('done');
    }
  };

  const handlePhoneSubmit = (e: Event) => {
    e.preventDefault();
    onSubmit({ phone: phone.trim(), email: email.trim() });
    setStep('done');
  };

  if (step === 'done') return null;

  return (
    <div className="helpin-inline-prechat" ref={formRef}>
      <div className="helpin-message-row helpin-message-row--agent">
        <div className="helpin-message-agent-bubble-wrap">
          <div className="helpin-message-bubble helpin-message--agent">
            {step === 'email' && (
              <>
                <div className="helpin-message-content">
                  Please enter your email address so we can get back to you by email if needed.
                </div>
                <form onSubmit={handleEmailSubmit} className="helpin-inline-prechat-form">
                  <label className="helpin-sr-only" htmlFor="helpin-prechat-email">Email address</label>
                  <div className="helpin-inline-prechat-input-wrap">
                    <MailIcon size={14} class="helpin-inline-prechat-icon" />
                    <input
                      id="helpin-prechat-email"
                      type="email"
                      className="helpin-inline-prechat-input"
                      placeholder="you@example.com"
                      value={email}
                      onInput={(e) => setEmail((e.target as HTMLInputElement).value)}
                      required
                      autoFocus
                    />
                  </div>
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
            {step === 'phone' && (
              <>
                <div className="helpin-message-content">
                  Thanks! What's your phone number?
                </div>
                <form onSubmit={handlePhoneSubmit} className="helpin-inline-prechat-form">
                  <label className="helpin-sr-only" htmlFor="helpin-prechat-phone">Your phone number</label>
                  <div className="helpin-inline-prechat-input-wrap">
                    <PhoneIcon size={14} class="helpin-inline-prechat-icon" />
                    <input
                      id="helpin-prechat-phone"
                      type="tel"
                      className="helpin-inline-prechat-input"
                      placeholder="+1 (555) 000-0000"
                      value={phone}
                      onInput={(e) => setPhone((e.target as HTMLInputElement).value)}
                      autoFocus
                    />
                  </div>
                  <button
                    type="submit"
                    className="helpin-inline-prechat-btn"
                    style={{ backgroundColor: brandColor }}
                    aria-label="Submit phone number"
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
