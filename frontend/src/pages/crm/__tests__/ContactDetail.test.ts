import { describe, expect, it } from 'vitest';

import {
  buyerSignalsSectionClassName,
  contactDetailDefaultFieldKeys,
  contactDetailExpandedFieldKeys,
  contactDetailOverviewGridClassName,
  contactDetailOverviewContentClassName,
  contactDetailOverviewSectionOrder,
  contactDetailSidebarSectionTitles,
  copyableRailValueClassName,
  deriveCRMContactInteractionSummary,
  editableRailFieldClassName,
  formatCRMContactLocation,
  normalizeCRMContactLabels,
  sidebarPopoverSelectTriggerClassName,
} from '../ContactDetail';
import { SOCIAL_PLATFORM_META } from '@/components/docs/helpcenter/SocialPlatformIcon';

describe('contactDetailOverviewGridClassName', () => {
  it('uses a wider right rail for the overview contact profile', () => {
    expect(contactDetailOverviewGridClassName).toContain('lg:grid-cols-[1fr_360px]');
  });
});

describe('contact profile rail fields', () => {
  it('leaves breathing room below the buyer signals area', () => {
    expect(buyerSignalsSectionClassName).not.toContain('pb-14');
  });

  it('leaves bottom space in the main overview content area', () => {
    expect(contactDetailOverviewContentClassName).toContain('pb-28');
  });

  it('does not duplicate buyer signals in the sidebar', () => {
    expect(contactDetailSidebarSectionTitles).not.toContain('Signals');
  });

  it('places recent activity at the bottom of the overview flow', () => {
    expect(contactDetailOverviewSectionOrder.at(-1)).toBe('recent_activity');
  });

  it('keeps high-signal profile attributes visible by default', () => {
    expect(contactDetailDefaultFieldKeys).toEqual(expect.arrayContaining([
      'linkedin_url',
      'primary_location',
      'labels',
      'description',
      'last_interaction',
    ]));
    expect(contactDetailDefaultFieldKeys).not.toContain('facebook_url');
    expect(contactDetailExpandedFieldKeys).toEqual(expect.arrayContaining([
      'facebook_url',
      'instagram_url',
      'x_url',
      'angellist_url',
      'first_email',
      'first_interaction',
    ]));
  });

  it('formats CRM contact location like the support profile location', () => {
    expect(formatCRMContactLocation({
      primary_location: 'Downers Grove, Illinois',
      country_name: 'United States',
    })).toBe('Downers Grove, Illinois, United States');
  });

  it('normalizes freeform CRM labels', () => {
    expect(normalizeCRMContactLabels(' Buyer , , VIP, buyer ')).toEqual(['Buyer', 'VIP']);
  });

  it('uses subtle empty-field styling without a filled background', () => {
    const className = editableRailFieldClassName('');

    expect(className).toContain('border-border/60');
    expect(className).toContain('bg-transparent');
    expect(className).not.toContain('border-dashed');
    expect(className).not.toContain('bg-background/70');
  });

  it('keeps selector and copy affordances aligned in the value column', () => {
    expect(sidebarPopoverSelectTriggerClassName).toContain('border-border/60');
    expect(sidebarPopoverSelectTriggerClassName).toContain('bg-transparent');
    expect(copyableRailValueClassName).toContain('grid-cols-[minmax(0,1fr)_20px]');
  });

  it('uses the shared help center SVG brand icons for supported socials', () => {
    expect(SOCIAL_PLATFORM_META.linkedin.brandIcon).toBe('linkedin');
    expect(SOCIAL_PLATFORM_META.facebook.brandIcon).toBe('facebook');
    expect(SOCIAL_PLATFORM_META.instagram.brandIcon).toBe('instagram');
    expect(SOCIAL_PLATFORM_META.x.brandIcon).toBe('x-twitter');
  });

  it('derives the last interaction across emails, meetings, notes, and support', () => {
    const summary = deriveCRMContactInteractionSummary({
      emails: [{ sent_at: '2026-01-02T12:00:00Z' }],
      meetings: [{ start_time: '2026-01-04T12:00:00Z' }],
      activities: [{ occurred_at: '2026-01-03T12:00:00Z' }],
      supportConversations: [{ updated_at: '2026-01-05T12:00:00Z' }],
    });

    expect(summary.lastInteraction?.kind).toBe('support');
    expect(summary.lastInteraction?.timestamp).toBe('2026-01-05T12:00:00Z');
    expect(summary.firstInteraction?.timestamp).toBe('2026-01-02T12:00:00Z');
  });
});
