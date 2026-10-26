import { describe, expect, it } from 'vitest';
import { isMessageInOtherLanguage, isMessageInReadingLanguage } from '../messageReadingLanguage';

const english = 'I can help you update your email address. Please open your account settings and select your profile.';
const spanish = 'Puedes cambiar la dirección de correo electrónico desde la configuración de tu cuenta. Abre tu perfil para continuar.';
describe('message reading language', () => {
  it('recognizes English without treating Latin script as English', () => {
    expect(isMessageInReadingLanguage(english, 'en')).toBe(true);
    expect(isMessageInReadingLanguage(spanish, 'en')).toBe(false);
    expect(isMessageInReadingLanguage('aap apni account settings mein ja kar email address tabdeel kar sakte hain.', 'en')).toBe(false);
  });
  it('uses the configured reading language', () => {
    expect(isMessageInReadingLanguage(spanish, 'es')).toBe(true);
    expect(isMessageInReadingLanguage(english, 'es')).toBe(false);
  });
  it.each(['Hello', 'Hola', 'OK', '123456', 'https://example.com/account', '```js\nconst message = "Your account settings are available here";\n```'])('keeps manual translation for ambiguous text: %s', text => {
    expect(isMessageInReadingLanguage(text, 'en')).toBe(false);
  });
  it('does not let a long English section hide a Spanish paragraph', () => {
    expect(isMessageInReadingLanguage(`${english.repeat(5)}\n\n${spanish}`, 'en')).toBe(false);
  });
  it('keeps mixed-language sentences translatable within one paragraph', () => {
    expect(isMessageInReadingLanguage(`${english.repeat(5)} ${spanish}`, 'en')).toBe(false);
  });
  it('recognizes regional English reading preferences', () => {
    expect(isMessageInReadingLanguage(english, 'en-GB')).toBe(true);
    expect(isMessageInReadingLanguage(english, ' EN_us ')).toBe(true);
  });
  it('ignores quoted history and code while classifying the authored text', () => {
    expect(isMessageInReadingLanguage(`${english}\n> ${spanish}\n\n\`\`\`\n${spanish}\n\`\`\``, 'en')).toBe(true);
  });
});

describe('message in another language', () => {
  const steps = 'Thanks for reaching out!\n\nTo reconnect your Instagram account:\n\n1. Go to **Settings > Social Accounts**\n2. Click **Reconnect** next to Instagram\n\nLet me know if that fixes it.';
  it.each(['Thanks, it is working now!', 'How do I cancel my subscription?', 'To reconnect your Instagram account:', steps, english, '👍', 'Hi, my name is 张伟'])('does not flag English for an English reader: %s', text => {
    expect(isMessageInOtherLanguage(text, 'en')).toBe(false);
  });
  it.each([spanish, 'Ich kann mein Instagram Konto nicht verbinden', 'مرحبا، أحتاج مساعدة', '你好，我需要帮助', `${english.repeat(5)}\n\n${spanish}`])('flags clear evidence of another language: %s', text => {
    expect(isMessageInOtherLanguage(text, 'en')).toBe(true);
  });
  it('uses the configured reading language', () => {
    expect(isMessageInOtherLanguage(spanish, 'es')).toBe(false);
    expect(isMessageInOtherLanguage(english, 'es')).toBe(true);
  });
  it('keeps manual translation for reading languages the local detector cannot check', () => {
    expect(isMessageInOtherLanguage(english, 'zh-Hans')).toBe(true);
  });
});
