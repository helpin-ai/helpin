# Go Best Practices Reference (2025-2026)

Compiled from: Uber Go Style Guide, JetBrains GoLand Blog, Go official docs, golangci-lint docs,
GORM docs, PingCAP, glukhov.org, reintech.io, cristiancurteanu.com, and other authoritative sources.

---

## 1. Uber Go Style Guide - Key Rules

Source: https://github.com/uber-go/guide/blob/master/style.md (updated Feb 2026)

### Guidelines (Correctness)

1. **Verify interface compliance at compile time**:
   ```go
   var _ http.Handler = (*Handler)(nil)
   ```
   Fails to compile if `*Handler` ever stops implementing `http.Handler`.

2. **Copy slices and maps at boundaries**: Never store references to caller-provided slices/maps. Copy them defensively:
   ```go
   func (d *Driver) SetTrips(trips []Trip) {
       d.trips = make([]Trip, len(trips))
       copy(d.trips, trips)
   }
   ```

3. **Defer to clean up**: Use `defer` for resource cleanup (files, locks, etc.) to guarantee execution.

4. **Channel size is one or none**: Channels should be unbuffered (size 0) or size 1. Any other size requires strong justification.

5. **Start enums at one**: So zero value indicates "unset" rather than a valid first value.

6. **Use `"time"` to handle time**: Use `time.Time` for instants, `time.Duration` for periods. Never raw ints for seconds.

7. **Handle type assertion failures**: Always use the two-value form `v, ok := x.(T)` to avoid panics.

8. **Don't Panic**: Production code must avoid panic. Return errors instead.

9. **Avoid mutable globals**: Inject dependencies instead. Mutable global state makes testing difficult and code brittle.

10. **Avoid embedding types in public structs**: Embedded types leak implementation details and inhibit evolution. Prefer explicit delegation.

11. **Avoid using built-in names**: Never shadow `error`, `string`, `len`, `make`, etc.

12. **Avoid `init()`**: Prefer explicit initialization. If init() is needed, it must be deterministic with no side effects beyond setting package-level state.

13. **Exit only in Main**: Call `os.Exit` or `log.Fatal` at most once, in `main()`. Extract logic into a `run() error` function:
    ```go
    func main() {
        if err := run(); err != nil {
            log.Fatal(err)
        }
    }
    ```

14. **Use field tags in marshaled structs**: Always specify explicit JSON field names.

15. **Don't fire-and-forget goroutines**: Every goroutine must have a predictable stop condition and a way to wait for it. Use `go.uber.org/goleak` to test for leaks.

### Error Handling (Uber Style)

16. **Error types decision matrix**:
    | Need matching? | Message type | Use |
    |---|---|---|
    | No | static | `errors.New` |
    | No | dynamic | `fmt.Errorf` |
    | Yes | static | top-level `var` + `errors.New` |
    | Yes | dynamic | custom `error` type |

17. **Error wrapping**:
    - Use `%w` when caller should access underlying cause (default choice)
    - Use `%v` to obfuscate the underlying error
    - Keep context succinct: avoid "failed to" prefixes that pile up

18. **Error naming**:
    - Exported sentinel errors: `ErrBrokenLink`
    - Unexported sentinel errors: `errNotFound` (not `_errNotFound`)
    - Custom error types: suffix with `Error` (e.g., `NotFoundError`)

19. **Handle errors once**: Either log OR return, never both. If you log and return, every caller up the stack will log again.

### Performance (Uber Style)

20. **Prefer `strconv` over `fmt`**: `strconv.Itoa()` is faster than `fmt.Sprintf("%d", n)`.

21. **Avoid repeated string-to-byte conversions**: Store the `[]byte` result if used multiple times.

22. **Prefer specifying container capacity**: Pre-allocate with `make([]T, 0, expectedSize)`.

### Style (Uber Style)

23. **Avoid overly long lines**: ~99 characters soft limit.

24. **Group similar declarations**: Group related `const`, `var`, `type` declarations.

25. **Import group ordering**: stdlib, then blank line, then everything else. Within groups, alphabetical.

26. **Function grouping and ordering**:
    - Sort by rough call order
    - Group by receiver
    - Exported functions first (after struct/const/var)
    - `NewXYZ()` right after type definition
    - Utility functions at end of file

27. **Reduce nesting**: Handle errors/special cases first, return early.

28. **Unnecessary else**: If both branches set a variable, use the default before the `if`:
    ```go
    a := 10
    if condition {
        a = 20
    }
    ```

29. **Prefix unexported globals with `_`**: `_defaultPort`, `_maxRetries`. Exception: error vars use `err` prefix.

30. **Nil is a valid slice**: Use `var t []string` (nil slice), not `t := []string{}` (empty non-nil slice), unless you need non-nil for JSON serialization.

31. **Reduce scope of variables**: Prefer `if err := ...; err != nil` over separate declaration.

32. **Use field names to initialize structs**: Always use named fields, never positional.

33. **Functional options pattern**: For constructors with 3+ optional arguments:
    ```go
    func Open(addr string, opts ...Option) (*Connection, error)
    ```

### Test Tables (Uber Style)

34. **Table-driven tests**: Use slice-of-structs pattern with `t.Run()` subtests. Each case should have `give` and `want` fields.

35. **Avoid complexity in table tests**: No conditional assertions, no `shouldError` branching. If logic varies, split into separate `Test...` functions.

---

## 2. Go Testing Best Practices

Sources: glukhov.org/2025, go.dev/wiki/TableDrivenTests, semaphore.io

### Structure

- Test files: `foo_test.go` alongside `foo.go`
- White-box tests: `package foo` (access unexported)
- Black-box tests: `package foo_test` (test public API only -- recommended for public interfaces)

### Table-Driven Tests

```go
func TestAdd(t *testing.T) {
    tests := []struct {
        name string
        a, b int
        want int
    }{
        {"positive", 2, 3, 5},
        {"zero", 0, 0, 0},
        {"negative", -1, -2, -3},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := Add(tt.a, tt.b)
            if got != tt.want {
                t.Errorf("Add(%d, %d) = %d, want %d", tt.a, tt.b, got, tt.want)
            }
        })
    }
}
```

### Test Helpers

```go
func newTestServer(t *testing.T) *httptest.Server {
    t.Helper() // Errors report caller's line, not helper's
    srv := httptest.NewServer(handler)
    t.Cleanup(func() { srv.Close() })
    return srv
}
```

### Parallel Tests

```go
for _, tt := range tests {
    tt := tt // capture range variable (not needed in Go 1.22+ with loop var fix)
    t.Run(tt.name, func(t *testing.T) {
        t.Parallel()
        // test body
    })
}
```

### TestMain for Expensive Setup

```go
func TestMain(m *testing.M) {
    setup()
    code := m.Run()
    teardown()
    os.Exit(code)
}
```

### Integration Tests with Build Tags

```go
//go:build integration

package mypackage_test
// Uses testcontainers-go for Docker-based DB tests
```

### Test Naming Convention

- `TestFunctionName` for unit tests
- `TestFunctionName_Scenario` for specific scenarios
- `TestFunctionName_Scenario_ExpectedResult` for maximum clarity
- Use `t.Run("descriptive name", ...)` for subtests

### Coverage

```bash
go test -cover ./...                    # Summary
go test -coverprofile=coverage.out ./... # Profile
go tool cover -html=coverage.out        # Visual HTML report
go test -race ./...                     # Race detector
go test -bench=. -benchmem              # Benchmarks with memory
```

### Key Rules

1. Write table-driven tests for multiple scenarios
2. Use `t.Run` for subtests (selective execution, better output)
3. Use `t.Helper()` in all helper functions
4. Test exported functions first (public API behavior)
5. Keep each test focused on one behavior
6. Use meaningful test names describing what + expected outcome
7. Don't test implementation details -- test behavior
8. Use interfaces for dependencies (enables mocking)
9. Always run with `-race` flag in CI
10. Use `TestMain` for expensive setup/teardown

---

## 3. Go Error Handling Best Practices

Sources: Uber Guide, JetBrains GoLand Blog (March 2026), go.dev/blog/go1.13-errors

### Core Rules

1. **Errors are values, not exceptions**: Handle explicitly at call site.

2. **Always check errors**: Never use `_ = someFunc()` for error-returning functions.

3. **Add context when wrapping**:
   ```go
   // Good: adds context
   return fmt.Errorf("get user %q: %w", id, err)

   // Bad: redundant "failed to" piles up
   return fmt.Errorf("failed to get user: %w", err)
   ```

4. **Use `errors.Is` and `errors.As`** instead of `==` or type assertions:
   ```go
   if errors.Is(err, sql.ErrNoRows) { ... }

   var pathErr *os.PathError
   if errors.As(err, &pathErr) { ... }
   ```

5. **Handle errors once**: Either log or return, never both:
   ```go
   // BAD
   log.Printf("query failed: %v", err)
   return err  // caller will also log

   // GOOD: wrap and return
   return fmt.Errorf("query user %s: %w", id, err)

   // GOOD: log and degrade gracefully
   log.Printf("cache miss: %v", err)
   result = fallbackValue
   ```

6. **Sentinel errors for known conditions**:
   ```go
   var (
       ErrNotFound     = errors.New("not found")
       ErrUnauthorized = errors.New("unauthorized")
   )
   ```

7. **Custom error types for structured info**:
   ```go
   type ValidationError struct {
       Field   string
       Message string
   }
   func (e *ValidationError) Error() string {
       return fmt.Sprintf("validation failed on %s: %s", e.Field, e.Message)
   }
   ```

### Security-Specific Error Handling (JetBrains 2026)

8. **Sanitize at subsystem boundaries**: Replace raw DB errors with domain errors:
   ```go
   // Raw: pq: duplicate key value violates unique constraint "users_email_key"
   // Sanitized: domain.ErrDuplicateUser
   ```

9. **Translate errors at API boundary**: Internal errors get wrapped and propagated between subsystems. At the HTTP handler, translate to user-safe messages:
   ```go
   // Handler layer -- the only place that maps errors to HTTP responses
   if errors.Is(err, domain.ErrNotFound) {
       http.Error(w, "Resource not found", 404)
       return
   }
   http.Error(w, "Internal server error", 500)
   ```

10. **Never expose internal details to users**: No stack traces, SQL queries, file paths, or dependency names in API responses.

11. **Log safe context only**: Even internal logs should avoid secrets. Log request IDs, entity IDs, operation names -- not tokens, passwords, or email bodies.

12. **Common security mistakes**:
    - Propagation of raw errors across trust boundaries
    - Accidentally logging secrets
    - Exposing verbose internal error messages to end users
    - Returning `err.Error()` directly instead of custom types

---

## 4. Go Concurrency Best Practices

Sources: cristiancurteanu.com/2025, Uber Guide, go.dev

### Goroutine Management

1. **Never fire-and-forget goroutines**: Every goroutine must have:
   - A predictable stop condition, AND
   - A way for the caller to wait for it to finish

2. **Use `sync.WaitGroup` for multiple goroutines**:
   ```go
   var wg sync.WaitGroup
   for i := 0; i < N; i++ {
       wg.Add(1)
       go func() {
           defer wg.Done()
           // work
       }()
   }
   wg.Wait()
   ```

3. **Use `done` channel for single goroutine**:
   ```go
   done := make(chan struct{})
   go func() {
       defer close(done)
       // work
   }()
   <-done
   ```

4. **Worker pools for bounded concurrency**:
   ```go
   func workerPool(numWorkers int, tasks []Task, results chan<- Result) {
       jobs := make(chan Task, len(tasks))
       for i := 0; i < numWorkers; i++ {
           go worker(jobs, results)
       }
       for _, task := range tasks {
           jobs <- task
       }
       close(jobs)
   }
   ```

5. **Recover panics in goroutines**: A panic in a goroutine crashes the whole process. `recover()` in main won't save it:
   ```go
   go func() {
       defer func() {
           if r := recover(); r != nil {
               slog.Error("goroutine panicked", "error", r)
           }
       }()
       doWork()
   }()
   ```

### Context Usage

6. **Always pass context as first parameter**: `func DoWork(ctx context.Context, ...) error`

7. **Use `context.WithTimeout` / `context.WithDeadline`** for operations that should not run forever.

8. **Always call `cancel()`**: Use `defer cancel()` immediately after creating a cancellable context.

9. **Check `ctx.Done()` in long-running loops**:
   ```go
   for {
       select {
       case <-ctx.Done():
           return ctx.Err()
       case task := <-tasks:
           process(task)
       }
   }
   ```

### errgroup Pattern

10. **Use `errgroup` for concurrent operations with error handling**:
    ```go
    g, ctx := errgroup.WithContext(ctx)
    for _, item := range items {
        item := item
        g.Go(func() error {
            return processItem(ctx, item)
        })
    }
    if err := g.Wait(); err != nil {
        return fmt.Errorf("processing items: %w", err)
    }
    ```
    Benefits: first-error cancellation, simplified coordination, no manual WaitGroup.

11. **Limit concurrency with `errgroup.SetLimit()`**:
    ```go
    g.SetLimit(10) // max 10 concurrent goroutines
    ```

### Race Detection

12. **Always run tests with `-race`**: `go test -race ./...`
13. **Use `go.uber.org/goleak` in tests** to detect goroutine leaks.

---

## 5. Go Project Structure Best Practices

Sources: go.dev/doc/modules/layout, golang-standards/project-layout, medium.com, oneuptime.com

### Standard Layout for HTTP Services

```
project/
├── cmd/
│   └── api/
│       └── main.go           # Entry point, DI wiring
├── internal/                  # Private packages (not importable by others)
│   ├── config/                # Configuration loading
│   ├── handler/               # HTTP handlers (request/response)
│   ├── middleware/             # HTTP middleware
│   ├── model/                 # Domain models, DTOs
│   ├── repository/            # Data access layer
│   ├── service/               # Business logic
│   ├── router/                # Route registration
│   └── worker/                # Background jobs
├── migrations/                # SQL migrations
├── pkg/                       # Reusable public packages (if any)
├── go.mod
└── go.sum
```

### Key Rules

1. **Use `internal/`** for all application-specific code. Prevents external imports.

2. **Keep `cmd/` minimal**: Only DI wiring and startup logic. All real logic in `internal/`.

3. **Favor shallow hierarchies**: One or two levels deep. Deep nesting increases cognitive load.

4. **Start simple, refactor as needed**: Don't over-engineer the initial structure.

5. **Handler -> Service -> Repository layering**: Handlers handle HTTP, services contain business logic, repositories talk to the database.

6. **Package by feature OR by layer**: Both are valid. Layer-based (handler/service/repo) works well for HTTP services. Feature-based works better for very large codebases.

7. **Avoid circular dependencies**: Go forbids circular imports. Design package boundaries to flow in one direction.

8. **One package per directory**: No mixed concerns.

---

## 6. Go Security Best Practices

Sources: GORM security docs, gosec, JetBrains, OWASP Golang guide

### Input Validation

1. **Validate all user input**: Use `github.com/go-playground/validator` or custom validation.
2. **Whitelist over blacklist**: Define allowed values rather than blocking known-bad ones.
3. **Check type conversions**: Parse string IDs to int with `strconv.Atoi()` before using in queries.

### SQL Injection Prevention

4. **Always use parameterized queries**:
   ```go
   // Safe
   db.Where("name = ?", userInput).First(&user)

   // VULNERABLE
   db.Where(fmt.Sprintf("name = %v", userInput)).First(&user)
   ```

5. **GORM unsafe methods** (never pass user input directly):
   - `db.Select()`, `db.Distinct()`, `db.Group()`, `db.Having()`
   - `db.Order()`, `db.Table()`, `db.Pluck()`
   - `db.Raw()`, `db.Exec()`
   - Always use `?` placeholders with these when incorporating user input.

6. **Validate numeric primary keys** before passing to GORM:
   ```go
   id, err := strconv.Atoi(userInputID)
   if err != nil {
       return err
   }
   db.First(&user, id) // Safe: integer type
   ```

### Secrets Handling

7. **Never store secrets in source code**: Use env vars or secret managers (Doppler, Vault, AWS Secrets Manager).
8. **Hash passwords with bcrypt**: `golang.org/x/crypto/bcrypt`.
9. **Use constant-time comparison** for token validation: `crypto/subtle.ConstantTimeCompare`.

### Security Tools

10. **gosec**: Scans for common security issues (hardcoded creds, SQL injection, weak crypto).
11. **govulncheck**: Scans dependencies for known CVEs:
    ```bash
    go install golang.org/x/vuln/cmd/govulncheck@latest
    govulncheck ./...
    ```
12. **Keep dependencies updated**: `go get -u ./...` regularly.

### API Security

13. **Sanitize errors at API boundary**: Never return raw internal errors to clients.
14. **Rate limit endpoints**: Prevent abuse.
15. **Validate Content-Type**: Reject unexpected content types.
16. **Set security headers**: CORS, CSP, X-Frame-Options, etc.

---

## 7. Go Performance Best Practices

Sources: reintech.io/2026, support.tools/2025, goperf.dev, Uber Guide

### The Three Pillars

Every Go performance problem falls into: **CPU-bound**, **Memory allocation**, or **I/O operations**.

### Profiling (Always Measure First)

```go
import "runtime/pprof"

// CPU profiling
f, _ := os.Create("cpu.prof")
pprof.StartCPUProfile(f)
defer pprof.StopCPUProfile()

// Memory profiling
f, _ := os.Create("mem.prof")
pprof.WriteHeapProfile(f)
```

```bash
go tool pprof cpu.prof       # Interactive analysis
go tool pprof -http=:8080 cpu.prof  # Web UI
go test -bench=. -benchmem   # Benchmark with memory stats
```

### Memory Allocation

1. **Pre-allocate slices**: `make([]T, 0, expectedLen)` avoids repeated growth.
2. **Use `strings.Builder`** for string concatenation (not `+=`).
3. **Use `sync.Pool`** for frequently allocated temporary objects (buffers, etc.):
   ```go
   var bufPool = sync.Pool{
       New: func() any { return new(bytes.Buffer) },
   }
   buf := bufPool.Get().(*bytes.Buffer)
   buf.Reset()
   defer bufPool.Put(buf)
   ```
   Can reduce allocation overhead by up to 45% in high-frequency scenarios.

4. **Prefer `strconv` over `fmt`**: `strconv.Itoa(n)` vs `fmt.Sprintf("%d", n)`.
5. **Avoid repeated string-to-byte conversions**: Store `[]byte` if used multiple times.

### Connection Pooling

6. **Database connections**:
   ```go
   sqlDB, _ := db.DB()
   sqlDB.SetMaxOpenConns(25)
   sqlDB.SetMaxIdleConns(10)
   sqlDB.SetConnMaxLifetime(5 * time.Minute)
   ```

7. **HTTP clients**: Reuse `http.Client` with connection pool:
   ```go
   client := &http.Client{
       Transport: &http.Transport{
           MaxIdleConns:        100,
           MaxIdleConnsPerHost: 10,
           IdleConnTimeout:     90 * time.Second,
       },
       Timeout: 10 * time.Second,
   }
   ```

### GC Tuning

8. **Default GOGC (100) is usually fine**. Only tune for specific workloads.
9. **GOMEMLIMIT** (Go 1.19+): Set soft memory limit for container environments.

### Concurrency Performance

10. **Worker pools**: Limit goroutines to prevent resource exhaustion. Rule of thumb: 10-20 per CPU core.
11. **Buffered I/O**: Use `bufio.Reader` / `bufio.Writer` for disk operations.

### Performance Checklist

1. Profile first (never optimize blind)
2. Reduce allocations (pre-allocate, strings.Builder, sync.Pool)
3. Optimize concurrency (worker pools, context cancellation)
4. Leverage compiler (inliner-friendly code, escape analysis)
5. Optimize I/O (connection pooling, buffered I/O, timeouts)
6. Benchmark everything (`go test -bench`)
7. Monitor production (GC metrics, goroutine counts, allocation rates)
8. Use PGO (Profile-Guided Optimization) in Go 1.21+

---

## 8. golangci-lint Recommended Configuration

Sources: golangci-lint.run, maratori golden config (v2.7.1), glukhov.org/2025

### Recommended Baseline `.golangci.yml`

```yaml
linters:
  enable:
    # Correctness
    - staticcheck      # Gold standard static analysis (150+ checks)
    - govet            # Reports suspicious constructs (shadow, printf args)
    - errcheck         # Unchecked errors (critical bugs)
    - gosec            # Security vulnerabilities
    - bodyclose        # HTTP response body not closed
    - contextcheck     # Context propagation issues
    - copyloopvar      # Loop variable capture bugs

    # Code Quality
    - revive           # Configurable replacement for golint
    - gocyclo          # Cyclomatic complexity
    - gosimple         # Simplifications
    - unconvert        # Unnecessary type conversions
    - unparam          # Unused function parameters
    - misspell         # Spelling mistakes in comments/strings
    - dupl             # Code duplication detection

    # Modernization
    - modernize        # Suggests modern Go idioms (new in 2025)

    # Performance
    - prealloc         # Suggests pre-allocating slices
    - noctx            # HTTP requests without context

    # Style
    - goimports        # Import ordering/formatting
    - asasalint        # Pass []any to variadic args
    - bidichk          # Dangerous unicode character sequences

linters-settings:
  errcheck:
    check-type-assertions: true
    check-blank: true

  govet:
    enable-all: true

  gocyclo:
    min-complexity: 15   # Flag functions over 15 cyclomatic complexity

  revive:
    severity: warning

  gosec:
    severity: high
    excludes:
      - G204             # Subprocess command (if you use os/exec intentionally)

run:
  timeout: 5m
  tests: true

issues:
  exclude-use-default: false
  max-issues-per-linter: 0
  max-same-issues: 0
```

### Key Linters Explained

| Linter | Category | What It Catches |
|--------|----------|-----------------|
| staticcheck | Correctness | Bugs, deprecated APIs, unreachable code, inefficient patterns |
| errcheck | Correctness | Unchecked error returns |
| gosec | Security | SQL injection, hardcoded creds, weak crypto, file traversal |
| bodyclose | Correctness | HTTP response bodies not closed (resource leak) |
| govet | Correctness | Printf format mismatches, struct tag issues, shadows |
| revive | Quality | Naming, exported without docs, blank imports, deep nesting |
| gocyclo | Quality | Cyclomatic complexity > threshold |
| gosimple | Quality | Simplifiable code patterns |
| unconvert | Quality | Unnecessary type conversions |
| unparam | Quality | Unused function parameters |
| misspell | Quality | Typos in comments and strings |
| modernize | Quality | Suggests modern Go idioms (new linter, 2025) |
| prealloc | Performance | Slices that could be pre-allocated |
| noctx | Correctness | HTTP requests missing context |
| contextcheck | Correctness | Broken context propagation chains |

### Security Scanning (Beyond golangci-lint)

```bash
# Run govulncheck for dependency CVEs
govulncheck ./...

# Run gosec standalone for detailed security report
gosec -fmt=json -out=results.json ./...
```

---

## 9. GORM Best Practices

Sources: gorm.io/docs, PingCAP best practices, GORM security docs

### Performance Configuration

```go
db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
    SkipDefaultTransaction: true,  // Disable per-write transactions (big perf gain)
    PrepareStmt:            true,  // Cache prepared statements
    Logger:                 logger.Default.LogMode(logger.Warn), // Don't log every query
})

// Connection pool settings
sqlDB, _ := db.DB()
sqlDB.SetMaxOpenConns(25)
sqlDB.SetMaxIdleConns(10)
sqlDB.SetConnMaxLifetime(5 * time.Minute)
```

### SQL Injection Prevention

**Safe -- uses parameterized queries:**
```go
db.Where("name = ?", userInput).First(&user)
db.First(&user, "name = ?", userInput)
```

**VULNERABLE -- never pass user input to these without `?` placeholders:**
```go
// These methods do NOT escape input:
db.Select(userInput)      // SQL injection
db.Distinct(userInput)    // SQL injection
db.Group(userInput)       // SQL injection
db.Having(userInput)      // SQL injection
db.Order(userInput)       // SQL injection
db.Table(userInput)       // SQL injection
db.Pluck(userInput, &out) // SQL injection
db.Raw(userInput)         // SQL injection
db.Exec(userInput)        // SQL injection
```

**Safe pattern for dynamic ordering:**
```go
allowedColumns := map[string]bool{"name": true, "created_at": true, "email": true}
if !allowedColumns[userInput] {
    return errors.New("invalid sort column")
}
db.Order(userInput + " ASC").Find(&users)
```

### Query Optimization

1. **Select only needed fields**: `db.Select("id", "name").Find(&users)` -- don't `SELECT *`.

2. **Use Preload for associations** instead of N+1 queries:
   ```go
   db.Preload("Comments").Find(&posts)
   ```

3. **Batch processing** for large datasets:
   ```go
   db.FindInBatches(&results, 100, func(tx *gorm.DB, batch int) error {
       for _, result := range results {
           // process
       }
       return nil
   })
   ```

4. **Use Scopes for reusable query conditions**:
   ```go
   func Active(db *gorm.DB) *gorm.DB {
       return db.Where("active = ?", true)
   }
   db.Scopes(Active).Find(&users)
   ```

### Transaction Management

```go
// Preferred: function-based transactions (auto-rollback on error)
err := db.Transaction(func(tx *gorm.DB) error {
    if err := tx.Create(&order).Error; err != nil {
        return err  // auto-rollback
    }
    if err := tx.Create(&payment).Error; err != nil {
        return err  // auto-rollback
    }
    return nil  // auto-commit
})
```

### Locking

```go
// Pessimistic locking (SELECT FOR UPDATE)
db.Clauses(clause.Locking{Strength: "UPDATE"}).First(&product, 1)
```

### Model Conventions

```go
type User struct {
    ID        string     `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
    Name      string     `json:"name" gorm:"not null"`
    Email     string     `json:"email" gorm:"uniqueIndex;not null"`
    DeletedAt *time.Time `json:"deleted_at" gorm:"index"` // Soft delete
    CreatedAt time.Time  `json:"created_at" gorm:"autoCreateTime"`
    UpdatedAt time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (User) TableName() string { return "users" }
```

### Connection & Error Handling

```go
// Retry logic for transient connection errors
var db *gorm.DB
for i := 0; i < 3; i++ {
    db, err = gorm.Open(postgres.Open(dsn), &config)
    if err == nil {
        break
    }
    time.Sleep(2 * time.Second)
}

// Always check for errors
result := db.First(&user, id)
if result.Error != nil {
    if errors.Is(result.Error, gorm.ErrRecordNotFound) {
        // handle not found
    }
    return fmt.Errorf("find user %s: %w", id, result.Error)
}
```

### Migrations

- **AutoMigrate** for development: `db.AutoMigrate(&User{}, &Post{})`
- **SQL migrations** for production: Use numbered sequential files, make them idempotent (`IF NOT EXISTS`).
- Never use AutoMigrate to drop columns or change types in production.

---

## 10. Chi Router Best Practices

Sources: github.com/go-chi/chi, alexedwards.net, sachinsmc.me

### Core Principles

1. **Chi is stdlib-compatible**: All middleware is standard `net/http` middleware. Any community middleware works.

2. **Lightweight**: Core router is <1000 LOC.

### Route Organization

```go
func NewRouter(h *Handlers, authz *authorization.AuthzService) *chi.Mux {
    r := chi.NewRouter()

    // Global middleware (applied to ALL routes)
    r.Use(middleware.RequestID)
    r.Use(middleware.RealIP)
    r.Use(middleware.Logger)
    r.Use(middleware.Recoverer)
    r.Use(middleware.Timeout(60 * time.Second))
    r.Use(cors.Handler(corsOptions))

    // Public routes
    r.Group(func(r chi.Router) {
        r.Post("/auth/login", h.Auth.Login)
        r.Post("/auth/register", h.Auth.Register)
    })

    // Authenticated routes
    r.Group(func(r chi.Router) {
        r.Use(RequireAuth)

        r.Get("/me", h.User.GetMe)

        // Workspace-scoped routes (nested group)
        r.Route("/workspaces/{id}", func(r chi.Router) {
            r.Use(RequireWorkspaceAccess)

            r.Get("/", h.Workspace.Get)
            r.With(RequirePermission("ws.update")).Put("/", h.Workspace.Update)
            r.With(RequireOwner).Delete("/", h.Workspace.Delete)

            // Sub-resource routes
            r.Route("/stories", func(r chi.Router) {
                r.Get("/", h.Story.List)
                r.With(RequirePermission("pm.edit")).Post("/", h.Story.Create)
            })
        })
    })

    return r
}
```

### Middleware Patterns

3. **Use `r.Use()` for group-wide middleware**: Applied to all routes in the group.

4. **Use `r.With()` for route-specific middleware**: Applied to individual routes:
   ```go
   r.With(RequirePermission("pm.edit")).Post("/stories", h.Story.Create)
   ```

5. **Group routes by authentication level**:
   - Public group (no auth middleware)
   - Authenticated group (`RequireAuth`)
   - Admin group (`RequireAuth`, `RequireAdmin`)

6. **Use `r.Route()` for sub-routers** with shared path prefixes.

7. **Middleware ordering matters**: Middleware runs in the order it's added. Put logging/recovery first, auth after.

### URL Parameters

```go
func (h *Handler) GetUser(w http.ResponseWriter, r *http.Request) {
    id := chi.URLParam(r, "id")
    // ...
}
```

### Middleware Best Practices

8. **Write middleware as closures** for dependency injection:
   ```go
   func RequirePermission(perm string) func(http.Handler) http.Handler {
       return func(next http.Handler) http.Handler {
           return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
               actor := GetActor(r.Context())
               if !actor.HasPermission(perm) {
                   http.Error(w, "Forbidden", 403)
                   return
               }
               next.ServeHTTP(w, r)
           })
       }
   }
   ```

9. **Use context for passing request-scoped data**:
   ```go
   ctx := context.WithValue(r.Context(), actorKey, actor)
   next.ServeHTTP(w, r.WithContext(ctx))
   ```

10. **Recovery middleware is essential**: Chi's `middleware.Recoverer` catches panics and returns 500 instead of crashing.

11. **Timeout middleware**: `middleware.Timeout(60 * time.Second)` prevents slow requests from hanging.

12. **WebSocket caveat**: WebSocket handlers bypass Chi's `Recoverer` because it strips the `http.Hijacker` interface. Handle WebSocket routes separately if needed.

---

## Summary: Top 25 Most Actionable Rules

1. Verify interface compliance at compile time with `var _ Interface = (*Type)(nil)`
2. Copy slices/maps at API boundaries
3. Handle errors once: log OR return, never both
4. Wrap errors with context using `fmt.Errorf("operation: %w", err)`
5. Use `errors.Is`/`errors.As` instead of `==` or type assertions
6. Sanitize errors at API boundary -- never expose internal details
7. Don't fire-and-forget goroutines -- ensure they can be stopped and waited on
8. Use `errgroup` for concurrent operations with error handling
9. Always pass and check `context.Context`
10. Pre-allocate slices with `make([]T, 0, cap)`
11. Use `sync.Pool` for high-frequency temporary allocations
12. Profile before optimizing (`pprof`, `go test -bench`)
13. Configure GORM: `SkipDefaultTransaction: true`, `PrepareStmt: true`
14. Never pass user input to GORM's Select/Order/Group/Raw/Exec without parameterization
15. Use table-driven tests with `t.Run()` subtests
16. Run tests with `-race` flag in CI
17. Use `t.Helper()` in all test helper functions
18. Enable golangci-lint with: staticcheck, errcheck, gosec, govet, revive, bodyclose
19. Run `govulncheck` regularly for dependency CVEs
20. Use Chi's `r.With()` for per-route middleware, `r.Use()` for group-wide
21. Keep `cmd/` minimal -- all logic in `internal/`
22. Use functional options for constructors with 3+ optional args
23. Prefix unexported globals with `_` (except error vars)
24. Reduce nesting with early returns
25. Configure DB connection pool: MaxOpenConns, MaxIdleConns, ConnMaxLifetime
