import { describe, expect, it } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

import {
  buyerSignalsSectionClassName,
  contactDetailDefaultFieldKeys,
  contactDetailExpandedFieldKeys,
  contactDetailCollapsedGridClassName,
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
import { normalizeContactDetailTab } from '@/lib/contactDetailTabs';

const contactDetailSource = readFileSync(resolve(__dirname, '../ContactDetail.tsx'), 'utf8');
const detailCollectionsSource = readFileSync(resolve(__dirname, '../../../components/crm/CompanyDetailCollections.tsx'), 'utf8');
const contactRouteSource = readFileSync(resolve(__dirname, '../../../routes/_authenticated/w/$slug/crm/contacts/$contactId.tsx'), 'utf8');

describe('contactDetailOverviewGridClassName', () => {
  it('uses a wider right rail for the overview contact profile', () => {
    expect(contactDetailOverviewGridClassName).toContain('lg:grid-cols-[1fr_360px]');
  });

  it('reserves only the vertical toggle bar when the desktop rail is minimized', () => {
    expect(contactDetailCollapsedGridClassName).toContain('lg:grid-cols-[minmax(0,1fr)_40px]');
  });
});

describe('contact detail navigation', () => {
  it('normalizes missing and invalid tab values to overview', () => {
    expect(normalizeContactDetailTab('tasks')).toBe('tasks');
    expect(normalizeContactDetailTab('companies')).toBe('overview');
    expect(normalizeContactDetailTab(undefined)).toBe('overview');
  });

  it('uses URL-backed specialist tabs with a persistent detail rail', () => {
    for (const tab of ['Overview', 'Tasks', 'Emails', 'Meetings', 'Calls', 'Deals', 'Support', 'Notes']) {
      expect(contactDetailSource).toContain(`label: '${tab}'`);
    }
    expect(contactRouteSource).toContain('normalizeContactDetailTab');
    expect(contactRouteSource).toContain('validateSearch');
    expect(contactDetailSource).toContain('<ContactMeetingsView');
    expect(contactDetailSource).toContain('<ContactDealsView');
    expect(contactDetailSource).toContain('<ContactSupportView');
    expect(contactDetailSource).toContain('<EmailTimeline');
    expect(contactDetailSource).toContain('fullHeight');
    expect(contactDetailSource).toContain('filterControl="dropdown"');
    expect(contactDetailSource).toContain("mobileDetailsOpen");
    expect(contactDetailSource).toContain("'hidden lg:flex'");
    expect(contactDetailSource).toContain("setDesktopDetailsCollapsed(activeTab !== 'overview')");
    expect(contactDetailSource).toContain('aria-label="Open contact details"');
    expect(contactDetailSource).toContain('aria-label="Minimize contact details"');
    expect(contactDetailSource).toContain('emailRecipient={contact.email}');
  });

  it('uses the shared borderless treatment for contact tab actions', () => {
    expect(detailCollectionsSource).toContain('export function ContactDealsView');
    expect(detailCollectionsSource).toContain('export function ContactMeetingsView');
    expect(detailCollectionsSource).toContain('export function ContactSupportView');
    expect(detailCollectionsSource).toContain(
      "getOptionalSectionActionClass(linkOpen ? 'open' : 'available', 'borderless')",
    );
    expect(detailCollectionsSource).toContain('<PlusSignIcon className="h-[15px] w-[15px]" />');
  });
});

describe('contact profile rail fields', () => {
  it('leaves breathing room below the buyer signals area', () => {
    expect(buyerSignalsSectionClassName).not.toContain('pb-14');
  });

  it('lets the unified overview sections own their spacing', () => {
    expect(contactDetailOverviewContentClassName).toBe('mt-0');
  });

  it('does not duplicate buyer signals in the sidebar', () => {
    expect(contactDetailSidebarSectionTitles).not.toContain('Signals');
  });

  it('places the unified activity stream at the bottom of the overview flow', () => {
    expect(contactDetailOverviewSectionOrder).toEqual(['summary_signals', 'activity']);
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
