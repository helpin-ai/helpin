export interface CLIAuthorizationQuery {
  client_id: string;
  redirect_uri: string;
  response_type: string;
  scope: string;
  state: string;
  code_challenge: string;
  code_challenge_method: string;
  resource: string;
}
export interface CLIConsentRequest {
  client_name: string;
  query: CLIAuthorizationQuery;
  workspaces: { id: string; name: string; role: string }[];
}
export function parseCLIAuthorizationQuery(search: Record<string, unknown>): CLIAuthorizationQuery {
  const text = (key: string) => typeof search[key] === 'string' ? search[key] as string : '';
  return { client_id: text('client_id'), redirect_uri: text('redirect_uri'), response_type: text('response_type'),
    scope: text('scope'), state: text('state'), code_challenge: text('code_challenge'),
    code_challenge_method: text('code_challenge_method'), resource: text('resource') };
}
