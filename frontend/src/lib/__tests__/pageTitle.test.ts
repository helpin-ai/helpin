import { describe, expect, it } from 'vitest'
import { getPageTitle } from '../pageTitle'

describe('getPageTitle', () => {
  it.each([
    ['/login', 'Sign In'],
    ['/join/invite-token', 'Join Workspace'],
    ['/w/acme/setup', 'Setup'],
    ['/w/acme/support/coverage', 'Support Coverage'],
    ['/w/acme/settings/inboxes-routing', 'Inboxes & Routing Settings'],
    ['/w/acme/crm/contacts/contact-1', 'Contact'],
    ['/w/acme/docs/documents/doc-1', 'Document'],
    ['/w/acme/pm/tasks/task-1', 'Task'],
    ['/w/acme/team-goals', 'Team Goals'],
    ['/unknown', 'Helpin'],
  ])('maps %s to %s', (pathname, expectedTitle) => {
    expect(getPageTitle(pathname)).toBe(expectedTitle)
  })
})
