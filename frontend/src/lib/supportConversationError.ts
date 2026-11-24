/** A conversation detail request failure that keeps the HTTP status. */
export class SupportConversationLoadError extends Error {
  readonly status?: number;

  constructor(message: string, status?: number) {
    super(message);
    this.name = 'SupportConversationLoadError';
    this.status = status;
  }
}

/** Missing and access-denied conversations look the same to the viewer. */
export function isConversationUnavailableError(error: unknown): boolean {
  return error instanceof SupportConversationLoadError && (error.status === 403 || error.status === 404);
}
