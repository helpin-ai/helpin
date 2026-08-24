import { beforeEach, describe, expect, it, vi } from 'vitest';
import { closeDealRoute, matchDealRoute, openDealRoute } from '../dealRouteNavigation';
import { useDealPanelStore } from '@/stores/dealPanelStore';

describe('deal route navigation', () => {
  beforeEach(() => {
    useDealPanelStore.setState({
      dealId: null,
      requestKey: 0,
      lastClosedDealId: null,
      lastClosedAt: 0,
    });
  });

  it('opens a contextual deal without replacing the background route', () => {
    const navigate = vi.fn();

    openDealRoute(navigate, { pathname: '/w/acme/crm/companies/company-1' }, 'acme', 'deal-1');

    expect(navigate).not.toHaveBeenCalled();
    expect(useDealPanelStore.getState().dealId).toBe('deal-1');
  });

  it('switches canonical deal routes through navigation', () => {
    const navigate = vi.fn();

    openDealRoute(navigate, { pathname: '/w/acme/crm/deals/deal-1' }, 'acme', 'deal-2');

    expect(navigate).toHaveBeenCalledWith({
      to: '/w/$slug/crm/deals/$dealId',
      params: { slug: 'acme', dealId: 'deal-2' },
    });
  });

  it('closes contextual deals in place and canonical deals back to the deal list', () => {
    const navigate = vi.fn();
    useDealPanelStore.getState().openDeal('deal-1');

    closeDealRoute(navigate, { pathname: '/w/acme/crm/contacts/contact-1' }, 'acme');
    expect(useDealPanelStore.getState().dealId).toBeNull();
    expect(navigate).not.toHaveBeenCalled();

    closeDealRoute(navigate, { pathname: '/w/acme/crm/deals/deal-2' }, 'acme');
    expect(navigate).toHaveBeenCalledWith({
      to: '/w/$slug/crm/deals/',
      params: { slug: 'acme' },
    });
  });

  it('matches only canonical deal detail routes', () => {
    expect(matchDealRoute('/w/acme/crm/deals/deal-1')).toEqual({ slug: 'acme', dealId: 'deal-1' });
    expect(matchDealRoute('/w/acme/crm/deals')).toBeNull();
    expect(matchDealRoute('/w/acme/crm/companies/company-1')).toBeNull();
  });
});
