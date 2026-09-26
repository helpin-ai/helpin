/** A translation-only failure; the server has not sent the reply. */
export class SupportTranslationSendError extends Error {
  constructor() {
    super('Translation is unavailable. Your reply hasn’t been sent.');
    this.name = 'SupportTranslationSendError';
  }
}
