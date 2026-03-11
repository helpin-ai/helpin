import {
  HelpinClient,
  helpinClient,
  HelpinOptions,
} from '@helpin/sdk-js';

function createClient(params: HelpinOptions): HelpinClient {
  return helpinClient(params);
}

export default createClient;
