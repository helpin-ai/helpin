import {
  HelpinClient,
  helpinClient,
  type HelpinOptions,
} from '@helpin-ai/sdk-js';

export default function createClient(
  options: HelpinOptions,
): HelpinClient | null {
  if (typeof window === 'undefined') {
    return null;
  }

  return helpinClient(options);
}
