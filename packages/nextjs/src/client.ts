import {
  HelpinClient,
  helpinClient,
  HelpinOptions,
} from '@helpin-ai/sdk-js';

function createClient(params: HelpinOptions): HelpinClient | null {
  if (typeof window === 'undefined') {
    return null;
  }
  return helpinClient(params);
}

export default createClient;
