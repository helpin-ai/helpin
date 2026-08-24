import type { FormCaptureConfig, InteractionCaptureRule } from '../core/types';

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
        if (typeof value === 'string')
          fields[name] = value.slice(0, MAX_CAPTURED_VALUE_LENGTH);
      }
    }
    if (fields.email && fields.email.includes('@'))
      this.client.lead?.({ email: fields.email, ...fields });
    this.client.track('$form', {
      form_id: config.formId,
      field_names: Object.keys(fields),
      fields,
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
