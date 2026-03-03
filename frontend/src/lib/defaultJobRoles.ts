// Comprehensive default job roles and scoring criteria for tech companies
import type { JobRoleScoringCriteria } from './workspaceSettings'

export const DEFAULT_TECH_JOB_ROLES: JobRoleScoringCriteria[] = [
  // ENGINEERING ROLES
  {
    job_role: 'Frontend Developer',
    criteria: [
      {
        id: 'fe-code-quality',
        name: 'Code Quality',
        description: 'Code is clean, maintainable, and follows best practices',
        question: 'Code quality meets standards?',
        enabled: true,
        weight: 1
      },
      {
        id: 'fe-ui-implementation',
        name: 'UI Implementation',
        description: 'UI matches design specifications and is pixel-perfect',
        question: 'UI implementation matches designs?',
        enabled: true,
        weight: 1
      },
      {
        id: 'fe-responsive-design',
        name: 'Responsive Design',
        description: 'Implementation works across all target devices and screen sizes',
        question: 'Responsive design implemented correctly?',
        enabled: true,
        weight: 1
      },
      {
        id: 'fe-performance',
        name: 'Performance Optimization',
        description: 'Frontend performance is optimized (bundle size, loading times)',
        question: 'Performance optimization achieved?',
        enabled: true,
        weight: 1
      },
      {
        id: 'fe-accessibility',
        name: 'Accessibility',
        description: 'Implementation follows WCAG guidelines and accessibility best practices',
        question: 'Accessibility requirements met?',
        enabled: true,
        weight: 1
      }
    ]
  },
  {
    job_role: 'Backend Developer',
    criteria: [
      {
        id: 'be-api-design',
        name: 'API Design',
        description: 'APIs are well-designed, RESTful, and properly documented',
        question: 'API design meets standards?',
        enabled: true,
        weight: 1
      },
      {
        id: 'be-code-quality',
        name: 'Code Quality',
        description: 'Code is clean, testable, and follows SOLID principles',
        question: 'Code quality meets standards?',
        enabled: true,
        weight: 1
      },
      {
        id: 'be-performance',
        name: 'Performance & Scalability',
        description: 'Code is optimized for performance and can handle scale',
        question: 'Performance requirements met?',
        enabled: true,
        weight: 1
      },
      {
        id: 'be-security',
        name: 'Security Implementation',
        description: 'Security best practices are followed (auth, validation, sanitization)',
        question: 'Security requirements implemented?',
        enabled: true,
        weight: 1
      },
      {
        id: 'be-testing',
        name: 'Testing Coverage',
        description: 'Adequate unit and integration tests are written',
        question: 'Testing coverage is adequate?',
        enabled: true,
        weight: 1
      }
    ]
  },
  {
    job_role: 'Full Stack Developer',
    criteria: [
      {
        id: 'fs-frontend-skills',
        name: 'Frontend Implementation',
        description: 'Frontend features are implemented with quality and attention to UX',
        question: 'Frontend implementation meets standards?',
        enabled: true,
        weight: 1
      },
      {
        id: 'fs-backend-skills',
        name: 'Backend Implementation',
        description: 'Backend features are robust, secure, and performant',
        question: 'Backend implementation meets standards?',
        enabled: true,
        weight: 1
      },
      {
        id: 'fs-integration',
        name: 'System Integration',
        description: 'Frontend and backend integrate seamlessly with proper error handling',
        question: 'System integration is seamless?',
        enabled: true,
        weight: 1
      },
      {
        id: 'fs-problem-solving',
        name: 'End-to-End Problem Solving',
        description: 'Can solve problems across the entire stack independently',
        question: 'Demonstrates full-stack problem solving?',
        enabled: true,
        weight: 1
      }
    ]
  },
  {
    job_role: 'DevOps Engineer',
    criteria: [
      {
        id: 'devops-automation',
        name: 'Automation & CI/CD',
        description: 'Implements effective automation and CI/CD pipelines',
        question: 'Automation and CI/CD implemented effectively?',
        enabled: true,
        weight: 1
      },
      {
        id: 'devops-infrastructure',
        name: 'Infrastructure Management',
        description: 'Infrastructure is reliable, scalable, and cost-effective',
        question: 'Infrastructure management meets standards?',
        enabled: true,
        weight: 1
      },
      {
        id: 'devops-monitoring',
        name: 'Monitoring & Alerting',
        description: 'Comprehensive monitoring and alerting systems are in place',
        question: 'Monitoring and alerting implemented?',
        enabled: true,
        weight: 1
      },
      {
        id: 'devops-security',
        name: 'Security & Compliance',
        description: 'Security best practices and compliance requirements are met',
        question: 'Security and compliance requirements met?',
        enabled: true,
        weight: 1
      },
      {
        id: 'devops-incident-response',
        name: 'Incident Response',
        description: 'Responds effectively to incidents and implements preventive measures',
        question: 'Incident response was effective?',
        enabled: true,
        weight: 1
      }
    ]
  },
  {
    job_role: 'QA Engineer',
    criteria: [
      {
        id: 'qa-test-planning',
        name: 'Test Planning & Strategy',
        description: 'Comprehensive test plans and strategies are developed',
        question: 'Test planning meets requirements?',
        enabled: true,
        weight: 1
      },
      {
        id: 'qa-test-execution',
        name: 'Test Execution',
        description: 'Tests are executed thoroughly and systematically',
        question: 'Test execution was comprehensive?',
        enabled: true,
        weight: 1
      },
      {
        id: 'qa-bug-detection',
        name: 'Bug Detection & Reporting',
        description: 'Critical bugs are identified and reported with clear reproduction steps',
        question: 'Bug detection and reporting effective?',
        enabled: true,
        weight: 1
      },
      {
        id: 'qa-automation',
        name: 'Test Automation',
        description: 'Automated tests are created and maintained effectively',
        question: 'Test automation implemented effectively?',
        enabled: true,
        weight: 1
      },
      {
        id: 'qa-quality-advocacy',
        name: 'Quality Advocacy',
        description: 'Advocates for quality throughout the development process',
        question: 'Demonstrated quality advocacy?',
        enabled: true,
        weight: 1
      }
    ]
  },
  {
    job_role: 'Mobile Developer (iOS)',
    criteria: [
      {
        id: 'ios-app-quality',
        name: 'App Quality',
        description: 'App follows iOS Human Interface Guidelines and performs well',
        question: 'App quality meets iOS standards?',
        enabled: true,
        weight: 1
      },
      {
        id: 'ios-code-quality',
        name: 'Code Quality',
        description: 'Swift/Objective-C code is clean and follows iOS best practices',
        question: 'iOS code quality meets standards?',
        enabled: true,
        weight: 1
      },
      {
        id: 'ios-performance',
        name: 'Performance Optimization',
        description: 'App is optimized for performance, memory usage, and battery life',
        question: 'iOS performance optimization achieved?',
        enabled: true,
        weight: 1
      },
      {
        id: 'ios-app-store',
        name: 'App Store Compliance',
        description: 'App meets App Store guidelines and submission requirements',
        question: 'App Store compliance requirements met?',
        enabled: true,
        weight: 1
      }
    ]
  },
  {
    job_role: 'Mobile Developer (Android)',
    criteria: [
      {
        id: 'android-app-quality',
        name: 'App Quality',
        description: 'App follows Material Design guidelines and Android best practices',
        question: 'App quality meets Android standards?',
        enabled: true,
        weight: 1
      },
      {
        id: 'android-code-quality',
        name: 'Code Quality',
        description: 'Kotlin/Java code is clean and follows Android development best practices',
        question: 'Android code quality meets standards?',
        enabled: true,
        weight: 1
      },
      {
        id: 'android-performance',
        name: 'Performance Optimization',
        description: 'App is optimized for various Android devices and API levels',
        question: 'Android performance optimization achieved?',
        enabled: true,
        weight: 1
      },
      {
        id: 'android-play-store',
        name: 'Play Store Compliance',
        description: 'App meets Google Play Store policies and requirements',
        question: 'Play Store compliance requirements met?',
        enabled: true,
        weight: 1
      }
    ]
  },

  // DESIGN ROLES
  {
    job_role: 'UX Designer',
    criteria: [
      {
        id: 'ux-user-research',
        name: 'User Research',
        description: 'Conducts thorough user research to inform design decisions',
        question: 'User research conducted effectively?',
        enabled: true,
        weight: 1
      },
      {
        id: 'ux-design-thinking',
        name: 'Design Thinking Process',
        description: 'Applies design thinking methodology to solve user problems',
        question: 'Design thinking process applied?',
        enabled: true,
        weight: 1
      },
      {
        id: 'ux-prototyping',
        name: 'Prototyping & Testing',
        description: 'Creates prototypes and conducts usability testing',
        question: 'Prototyping and testing completed?',
        enabled: true,
        weight: 1
      },
      {
        id: 'ux-collaboration',
        name: 'Cross-functional Collaboration',
        description: 'Collaborates effectively with product, engineering, and stakeholders',
        question: 'Cross-functional collaboration effective?',
        enabled: true,
        weight: 1
      },
      {
        id: 'ux-impact',
        name: 'User Experience Impact',
        description: 'Designs demonstrably improve user experience and metrics',
        question: 'UX improvements achieved measurable impact?',
        enabled: true,
        weight: 1
      }
    ]
  },
  {
    job_role: 'Product Designer',
    criteria: [
      {
        id: 'pd-design-quality',
        name: 'Design Quality',
        description: 'Designs are visually excellent and solve user problems effectively',
        question: 'Design quality meets high standards?',
        enabled: true,
        weight: 1
      },
      {
        id: 'pd-system-thinking',
        name: 'Design System Thinking',
        description: 'Contributes to and maintains design system consistency',
        question: 'Design system thinking demonstrated?',
        enabled: true,
        weight: 1
      },
      {
        id: 'pd-product-strategy',
        name: 'Product Strategy Alignment',
        description: 'Designs align with and support overall product strategy',
        question: 'Designs support product strategy?',
        enabled: true,
        weight: 1
      },
      {
        id: 'pd-iteration',
        name: 'Design Iteration',
        description: 'Iterates on designs based on feedback and data',
        question: 'Design iteration based on feedback?',
        enabled: true,
        weight: 1
      }
    ]
  },

  // PRODUCT & MANAGEMENT ROLES
  {
    job_role: 'Product Manager',
    criteria: [
      {
        id: 'pm-strategy',
        name: 'Product Strategy',
        description: 'Develops and executes clear product strategy aligned with business goals',
        question: 'Product strategy clearly defined and executed?',
        enabled: true,
        weight: 1
      },
      {
        id: 'pm-roadmap',
        name: 'Roadmap Management',
        description: 'Maintains and communicates product roadmap effectively',
        question: 'Product roadmap managed effectively?',
        enabled: true,
        weight: 1
      },
      {
        id: 'pm-stakeholder',
        name: 'Stakeholder Management',
        description: 'Manages stakeholder expectations and communication effectively',
        question: 'Stakeholder management effective?',
        enabled: true,
        weight: 1
      },
      {
        id: 'pm-data-driven',
        name: 'Data-Driven Decisions',
        description: 'Makes product decisions based on data and user feedback',
        question: 'Decisions supported by data and research?',
        enabled: true,
        weight: 1
      },
      {
        id: 'pm-delivery',
        name: 'Feature Delivery',
        description: 'Successfully delivers features on time and within scope',
        question: 'Feature delivery met expectations?',
        enabled: true,
        weight: 1
      }
    ]
  },
  {
    job_role: 'Engineering Manager',
    criteria: [
      {
        id: 'em-team-performance',
        name: 'Team Performance',
        description: 'Team consistently meets sprint goals and maintains high productivity',
        question: 'Team performance meets expectations?',
        enabled: true,
        weight: 1
      },
      {
        id: 'em-technical-leadership',
        name: 'Technical Leadership',
        description: 'Provides effective technical guidance and architectural decisions',
        question: 'Technical leadership provided effectively?',
        enabled: true,
        weight: 1
      },
      {
        id: 'em-people-management',
        name: 'People Management',
        description: 'Supports team member growth and maintains team morale',
        question: 'People management effective?',
        enabled: true,
        weight: 1
      },
      {
        id: 'em-process-improvement',
        name: 'Process Improvement',
        description: 'Continuously improves team processes and development practices',
        question: 'Process improvements implemented?',
        enabled: true,
        weight: 1
      },
      {
        id: 'em-cross-team',
        name: 'Cross-team Collaboration',
        description: 'Facilitates effective collaboration with other teams and stakeholders',
        question: 'Cross-team collaboration facilitated?',
        enabled: true,
        weight: 1
      }
    ]
  },

  // DATA ROLES
  {
    job_role: 'Data Scientist',
    criteria: [
      {
        id: 'ds-analysis',
        name: 'Data Analysis',
        description: 'Conducts thorough and accurate data analysis',
        question: 'Data analysis meets quality standards?',
        enabled: true,
        weight: 1
      },
      {
        id: 'ds-modeling',
        name: 'Model Development',
        description: 'Develops effective machine learning models and algorithms',
        question: 'Model development meets requirements?',
        enabled: true,
        weight: 1
      },
      {
        id: 'ds-insights',
        name: 'Business Insights',
        description: 'Translates data findings into actionable business insights',
        question: 'Business insights provided effectively?',
        enabled: true,
        weight: 1
      },
      {
        id: 'ds-communication',
        name: 'Data Communication',
        description: 'Communicates complex data findings clearly to stakeholders',
        question: 'Data communication effective?',
        enabled: true,
        weight: 1
      }
    ]
  },

  // CONTENT & MARKETING ROLES
  {
    job_role: 'Technical Writer',
    criteria: [
      {
        id: 'tw-clarity',
        name: 'Documentation Clarity',
        description: 'Documentation is clear, comprehensive, and easy to understand',
        question: 'Documentation clarity meets standards?',
        enabled: true,
        weight: 1
      },
      {
        id: 'tw-accuracy',
        name: 'Technical Accuracy',
        description: 'Technical information is accurate and up-to-date',
        question: 'Technical accuracy maintained?',
        enabled: true,
        weight: 1
      },
      {
        id: 'tw-user-focus',
        name: 'User-Focused Content',
        description: 'Content is written from the user perspective and addresses their needs',
        question: 'Content meets user needs effectively?',
        enabled: true,
        weight: 1
      },
      {
        id: 'tw-collaboration',
        name: 'SME Collaboration',
        description: 'Collaborates effectively with subject matter experts',
        question: 'SME collaboration effective?',
        enabled: true,
        weight: 1
      }
    ]
  },

  // LEADERSHIP ROLES
  {
    job_role: 'CTO',
    criteria: [
      {
        id: 'cto-strategy',
        name: 'Technical Strategy',
        description: 'Develops and executes technical strategy aligned with business objectives',
        question: 'Technical strategy alignment achieved?',
        enabled: true,
        weight: 1
      },
      {
        id: 'cto-leadership',
        name: 'Technical Leadership',
        description: 'Provides effective leadership across all technical teams',
        question: 'Technical leadership effective?',
        enabled: true,
        weight: 1
      },
      {
        id: 'cto-innovation',
        name: 'Innovation & Technology',
        description: 'Drives innovation and evaluates new technologies for adoption',
        question: 'Innovation and technology advancement achieved?',
        enabled: true,
        weight: 1
      },
      {
        id: 'cto-scaling',
        name: 'Scaling & Architecture',
        description: 'Ensures technical architecture supports business scaling',
        question: 'Scaling and architecture requirements met?',
        enabled: true,
        weight: 1
      },
      {
        id: 'cto-culture',
        name: 'Engineering Culture',
        description: 'Builds and maintains strong engineering culture and practices',
        question: 'Engineering culture development effective?',
        enabled: true,
        weight: 1
      }
    ]
  }
]

// Helper function to get default criteria for a specific job role
export const getDefaultCriteriaForJobRole = (jobRole: string) => {
  const roleConfig = DEFAULT_TECH_JOB_ROLES.find(config => config.job_role === jobRole)
  return roleConfig?.criteria || []
}

// Get all available job roles
export const getAllDefaultJobRoles = () => {
  return DEFAULT_TECH_JOB_ROLES.map(config => config.job_role)
}