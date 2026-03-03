import type { GoalTemplate } from './goalCreationTypes'

export const GOAL_TEMPLATES: GoalTemplate[] = [
  {
    id: 'revenue-growth',
    name: 'Revenue Growth',
    description: 'Increase monthly recurring revenue or total revenue',
    category: 'Revenue Growth',
    default_data: {
      title: 'Increase MRR by 25%',
      description: 'Drive revenue growth through customer acquisition, expansion, and retention',
      priority: 'high',
      metric_type: 'currency',
      success_criteria: 'Achieve sustained MRR growth with healthy unit economics'
    },
    suggested_teams: ['Sales Team', 'Product Team', 'Marketing Team'],
    typical_sprint_breakdown: [10, 15, 20, 20, 20, 15] // Sprint 1-6 percentages
  },
  {
    id: 'customer-acquisition',
    name: 'Customer Acquisition',
    description: 'Acquire new customers or users',
    category: 'Customer Acquisition',
    default_data: {
      title: 'Acquire 500 new customers',
      description: 'Expand customer base through targeted acquisition campaigns',
      priority: 'high',
      metric_type: 'number',
      success_criteria: 'Achieve customer acquisition with sustainable CAC:LTV ratio'
    },
    suggested_teams: ['Sales Team', 'Marketing Team', 'Product Team'],
    typical_sprint_breakdown: [12, 15, 18, 20, 20, 15]
  },
  {
    id: 'performance-improvement',
    name: 'Performance & Reliability',
    description: 'Improve system performance, uptime, or reliability metrics',
    category: 'Performance & Reliability',
    default_data: {
      title: 'Achieve 99.9% uptime',
      description: 'Improve system reliability and performance',
      priority: 'high',
      metric_type: 'percentage',
      success_criteria: 'Maintain consistent uptime with fast incident resolution'
    },
    suggested_teams: ['Platform Engineering', 'DevOps Team', 'QA Team'],
    typical_sprint_breakdown: [15, 20, 20, 20, 15, 10]
  },
  {
    id: 'customer-satisfaction',
    name: 'Customer Satisfaction',
    description: 'Improve NPS, CSAT, or other satisfaction metrics',
    category: 'Customer Satisfaction',
    default_data: {
      title: 'Achieve NPS score of 50+',
      description: 'Improve customer satisfaction through better experience and support',
      priority: 'medium',
      metric_type: 'number',
      success_criteria: 'Sustained high satisfaction with improved retention'
    },
    suggested_teams: ['Product Team', 'Customer Success', 'Support Team'],
    typical_sprint_breakdown: [10, 15, 20, 25, 20, 10]
  },
  {
    id: 'product-launch',
    name: 'Product Launch',
    description: 'Successfully launch a new product or major feature',
    category: 'Product Development',
    default_data: {
      title: 'Launch new product feature',
      description: 'Deliver and launch new product capability',
      priority: 'high',
      metric_type: 'percentage',
      target_value: 100,
      success_criteria: 'Successful launch with positive user adoption'
    },
    suggested_teams: ['Product Team', 'Engineering Team', 'Design Team'],
    typical_sprint_breakdown: [5, 15, 25, 25, 20, 10]
  },
  {
    id: 'market-expansion',
    name: 'Market Expansion',
    description: 'Enter new markets or expand geographic reach',
    category: 'Market Expansion',
    default_data: {
      title: 'Expand to 3 new markets',
      description: 'Strategic expansion into new geographic or demographic markets',
      priority: 'medium',
      metric_type: 'number',
      success_criteria: 'Successful market entry with initial traction'
    },
    suggested_teams: ['Sales Team', 'Marketing Team', 'Business Development'],
    typical_sprint_breakdown: [8, 12, 20, 25, 25, 10]
  }
]

export const getTemplateByCategory = (category: string): GoalTemplate[] => {
  return GOAL_TEMPLATES.filter(template => template.category === category)
}

export const getTeamSuggestions = (workspaceName: string): string[] => {
  // Return team names based on workspace - using our existing team data
  if (workspaceName === 'Product A') {
    return ['Platform Engineering', 'Product Design', 'Data Analytics']
  } else if (workspaceName === 'Product B') {
    return ['Mobile Development', 'Consumer Marketing']
  } else if (workspaceName === 'Brand X') {
    return ['Enterprise Solutions', 'Client Success']
  }
  return []
}