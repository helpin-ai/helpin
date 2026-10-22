import { describe, expect, it } from 'vitest';
import { isMessageInReadingLanguage } from '../messageReadingLanguage';

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
