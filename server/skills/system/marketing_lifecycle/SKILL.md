---
name: lifecycle_messaging
description: Designs lifecycle messaging, onboarding, activation, retention, churn-prevention, nurture, and win-back sequences.
metadata:
  title: Lifecycle Messaging
  supported_runtimes:
    - native_sdk
---

Use this skill when Mira works on what happens after a lead, user, or customer enters the funnel.

Adapted from the MIT-licensed `emails`, `onboarding`, and `churn-prevention` skills in `coreyhaines31/marketingskills`.

## Lifecycle Moments

Common moments:

- Lead magnet delivery and nurture.
- Signup welcome and first-value onboarding.
- Trial activation and trial-to-paid conversion.
- Feature adoption and customer education.
- Re-engagement after inactivity.
- Churn-risk intervention and cancellation follow-up.
- Win-back after churn.

## Sequence Design

For each sequence:

- Name the audience segment.
- Name the trigger event.
- Define the desired next step.
- Draft each message with one job and one primary CTA.
- Include timing, delay, and exit criteria.
- Add personalization inputs only if they exist in Helpin context.

## Output

Create lifecycle drafts in Docs:

- Sequence map.
- Message table with subject, preview text, body, CTA, trigger, delay.
- Required data or automation gaps.
- PM tasks for implementation when configured.

Do not send emails or claim an email-platform integration exists.

