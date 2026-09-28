import type { CustomerPortalRequest } from '@/lib/services/customerPortalService'

/** requestNumber is the short ticket number, falling back to the reference. */
export function requestNumber(request: Pick<CustomerPortalRequest, 'number' | 'reference'>) {
  return request.number ? `#${request.number}` : request.reference
}
