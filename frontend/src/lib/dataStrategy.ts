// Central Data Strategy Manager
// Determines whether to use mock data or database based on workspace

export interface DataStrategy {
  useMockData: boolean
  workspaceId: string
  workspaceName: string
  description: string
}

// Configuration for which workspaces use mock data
const MOCK_WORKSPACE_IDS = ['1', 'demo', 'demo-demo-demo-demo-demolicious', 'mock'] // Product A and any demo workspaces

export function getDataStrategy(workspaceId: string): DataStrategy {
  const useMockData = MOCK_WORKSPACE_IDS.includes(workspaceId)

  // Get workspace info based on ID
  const workspaceInfo = getWorkspaceInfo(workspaceId)

  return {
    useMockData,
    workspaceId,
    workspaceName: workspaceInfo.name,
    description: workspaceInfo.description
  }
}

export function isMockWorkspace(workspaceId: string): boolean {
  return MOCK_WORKSPACE_IDS.includes(workspaceId)
}

export function isDatabaseWorkspace(workspaceId: string): boolean {
  return !isMockWorkspace(workspaceId)
}

// Workspace information lookup
function getWorkspaceInfo(workspaceId: string) {
  const workspaceMap: Record<string, { name: string; description: string }> = {
    '1': {
      name: 'Product A (Demo)',
      description: 'Demo workspace with mock data - Core SaaS platform development and operations'
    },
    'demo': {
      name: 'Demo Company',
      description: 'Explore Helpin with comprehensive demo data'
    },
    'demo-demo-demo-demo-demolicious': {
      name: 'Demo Company',
      description: 'Explore Helpin with comprehensive demo data'
    },
    '2': {
      name: 'Product B',
      description: 'Live workspace - Mobile app and consumer-facing products'
    },
    '3': {
      name: 'Brand X',
      description: 'Live workspace - Enterprise solutions and B2B integrations'
    }
  }

  return workspaceMap[workspaceId] || {
    name: `Workspace ${workspaceId}`,
    description: 'Custom workspace'
  }
}

// Helper function to log data strategy decisions
export function logDataStrategy(workspaceId: string, dataType: string) {
  const strategy = getDataStrategy(workspaceId)
  console.log(`[${dataType}] Using ${strategy.useMockData ? 'MOCK' : 'DATABASE'} data for workspace: ${strategy.workspaceName}`)
}