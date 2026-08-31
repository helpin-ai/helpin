import { renderToStaticMarkup } from 'react-dom/server';
import { describe, expect, it } from 'vitest';

import { QuietBreadcrumbs, QuietDetailHeader, QuietEmptyState, QuietIdentityHeader, QuietPageHeader, QuietPrimaryAction, QuietStatusText, QuietTitleInput, QuietTitleTextarea } from '../quiet';
import { SaveIndicator } from '@/components/pm/SaveIndicator';
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';

describe('Quiet Hairline primitives', () => {
  it('preserves Helpin small-button geometry for primary actions', () => {
    const markup = renderToStaticMarkup(<QuietPrimaryAction>New flow</QuietPrimaryAction>);

    expect(markup).toContain('rounded-4xl');
    expect(markup).toContain('h-8');
    expect(markup).toContain('px-3');
    expect(markup).toContain('text-sm');
    expect(markup).not.toContain('rounded-[7px]');
    expect(markup).not.toContain('px-4');
  });

  it('keeps entity title inputs at 26px on desktop', () => {
    const markup = renderToStaticMarkup(<QuietTitleInput aria-label="Entity name" value="Acme" readOnly />);
    const textareaMarkup = renderToStaticMarkup(<QuietTitleTextarea aria-label="Document name" value="Guide" readOnly />);

    expect(markup).toContain('text-[26px]');
    expect(markup).toContain('md:text-[26px]');
    expect(markup).not.toContain('md:text-sm');
    expect(textareaMarkup).toContain('text-[26px]');
    expect(textareaMarkup).toContain('[field-sizing:content]');
  });

  it('composes a two-level sidebar-safe detail header', () => {
    const markup = renderToStaticMarkup(
      <QuietDetailHeader
        breadcrumbs={<QuietBreadcrumbs items={[{ id: 'contacts', label: 'Contacts' }]} onBack={() => undefined} backLabel="Back to contacts" />}
        avatar={<span>AC</span>}
        title="Ada Cole"
        meta="VP Sales"
        state={<QuietStatusText tone="lifecycle">Customer</QuietStatusText>}
        actions={<QuietPrimaryAction>New deal</QuietPrimaryAction>}
      />,
    );

    expect(markup).toContain('aria-label="Breadcrumb"');
    expect(markup).toContain('aria-label="Back to contacts"');
    expect(markup).toContain('text-[26px]');
    expect(markup).toContain('border-quiet-divider-strong');
    expect(markup).toContain('group-data-[sidebar-toggle-visible=true]/workspace-main:pl-14');
    expect(markup).toContain('md:flex-row');
    expect(markup).toContain('pb-1.5');
    expect(markup).toContain('pt-2');
    expect(markup).toContain('mb-0.5');
    expect(markup).toContain('mt-0.5');
    expect(markup.indexOf('New deal')).toBeLessThan(markup.indexOf('Customer'));
    expect(markup).toContain('flex-col items-start gap-1.5');
    expect(markup).not.toContain('rounded-full border');
  });

  it('keeps page headers at 20px and identity headers at 24px', () => {
    const pageMarkup = renderToStaticMarkup(<QuietPageHeader title="Meetings" />);
    const identityMarkup = renderToStaticMarkup(<QuietIdentityHeader title="Customer call" />);

    expect(pageMarkup).toContain('text-[20px]');
    expect(identityMarkup).toContain('text-[24px]');
    expect(pageMarkup).not.toContain('text-[26px]');
    expect(identityMarkup).not.toContain('text-[26px]');
  });

  it('provides compact shell geometry for page-owned index headers', () => {
    const markup = renderToStaticMarkup(<QuietPageHeader variant="shell" title="Tasks" context="Platform" />);

    expect(markup).toContain('py-4');
    expect(markup).toContain('border-quiet-divider-strong');
    expect(markup).toContain('px-4');
    expect(markup).toContain('lg:px-8');
    expect(markup).toContain('text-[20px]');
    expect(markup).not.toContain('text-[24px]');
    expect(markup).toContain('Tasks');
    expect(markup).toContain(' (Platform)');
    expect(markup).toContain('text-quiet-text-tertiary');
    expect(markup).toContain('group-data-[sidebar-toggle-visible=true]/workspace-main:pl-14');
  });

  it('keeps route navigation inside the same sidebar-safe header contract', () => {
    const markup = renderToStaticMarkup(
      <QuietPageHeader
        title="Install the widget"
        navigation={<nav aria-label="Breadcrumb">Support</nav>}
      />,
    );

    expect(markup).toContain('aria-label="Breadcrumb"');
    expect(markup).toContain('Support');
    expect(markup).toContain('mb-2');
    expect(markup).toContain('group-data-[sidebar-toggle-visible=true]/workspace-main:pl-14');
  });

  it('uses hairline tabs with inline counts instead of pills', () => {
    const markup = renderToStaticMarkup(
      <Tabs defaultValue="overview">
        <TabsList variant="quiet" aria-label="Views">
          <TabsTrigger value="overview">
            Overview <span className="tabular-nums">3</span>
          </TabsTrigger>
          <TabsTrigger value="activity">Activity</TabsTrigger>
        </TabsList>
      </Tabs>,
    );

    expect(markup).toContain('role="tablist"');
    expect(markup).toContain('data-variant="quiet"');
    expect(markup).toContain('after:bg-quiet-text-primary');
    expect(markup).toContain('group-data-[variant=quiet]/tabs-list:text-[14px]');
    expect(markup).toContain('group-data-[variant=quiet]/tabs-list:font-medium');
    expect(markup).toContain('group-data-[variant=quiet]/tabs-list:data-active:font-semibold');
    expect(markup).not.toContain('group-data-[variant=quiet]/tabs-list:text-[13px]');
    expect(markup).not.toContain('group-data-[variant=quiet]/tabs-list:text-sm');
    expect(markup).not.toContain('group-data-[variant=quiet]/tabs-list:font-normal');
    expect(markup).toContain('tabular-nums');
    expect(markup).toContain('data-[variant=quiet]:rounded-none');
    expect(markup).toContain('group-data-[variant=quiet]/tabs-list:rounded-none');
  });

  it('keeps empty states left aligned and bounded by hairlines', () => {
    const markup = renderToStaticMarkup(<QuietEmptyState title="Nothing yet" description="Add the first item." />);

    expect(markup).toContain('border-y');
    expect(markup).toContain('text-left');
    expect(markup).not.toContain('text-center');
  });

  it('supports a pulsing current status without a chip container', () => {
    const markup = renderToStaticMarkup(
      <QuietStatusText tone="current" pulse>
        Recording
      </QuietStatusText>,
    );

    expect(markup).toContain('bg-quiet-text-primary');
    expect(markup).toContain('motion-safe:animate-pulse');
    expect(markup).not.toContain('rounded-full border');
  });

  it('renders quiet save failures without a contradictory saved state', () => {
    const markup = renderToStaticMarkup(<SaveIndicator saving={false} error="Failed to save" presentation="quiet" />);

    expect(markup).toContain('Failed to save');
    expect(markup).toContain('text-quiet-accent');
    expect(markup).not.toContain('All changes saved');
    expect(markup).not.toContain('rounded-full');
  });
});
