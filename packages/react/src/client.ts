import {
  HelpinClient,
  helpinClient,
  HelpinOptions,
} from '@helpin-ai/sdk-js';

function createClient(params: HelpinOptions): HelpinClient | null {
  return helpinClient(params);
}

export default createClient;
