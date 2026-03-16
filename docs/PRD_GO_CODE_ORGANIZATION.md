# PRD: Go Code Organization & Refactoring

## Executive Summary

This document provides a comprehensive analysis and refactoring plan for the Helpin Go backend codebase. The application has grown to **309 Go source files** with significant code organization challenges that impact maintainability, developer velocity, and system reliability.

---

## 1. Current State Analysis

### 1.1 Codebase Scale

| Metric | Count |
|--------|-------|
| Total Go source files (non-test) | 309 |
| Total Go test files | ~100+ |
| GORM Models | 60+ |
| Repositories | 40+ |
| Handlers | 70+ |
| Services | 50+ |
| Lines of Go code (non-test) | ~61,000 |

### 1.2 Top Files by Lines of Code (LOC)

| Rank | LOC | File | Package | Issues |
|------|-----|------|---------|--------|
| 1 | 2191 | `temporalapp/activities.go` | temporalapp | 30 methods, god object |
| 2 | 1763 | `service/pm_import.go` | service | 22 methods, feature overload |
| 3 | 1369 | `service/notification.go` | service | 19 methods, complex rendering |
| 4 | 1323 | `repository/settings.go` | repository | 51 methods, Swiss army knife |
| 5 | 1257 | `service/agent_planning.go` | service | Agent planning logic |
| 6 | 1178 | `worker/opencode.go` | worker | 50+ functions, execution engine |
| 7 | 1143 | `repository/pm_story.go` | repository | Story queries |
| 8 | 1094 | `service/agent.go` | service | 13+ dependencies |
| 9 | 1041 | `service/pm_story.go` | service | Story business logic |
| 10 | 960 | `handler/docs.go` | handler | 40+ endpoints |

### 1.3 Structural Issues Identified

#### Issue 1: God Objects with Excessive Dependencies

**Example: `AgentRunActivities` in `temporalapp/activities.go`**

```go
type AgentRunActivities struct {
    runRepo         *repository.AgentRunRepository
    agentRepo       *repository.AgentRepository
    artifactRepo    *repository.AgentRunArtifactRepository
    storyRepo       *repository.PMStoryRepository
    storyLinkRepo   *repository.PMStoryLinkRepository
    epicRepo        *repository.PMEpicRepository
    conversationRepo *repository.SupportConversationRepository
    commentRepo      *repository.PMCommentRepository
    checklistRepo    *repository.PMChecklistItemRepository
    messageRepo      *repository.SupportMessageRepository
    gitIntRepo      *repository.GitIntegrationRepository
    gitRepo         *repository.GitRepositoryRepository
    gitLinkRepo     *repository.StoryGitLinkRepository
    deliveryRepo    *repository.StoryDeliveryTargetRepository
    settingsRepo    *repository.SettingsRepository
    docsSpaceRepo   *repository.DocsSpaceRepository
    docsDocRepo     *repository.DocsDocumentRepository
    docsContentRepo *repository.DocsContentRepository
    docsVersionRepo *repository.DocsVersionRepository
    docsLinkRepo    *repository.DocsLinkRepository
    runtimes        *workerpkg.RuntimeRegistry
    githubApp       *githubapp.Client
}
```

**Problem**: 21 dependencies in a single struct violates Single Responsibility Principle.

#### Issue 2: Monolithic Activity Files

The `temporalapp/activities.go` file contains 30 methods covering:
- Agent run execution
- Planning workflows  
- Git operations (checkout, push, PR)
- Docs management
- Story delivery preparation
- Artifact management
- Token minting
- Run state management

Each of these should be a separate domain.

#### Issue 3: Repository Method Explosion

**`repository/settings.go`** - 51 methods including:
- Team management (CRUD, memberships)
- Person management
- Workspace membership
- Bonus tiers
- Job roles
- Team estimates
- Field visibility
- Invitation preassignments

This should be split into: `team_repo.go`, `person_repo.go`, `bonus_repo.go`, etc.

#### Issue 4: Service Layer Fatals

**`service/agent.go`** - 13+ repository dependencies
**`service/notification.go`** - Complex rendering logic with 19 methods handling:
- Event emission
- Delivery planning
- Immediate email rendering
- Digest processing
- Preference filtering
- Email template rendering (multiple formats)
- Cleanup
- CRUD operations

#### Issue 5: Handler Method Count

| Handler | Methods | File Size |
|---------|---------|-----------|
| `DocsHandler` | 40+ | 960 LOC |
| `AgentHandler` | 25+ | Large |
| `PMStoryHandler` | 30+ | Large |
| `CRM*Handler` (multiple) | 15-25 each | Large |

### 1.4 Import Dependencies Analysis

The `main.go` wires:
- 40+ repositories
- 30+ services
- 25+ handlers
- Multiple infrastructure clients (S3, Temporal, JWT, WebSocket)

This creates a **monolithic dependency graph** where:
- Changes to one service require understanding many others
- Testing requires mocking 20+ dependencies
- New developers struggle to find related code

---

## 2. Architectural Problems

### 2.1 Circular Dependency Risk

As the codebase grows, certain patterns create circular dependency risks:
- Services calling other services directly
- No clear boundaries between domains

### 2.2 Testing Difficulty

With god objects having 20+ dependencies:
- Unit tests require extensive mocking
- Integration tests become necessary too early
- Test coverage metrics are misleading

### 2.3 Code Navigation

Finding related code is difficult:
- "Where is the team deletion logic?" → Search across multiple files
- "Where is the email rendering?" → `notification.go` + `email.go` + templates
- "How do I add a new agent activity?" → Navigate 2191-line file

### 2.4 Onboarding Impact

New developers report:
- Difficulty understanding data flow
- Fear of making changes to large files
- Long code review cycles

---

## 3. Target Architecture

### 3.1 Package Structure

```
internal/
├── agent/                          # NEW: Agent domain
│   ├── service.go                  # Agent orchestration
│   ├── planning.go                 # Planning logic
│   ├── execution.go                # Execution logic
│   ├── artifacts.go                # Artifact management
│   └── types.go                    # Domain types
├── temporal/
│   ├── engine.go                   # Workflow engine setup
│   ├── activities/
│   │   ├── base.go                 # Common structs, interfaces
│   │   ├── agent_run.go           # Agent execution activities (~400 LOC)
│   │   ├── planning.go             # Planning activities (~300 LOC)
│   │   ├── docs.go                 # Docs activities (~200 LOC)
│   │   └── git.go                  # Git operations (~200 LOC)
│   ├── workflows/
│   │   ├── agent_run.go
│   │   ├── planning.go
│   │   └── docs.go
│   └── adapters/
│       ├── opencode.go
│       └── anthropic.go
├── notification/                   # NEW: Notification domain
│   ├── service.go                  # Main orchestration
│   ├── emit.go                     # Event emission
│   ├── delivery/                   # Delivery planning
│   │   ├── planner.go
│   │   └── executor.go
│   ├── email/
│   │   ├── client.go               # Email sending
│   │   ├── immediate.go            # Immediate email templates
│   │   └── digest.go              # Digest email templates
│   ├── inapp/                     # In-app notifications
│   │   ├── store.go
│   │   └── renderer.go
│   └── preferences/                # User preferences
│       ├── store.go
│       └── processor.go
├── pm/                             # NEW: Project Management domain
│   ├── story/
│   │   ├── service.go
│   │   ├── commands.go
│   │   └── queries.go
│   ├── epic/
│   ├── sprint/
│   ├── objective/
│   └── import/
│       ├── service.go
│       ├── shortcut/
│       │   ├── parser.go
│       │   ├── mapper.go
│       │   └── validator.go
│       └── csv/
├── crm/                            # NEW: CRM domain
│   ├── contact/
│   ├── deal/
│   ├── email/
│   └── intelligence/
├── docs/                           # NEW: Documentation domain
│   ├── service.go
│   ├── space.go
│   ├── document.go
│   └── helpcenter.go
├── settings/                       # NEW: Settings domain
│   ├── team/
│   │   ├── repo.go
│   │   └── service.go
│   ├── person/
│   ├── bonus/
│   └── estimates/
├── repository/                     # Existing, refactor large repos
│   ├── settings/                   # Split into sub-repos
│   │   ├── team.go
│   │   ├── person.go
│   │   ├── bonus.go
│   │   └── estimates.go
│   └── ...
├── model/                          # Keep, but organize
│   ├── agent.go
│   ├── pm/
│   │   ├── story.go
│   │   ├── epic.go
│   │   └── ...
│   └── crm/
├── handler/                        # Keep structure, split large handlers
│   ├── docs.go                     # Split: doc_space.go, doc_document.go
│   └── ...
└── worker/
    ├── tools/                      # Split tools by domain
    │   ├── story.go
    │   ├── support.go
    │   └── git.go
    └── runtimes/
```

### 3.2 File Size Guidelines

| Type | Target LOC | Max LOC | Rationale |
|------|------------|---------|-----------|
| Handlers | 150-250 | 350 | HTTP concerns are narrow |
| Services | 200-350 | 450 | Business logic, focused domain |
| Repositories | 250-400 | 500 | Data access patterns |
| Models | 150-300 | 400 | Definitions only |
| Activities | 300-400 | 500 | Temporal activities |
| Workflows | 200-300 | 400 | Workflow definitions |
| Workers | 300-500 | 600 | Execution engines |
| Utils/Helpers | 100-200 | 300 | Pure functions |

### 3.3 Dependency Injection Guidelines

**Current (Anti-pattern)**:
```go
type AgentService struct {
    agentRepo *repository.AgentRepository
    runRepo *repository.AgentRunRepository
    // ... 15 more
}
```

**Target (Composition)**:
```go
type AgentService struct {
    runAgent *RunAgentService  // Sub-domain service
    planner  *PlannerService
}

type RunAgentService struct {
    runRepo     *repository.AgentRunRepository
    agentRepo   *repository.AgentRepository
    // Only what it needs
}

type PlannerService struct {
    storyRepo *repository.PMStoryRepository
    epicRepo  *repository.PMEpicRepository
}
```

---

## 4. Refactoring Phases

### Phase 1: Domain Extraction (Weeks 1-3)

#### 1.1 Extract Notification Domain

**From**: `service/notification.go` (1369 LOC, 19 methods)

**To**: `internal/notification/` package

```
notification/
├── service.go              # Orchestration
├── emit.go                 # Event emission logic
├── delivery/
│   ├── planner.go         # Delivery planning
│   └── executor.go        # Delivery execution
├── email/
│   ├── client.go          # Postmark wrapper
│   ├── immediate.go       # Immediate notifications
│   └── digest.go          # Digest notifications
└── preferences/
    ├── store.go
    └── processor.go
```

**Effort**: 1 week
**Risk**: Low (isolated domain)

#### 1.2 Extract PM Import Domain

**From**: `service/pm_import.go` (1763 LOC, 22 methods) + `shortcut_*.go`

**To**: `internal/pm/import/` package

```
pm/import/
├── service.go              # Orchestration
├── shortcut/
│   ├── client.go           # Shortcut API client
│   ├── parser.go           # CSV parsing
│   ├── mapper.go           # Field mapping
│   └── validator.go        # Validation
├── csv/
│   ├── reader.go
│   └── transformer.go
└── workers/
    └── media.go           # Media import workers
```

**Effort**: 1 week
**Risk**: Low (isolated feature)

#### 1.3 Extract Agent Domain

**From**: `service/agent.go` (1094 LOC) + `service/agent_planning.go` (1257 LOC)

**To**: `internal/agent/` package

```
agent/
├── service.go              # Main orchestration
├── planning.go             # Planning logic
├── execution.go            # Execution logic
├── artifacts.go            # Artifact management
└── types.go                # Domain types
```

**Effort**: 1 week
**Risk**: Medium (used by handlers and Temporal)

### Phase 2: Temporal Activities Refactoring (Weeks 4-6)

#### 2.1 Split `temporalapp/activities.go`

**Current**: 2191 LOC, 30 methods, 21 dependencies

**Target**: `internal/temporal/activities/` package

```
temporal/activities/
├── base.go                 # Common structs, interfaces
│   - planningRunInput
│   - planningRunSummary
│   - resolvedRunState
├── agent_run.go            # ~400 LOC
│   - PrepareRunActivity
│   - ExecuteRunActivity
│   - loadRunState
│   - prepareStoryDelivery
├── planning.go             # ~300 LOC
│   - resolvePlanningRunInput
│   - buildInitialInstructions
│   - finalizePlanningRun
├── git.go                  # ~200 LOC
│   - checkoutRunRef
│   - ensureRemoteBranch
│   - recordPush
│   - recordPR
├── docs.go                 # ~200 LOC
│   - ensureEpicSpecDocument
│   - renderLinkedDocsContext
└── artifacts.go            # ~150 LOC
    - createRunArtifact
```

**Key**: Use composition for sub-activity structs:

```go
// Instead of one giant struct
type AgentRunActivities struct { 21 dependencies }

// Use focused sub-services
type AgentRunActivities struct {
    agentRunner *AgentRunner
    planner     *ActivityPlanner
    gitOps      *GitOperations
    docsManager *DocsManager
}

type AgentRunner struct {
    runRepo   *repository.AgentRunRepository
    agentRepo *repository.AgentRepository
}

type GitOperations struct {
    gitIntRepo *repository.GitIntegrationRepository
    gitRepo    *repository.GitRepositoryRepository
    githubApp  *githubapp.Client
}
```

**Effort**: 2 weeks
**Risk**: High (core to agent execution)

### Phase 3: Repository Optimization (Weeks 7-8)

#### 3.1 Split `repository/settings.go`

**Current**: 1323 LOC, 51 methods

**To**: `internal/repository/settings/` package

```
repository/settings/
├── repo.go                 # Main repo (fewer methods)
├── team.go                 # Team CRUD + memberships
├── person.go               # Person management
├── bonus.go                # Bonus tier operations
├── estimates.go            # Team estimates
└── preferences.go          # User preferences
```

**Effort**: 1 week
**Risk**: Medium (used widely)

#### 3.2 Split `repository/pm_story.go`

**Current**: 1143 LOC

**To**: Keep as single file but extract query builders

```
repository/pm_story/
├── repo.go                 # Main repo
├── queries.go              # Complex query builders
└── mutations.go            # Write operations
```

**Effort**: 1 week
**Risk**: Medium

### Phase 4: Handler Organization (Weeks 9-10)

#### 4.1 Split Large Handlers

| Handler | Current LOC | Split Into |
|---------|-------------|------------|
| `docs.go` | 960 | `docs_space.go`, `docs_document.go`, `docs_helpcenter.go` |
| `agent.go` | Large | Keep, but refactor service layer |
| `crm_*.go` | Multiple | Each is ~300 LOC, acceptable |

#### 4.2 Add File Size Linter Rules

```yaml
# .golangci.yml
linters-settings:
  funlen:
    lines: 120
    statements: 40
  gocognit:
    min-complexity: 20
  lll:
    max-line-length: 120
```

---

## 5. Migration Strategy

### 5.1 Backward Compatibility

**Step 1**: Create new package structure
```
internal/
├── notification/           # NEW
│   └── ...
└── notification/
    └── notification.go     # OLD - re-exports NEW
```

**Step 2**: Update imports incrementally
- Change `service.NotificationService` → `notification.Service`
- Keep old imports working during transition

**Step 3**: Remove old re-exports after full migration

### 5.2 Testing Strategy

1. **Before split**: Ensure test coverage > 70%
2. **During split**: Move tests with code
3. **After split**: Add integration tests for boundaries
4. **CI**: Run full test suite after each PR

### 5.3 Rollback Plan

- Keep git history intact
- Use feature flags for gradual rollout
- Monitor error rates during migration

---

## 6. Code Organization Standards

### 6.1 Package Design Principles

1. **Single Responsibility**: Each package has one clear purpose
2. **Low Coupling**: Packages communicate via interfaces
3. **High Cohesion**: Related code lives together
4. **No Import Cycles**: Strict DAG structure
5. **Max Dependencies**: Package should not depend on >10 other packages

### 6.2 File Naming Conventions

| Type | Convention | Example |
|------|------------|---------|
| Main file | `domain.go` | `agent.go`, `settings.go` |
| Sub-domain | `domain_feature.go` | `agent_planning.go` |
| Commands | `domain_commands.go` | `story_commands.go` |
| Queries | `domain_queries.go` | `story_queries.go` |
| Helpers | `domain_utils.go` | `string_utils.go` |

### 6.3 Struct Organization

```go
// 1. Type definitions
type UserService struct{}

// 2. Constructor
func NewUserService(repo *UserRepository) *UserService {}

// 3. Public methods (grouped by usage)
// - CRUD operations
// - Business logic
// - Queries

// 4. Private methods (grouped by dependency)
```

### 6.4 Import Organization

```go
import (
    // Standard library
    "context"
    "time"
    
    // External packages
    "gorm.io/gorm"
    "github.com/go-chi/chi/v5"
    
    // Internal packages (grouped by layer)
    "github.com/helpin-ai/helpin/server/internal/model"
    "github.com/helpin-ai/helpin/server/internal/repository"
)
```

---

## 7. Implementation Roadmap

### Phase 1: Foundation (Weeks 1-3)

| Week | Task | Owner | Deliverable |
|------|------|-------|--------------|
| 1 | Extract notification domain | TBD | `internal/notification/` package |
| 2 | Extract PM import domain | TBD | `internal/pm/import/` package |
| 3 | Extract agent domain | TBD | `internal/agent/` package |

### Phase 2: Core Refactoring (Weeks 4-6)

| Week | Task | Owner | Deliverable |
|------|------|-------|--------------|
| 4 | Split temporal activities | TBD | `internal/temporal/activities/` |
| 5 | Refactor agent run activities | TBD | Composition pattern applied |
| 6 | Split git operations | TBD | `GitOperations` struct |

### Phase 3: Data Layer (Weeks 7-8)

| Week | Task | Owner | Deliverable |
|------|------|-------|--------------|
| 7 | Split settings repository | TBD | `internal/repository/settings/` |
| 8 | Optimize pm_story repository | TBD | Query builders extracted |

### Phase 4: Handlers & Polish (Weeks 9-10)

| Week | Task | Owner | Deliverable |
|------|------|-------|--------------|
| 9 | Split docs handler | TBD | Sub-handlers created |
| 10 | Add linter rules, cleanup | TBD | Code standards enforced |

### Phase 5: Validation (Week 11)

| Week | Task | Owner | Deliverable |
|------|------|-------|--------------|
| 11 | Full testing, bug fixes | TBD | Production ready |

---

## 8. Success Metrics

### 8.1 Code Quality Metrics

| Metric | Current | Phase 1 | Phase 2 | Target |
|--------|---------|---------|---------|--------|
| Max file LOC | 2191 | 1800 | 1200 | < 600 |
| Avg file LOC | ~200 | 200 | 200 | 200-300 |
| Files > 500 LOC | 10+ | 8 | 4 | 0 |
| Max struct deps | 21 | 15 | 10 | < 8 |
| Methods per struct | 30+ | 25 | 15 | < 10 |

### 8.2 Developer Experience

| Metric | Current | Target |
|--------|---------|--------|
| Time to find related code | 10+ min | < 2 min |
| Avg PR review time | Long | < 1 day |
| Onboarding time | 2+ weeks | 1 week |
| Test coverage | ~60% | > 75% |

### 8.3 System Reliability

| Metric | Current | Target |
|--------|---------|--------|
| Build time | Slow | < 2 min |
| Linter issues | Few | Zero |
| Import cycles | None | None |

---

## 9. Risks & Mitigation

### 9.1 Technical Risks

| Risk | Impact | Mitigation |
|------|--------|------------|
| Breaking changes | High | Incremental migration, backward compat |
| Import cycles | Medium | Use interfaces, careful planning |
| Test failures | Medium | Comprehensive test suite |
| Performance regression | Medium | Benchmark before/after |

### 9.2 Organizational Risks

| Risk | Impact | Mitigation |
|------|--------|------------|
| Scope creep | High | Strict phase gates |
| Resource constraints | Medium | Prioritize high-impact splits |
| Developer resistance | Medium | Clear communication, training |

---

## 10. Appendix: Industry Standards

### 10.1 Ideal LOC Guidelines

| Source | Recommendation |
|--------|---------------|
| Google Go Style | "Files should not exceed a few hundred lines" |
| Uber Go Style | "Limit file length to a few hundred lines" |
| Go Code Review Comments | "Keep files small and focused" |
| Practical Experience | 200-300 LOC average |

### 10.2 When to Split

Split a file when:
1. It exceeds 400 LOC
2. It has >10 public functions
3. It handles >3 distinct concepts
4. It imports >10 packages
5. New developers struggle to find related code
6. Testing requires >10 mocks

### 10.3 When NOT to Split

Don't split if:
1. Code is genuinely cohesive
2. Splitting creates import cycles
3. Files become trivial wrappers (< 50 LOC)
4. You break existing test organization

---

## 11. Related Documentation

- `AGENTS.md` - Architecture overview
- `docs/AGENTS_AND_AUTOMATION.md` - Agent system details
- `docs/CRM_MODULE.md` - CRM architecture

---

*Document Version: 2.0*  
*Created: March 2026*  
*Status: Draft for Review*
