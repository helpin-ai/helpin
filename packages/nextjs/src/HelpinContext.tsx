import { createContext } from 'react';
import { HelpinClient } from '@helpin-ai/sdk-js';

const HelpinContext = createContext<HelpinClient | null>(null);

export default HelpinContext;
