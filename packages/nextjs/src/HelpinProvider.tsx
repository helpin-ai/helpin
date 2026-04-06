import * as React from 'react';
import HelpinContext from './HelpinContext';
import { HelpinClient } from '@helpin-ai/sdk-js';
import { PropsWithChildren } from 'react';

export interface HelpinProviderProps {
  client: HelpinClient | null;
}

const HelpinProvider: React.FC<PropsWithChildren<HelpinProviderProps>> =
  function ({ children, client }) {
    const Context = HelpinContext;
    return <Context.Provider value={client}>{children}</Context.Provider>;
  };

export default HelpinProvider;
