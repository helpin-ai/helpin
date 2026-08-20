import { describe, expect, it } from 'vitest'

import { supportPermissionFlags } from '@mobile/lib/use-workspace-permissions'
import type { WorkspaceAccess } from '@mobile/lib/types'

function access(permissions: string[], modules: string[] = ['support']): WorkspaceAccess {
  return {
    workspace_id: 'ws-1',
    membership: {
      id: 'member-1',
      user_id: 'user-1',
      role: 'member',
      status: 'active',
    },
    permissions,
    team_memberships: [],
    modules,
  }
}

describe('supportPermissionFlags', () => {
  it('keeps viewers read-only', () => {
    expect(supportPermissionFlags(access(['support.read']))).toEqual({
      canReadSupport: true,
      canEditSupport: false,
      canAdminSupport: false,
      canReadPM: false,
      canEditPM: false,
    })
  })

  it('exposes edit and admin permissions independently', () => {
    expect(
      supportPermissionFlags(
        access(['support.read', 'support.edit', 'support.admin']),
      ),
    ).toEqual({
      canReadSupport: true,
      canEditSupport: true,
      canAdminSupport: true,
      canReadPM: false,
      canEditPM: false,
    })
  })

  it('denies support access when the RBAC role has permissions but the module grant is absent', () => {
    expect(
      supportPermissionFlags(
        access(['support.read', 'support.edit'], ['pm', 'docs']),
      ),
    ).toEqual({
      canReadSupport: false,
      canEditSupport: false,
      canAdminSupport: false,
      canReadPM: false,
      canEditPM: false,
    })
  })

  it('requires both the PM module and matching permissions for linked tasks', () => {
    expect(supportPermissionFlags(access(['support.read', 'pm.read', 'pm.edit'], ['support', 'pm']))).toMatchObject({
      canReadPM: true,
      canEditPM: true,
    })
    expect(supportPermissionFlags(access(['support.read', 'pm.read'], ['support']))).toMatchObject({
      canReadPM: false,
      canEditPM: false,
    })
  })
})
