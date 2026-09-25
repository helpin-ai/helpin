/** Who may create an account on a self-hosted server. */
export type SignupMode = 'open' | 'invite_only' | 'domains';

/** GET/PUT /api/instance/signup-policy (server admins). */
export interface SignupPolicy {
  mode: SignupMode;
  allowed_domains: string[];
  /** Domain signup sends a verification email, so it needs application email. */
  app_email_configured: boolean;
  recommend_invite_only: boolean;
}

/** GET/POST/DELETE /api/instance/admins. */
export interface ServerAdmin {
  user_id: string;
  email: string;
  full_name: string;
  /** `env` admins come from HELPIN_ADMIN_EMAILS and cannot be removed in the app. */
  source: 'granted' | 'env';
}

/** GET/PUT /api/instance/email. The password is write-only. */
export interface AppEmailSettings {
  source: 'env' | 'database' | 'none';
  provider: '' | 'smtp' | 'postmark';
  editable: boolean;
  host: string;
  port: number;
  username: string;
  from: string;
  tls_mode: '' | 'starttls' | 'tls' | 'none';
  password_set: boolean;
  problem?: string;
}

/** A null password keeps the stored one; an empty string clears it. */
export interface AppEmailSettingsUpdate {
  host: string;
  port: number;
  username: string;
  password: string | null;
  from: string;
  tls_mode: 'starttls' | 'tls' | 'none';
}
