import * as React from 'react';
import HelpinContext from './HelpinContext'; // Assuming you created this context earlier
import { HelpinClient } from '@helpin/sdk-js';
import { PropsWithChildren } from 'react';

// Define the props to accept the client
export interface HelpinProviderProps {
  client: HelpinClient;
}

// The functional component that provides the Helpin client context
const HelpinProvider: React.FC<
  PropsWithChildren<HelpinProviderProps>
> = ({ children, client }) => {
  const Context = HelpinContext;

  // Render the provided client as value within the Context
  return <Context.Provider value={client}>{children}</Context.Provider>;
};

export default HelpinProvider;
