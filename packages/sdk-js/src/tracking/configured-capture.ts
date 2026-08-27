import type {
  FormCaptureConfig,
  FormFieldMappingTarget,
  InteractionCaptureRule,
} from '../core/types';

type CaptureClient = {
  track(eventName: string, payload?: Record<string, unknown>): void;
  lead?(payload: { email: string; [key: string]: unknown }): void;
};

const MAX_CAPTURED_VALUE_LENGTH = 256;
const SENSITIVE_FIELD_NAMES = new Set([
  'password',
  'passcode',
  'token',
  'secret',
  'card_number',
  'credit_card',
  'cvv',
  'cvc',
  'ssn',
]);

const AUTOMATIC_FIELD_MAPPINGS: Record<string, FormFieldMappingTarget> = {
  email: 'contact.email',
  email_address: 'contact.email',
  user_email: 'contact.email',
  contact_email: 'contact.email',
  name: 'contact.name',
  full_name: 'contact.name',
  fullname: 'contact.name',
  contact_name: 'contact.name',
  first_name: 'contact.first_name',
  firstname: 'contact.first_name',
  fname: 'contact.first_name',
  given_name: 'contact.first_name',
  last_name: 'contact.last_name',
  lastname: 'contact.last_name',
  lname: 'contact.last_name',
  surname: 'contact.last_name',
  family_name: 'contact.last_name',
  phone: 'contact.phone',
  phone_number: 'contact.phone',
  telephone: 'contact.phone',
  tel: 'contact.phone',
  mobile: 'contact.phone',
  role: 'contact.job_title',
  job_title: 'contact.job_title',
  jobtitle: 'contact.job_title',
  position: 'contact.job_title',
  company: 'company.name',
  company_name: 'company.name',
  organization: 'company.name',
  organisation: 'company.name',
  business: 'company.name',
  business_name: 'company.name',
  company_domain: 'company.domain',
  organization_domain: 'company.domain',
};

function normalizedFieldName(name: string): string {
  return name
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '_')
    .replace(/^_+|_+$/g, '');
}

function automaticFieldMapping(
  name: string,
  control: Element | RadioNodeList | null,
): FormFieldMappingTarget | undefined {
  const configured = AUTOMATIC_FIELD_MAPPINGS[normalizedFieldName(name)];
  if (configured) return configured;
  if (control instanceof HTMLInputElement && control.type === 'email')
    return 'contact.email';
  if (control instanceof HTMLInputElement && control.type === 'tel')
    return 'contact.phone';
  return undefined;
}

function buildLeadPayload(
  fields: Record<string, string>,
  mappings: Record<string, FormFieldMappingTarget>,
): Record<string, unknown> | null {
  const payload: Record<string, unknown> = { ...fields };
  const company: Record<string, string> = {};
  let email = '';

  for (const [fieldName, target] of Object.entries(mappings)) {
    const value = fields[fieldName]?.trim();
    if (!value) continue;
    switch (target) {
      case 'ignore':
        break;
      case 'contact.email':
        email = value;
        payload.email = value;
        break;
      case 'contact.name':
        payload.name = value;
        break;
      case 'contact.first_name':
        payload.first_name = value;
        break;
      case 'contact.last_name':
        payload.last_name = value;
        break;
      case 'contact.phone':
        payload.phone = value;
        break;
      case 'contact.job_title':
        payload.job_title = value;
        break;
      case 'company.id':
        company.id = value;
        break;
      case 'company.name':
        company.name = value;
        break;
      case 'company.domain':
        company.domain = value;
        break;
    }
  }

  if (!email || !email.includes('@')) return null;
  if (Object.keys(company).length > 0) payload.company = company;
  return payload;
}

function validRule(rule: InteractionCaptureRule): boolean {
  return (
    Boolean(rule.ruleKey.trim()) &&
    Number.isInteger(rule.version) &&
    rule.version > 0
  );
}

function safeSelectorMatch(element: Element, selector: string): boolean {
  try {
    return element.matches(selector) || element.closest(selector) !== null;
  } catch {
    return false;
  }
}

// ConfiguredCapture installs no listeners unless an explicit form or versioned
// interaction rule is supplied. It never captures passwords, files, or fields
// outside a form's allowlist.
export class ConfiguredCapture {
  private readonly forms: FormCaptureConfig[];
  private readonly rules: InteractionCaptureRule[];
  private readonly reachedMilestones = new Set<string>();
  private scrollQueued = false;

  constructor(
    private readonly client: CaptureClient,
    forms: FormCaptureConfig[] = [],
    rules: InteractionCaptureRule[] = [],
  ) {
    this.forms = forms.filter(
      (form) => Boolean(form.formId.trim()) && Boolean(form.selector.trim()),
    );
    this.rules = rules.filter(validRule);
  }

  public init(): void {
    if (this.forms.length > 0)
      document.addEventListener('submit', this.onSubmit, true);
    if (this.rules.some((rule) => (rule.clickSelectors?.length ?? 0) > 0))
      document.addEventListener('click', this.onClick, true);
    if (this.rules.some((rule) => (rule.scrollMilestones?.length ?? 0) > 0))
      window.addEventListener('scroll', this.onScroll, { passive: true });
  }

  public destroy(): void {
    document.removeEventListener('submit', this.onSubmit, true);
    document.removeEventListener('click', this.onClick, true);
    window.removeEventListener('scroll', this.onScroll);
  }

  private readonly onSubmit = (event: Event): void => {
    if (!(event.target instanceof HTMLFormElement)) return;
    const config = this.forms.find((form) =>
      safeSelectorMatch(event.target as HTMLFormElement, form.selector),
    );
    if (!config) return;
    const fields: Record<string, string> = {};
    const mappings: Record<string, FormFieldMappingTarget> = {};
    const allowed = new Set(config.fields ?? []);
    if (allowed.size > 0) {
      const data = new FormData(event.target);
      for (const name of allowed) {
        if (SENSITIVE_FIELD_NAMES.has(name.toLowerCase())) continue;
        const control = event.target.elements.namedItem(name);
        if (
          control instanceof HTMLInputElement &&
          (control.type === 'password' || control.type === 'file')
        )
          continue;
        const value = data.get(name);
        if (typeof value === 'string') {
          fields[name] = value.slice(0, MAX_CAPTURED_VALUE_LENGTH);
          const target =
            config.fieldMappings?.[name] ??
            config.field_mappings?.[name] ??
            automaticFieldMapping(name, control);
          if (target) mappings[name] = target;
        }
      }
    }
    const lead = buildLeadPayload(fields, mappings);
    if (lead) this.client.lead?.(lead as { email: string; [key: string]: unknown });
    this.client.track('$form', {
      form_id: config.formId,
      field_names: Object.keys(fields),
      fields,
      field_mappings: mappings,
    });
  };

  private readonly onClick = (event: MouseEvent): void => {
    if (!(event.target instanceof Element)) return;
    for (const rule of this.rules) {
      const selector = rule.clickSelectors?.find((candidate) =>
        safeSelectorMatch(event.target as Element, candidate),
      );
      if (!selector) continue;
      this.client.track('$interaction', {
        interaction_type: 'click',
        capture_rule_key: rule.ruleKey,
        capture_rule_version: rule.version,
        selector,
      });
    }
  };

  private readonly onScroll = (): void => {
    if (this.scrollQueued) return;
    this.scrollQueued = true;
    window.requestAnimationFrame(() => {
      this.scrollQueued = false;
      const height = Math.max(
        document.documentElement.scrollHeight - window.innerHeight,
        0,
      );
      const percent =
        height === 0
          ? 100
          : Math.min(100, Math.round((window.scrollY / height) * 100));
      for (const rule of this.rules) {
        for (const milestone of rule.scrollMilestones ?? []) {
          if (
            !Number.isFinite(milestone) ||
            milestone <= 0 ||
            milestone > 100 ||
            percent < milestone
          )
            continue;
          const key = `${rule.ruleKey}:${rule.version}:${milestone}`;
          if (this.reachedMilestones.has(key)) continue;
          this.reachedMilestones.add(key);
          this.client.track('$interaction', {
            interaction_type: 'scroll',
            capture_rule_key: rule.ruleKey,
            capture_rule_version: rule.version,
            scroll_milestone: milestone,
          });
        }
      }
    });
  };
}
