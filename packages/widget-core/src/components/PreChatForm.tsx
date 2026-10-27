import { FunctionComponent } from 'preact';
import { useState, useRef, useEffect } from 'preact/hooks';
import type { WidgetConfig } from '../types';
import { MailIcon, PhoneIcon, XIcon } from './icons';

interface PreChatFormProps {
  config: WidgetConfig;
  onSubmit: (data: { phone: string; email: string }) => void | Promise<void>;
  purpose?: 'chat' | 'handoff';
  onDismiss?: () => void;
}

export const PreChatForm: FunctionComponent<PreChatFormProps> = ({
  config,
  onSubmit,
  purpose = 'handoff',
  onDismiss,
}) => {
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');
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

  const requirePhone = config.features?.requirePhone === true;
  const emailRequired = config.features?.forceIdentify === true;
  const isOnline = config.availability?.isOnline !== false;
  const emailDescription = purpose === 'chat'
    ? 'Continue chatting, and get replies by email if you leave.'
    : isOnline
      ? 'Get replies from our team, even if you leave the chat.'
      : 'Our team is currently offline. Leave your email and we’ll notify you when someone replies.';
  const continueLabel = purpose === 'chat' ? 'Continue chatting' : 'Continue to human support';
  const emailSubmitLabel = requirePhone ? 'Continue' : continueLabel;

  const save = async (data: { phone: string; email: string }) => {
    if (saving) return;
    setSaving(true);
    setError('');
    try {
      await onSubmit(data);
      setStep('done');
    } catch {
      setError('Could not save your details. Please try again.');
    } finally {
      setSaving(false);
    }
  };
  const handleEmailSubmit = (e: Event) => {
    e.preventDefault();
    if (!email.trim() || saving) return;
    if (requirePhone) setStep('phone');
    else void save({ phone: '', email: email.trim() });
  };
  const handleSkipEmail = () => {
    if (requirePhone) setStep('phone');
    else void save({ phone: '', email: '' });
  };
  const handlePhoneSubmit = (e: Event) => {
    e.preventDefault();
    void save({ phone: phone.trim(), email: email.trim() });
  };

  if (step === 'done') return null;

  return (
    <div className="helpin-human-contact-card" ref={formRef} role="region" aria-label="Enter your email">
      {onDismiss && !emailRequired && !saving && (
        <button type="button" className="helpin-email-dismiss" aria-label="Dismiss email request" onClick={onDismiss}><XIcon size={16} /></button>
      )}
      {error && <p role="alert" className="helpin-email-error">{error}</p>}
      {step === 'email' && (
        <>
          <div className="helpin-human-contact-card__heading">
            <span className="helpin-human-contact-card__title">Enter your email</span>
            <span className="helpin-human-contact-card__description">{emailDescription}</span>
          </div>
          <form onSubmit={handleEmailSubmit} className="helpin-human-contact-form">
            <label className="helpin-sr-only" htmlFor="helpin-prechat-email">Email address</label>
            <div className="helpin-human-contact-input-wrap">
              <MailIcon size={16} class="helpin-human-contact-icon" />
              <input
                id="helpin-prechat-email"
                type="email"
                className="helpin-human-contact-input"
                placeholder="you@example.com"
                value={email}
                onInput={(e) => setEmail((e.target as HTMLInputElement).value)}
                required
                autoFocus
              />
            </div>
            <button
              type="submit"
              className="helpin-human-contact-primary"
              style={{ backgroundColor: brandColor }}
              disabled={!email.trim() || saving}
            >
              {saving ? 'Saving…' : emailSubmitLabel}
            </button>
          </form>
          {!emailRequired && (!onDismiss || purpose === 'handoff') && (
            <button
              type="button"
              className="helpin-human-contact-secondary"
              onClick={handleSkipEmail}
            >
              Continue without email
            </button>
          )}
          <span className="helpin-human-contact-card__privacy">
            Your email is used only for this conversation.
          </span>
        </>
      )}
      {step === 'phone' && (
        <>
          <div className="helpin-human-contact-card__heading">
            <span className="helpin-human-contact-card__title">One more detail</span>
            <span className="helpin-human-contact-card__description">
              What phone number should our team use?
            </span>
          </div>
          <form onSubmit={handlePhoneSubmit} className="helpin-human-contact-form">
            <label className="helpin-sr-only" htmlFor="helpin-prechat-phone">Your phone number</label>
            <div className="helpin-human-contact-input-wrap">
              <PhoneIcon size={16} class="helpin-human-contact-icon" />
              <input
                id="helpin-prechat-phone"
                type="tel"
                className="helpin-human-contact-input"
                placeholder="+1 (555) 000-0000"
                value={phone}
                onInput={(e) => setPhone((e.target as HTMLInputElement).value)}
                autoFocus
              />
            </div>
            <button
              type="submit"
              className="helpin-human-contact-primary"
              style={{ backgroundColor: brandColor }}
              disabled={saving}
            >
              {saving ? 'Saving…' : continueLabel}
            </button>
          </form>
          <button
            type="button"
            className="helpin-human-contact-secondary"
            disabled={saving}
            onClick={() => void save({ phone: '', email: email.trim() })}
          >
            Continue without phone
          </button>
        </>
      )}
    </div>
  );
};
