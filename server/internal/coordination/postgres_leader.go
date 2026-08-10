// Package coordination provides process leadership for singleton background work.
package coordination

import (
	"context"
	"database/sql"
	"fmt"
	"hash/fnv"
	"log/slog"
	"time"
)

const (
	defaultLeaderRetryInterval  = 2 * time.Second
	defaultLeaderHealthInterval = 5 * time.Second
)

// PostgresLeaderOptions configures a PostgreSQL advisory-lock leadership loop.
type PostgresLeaderOptions struct {
	Name           string
	InstanceID     string
	RetryInterval  time.Duration
	HealthInterval time.Duration
}

// PostgresLeader runs work only while this process owns a session-scoped
// PostgreSQL advisory lock. Losing the pinned database connection cancels the
// work before this process can contend for leadership again.
type PostgresLeader struct {
	locker         leaderLocker
	name           string
	instanceID     string
	retryInterval  time.Duration
	healthInterval time.Duration
}

// NewPostgresLeader creates a leader elector backed by a pinned sql.Conn.
func NewPostgresLeader(db *sql.DB, options PostgresLeaderOptions) *PostgresLeader {
	name := options.Name
	if name == "" {
		name = "helpin-background-work"
	}
	retryInterval := options.RetryInterval
	if retryInterval <= 0 {
		retryInterval = defaultLeaderRetryInterval
	}
	healthInterval := options.HealthInterval
	if healthInterval <= 0 {
		healthInterval = defaultLeaderHealthInterval
	}
	return &PostgresLeader{
		locker: &postgresAdvisoryLocker{
			db:     db,
			lockID: advisoryLockID(name),
		},
		name:           name,
		instanceID:     options.InstanceID,
		retryInterval:  retryInterval,
		healthInterval: healthInterval,
	}
}

// Run continually contends for leadership and invokes work for each term.
// Work must stop when its context is cancelled. Run returns when ctx ends.
func (l *PostgresLeader) Run(ctx context.Context, work func(context.Context) error) error {
	if l == nil || l.locker == nil {
		return fmt.Errorf("postgres leader is not configured")
	}
	if work == nil {
		return fmt.Errorf("leader work is required")
	}

	for {
		if ctx.Err() != nil {
			return nil
		}
		lease, acquired, err := l.locker.TryAcquire(ctx)
		if err != nil {
			slog.WarnContext(ctx, "leader election acquire failed",
				"name", l.name,
				"instance_id", l.instanceID,
				"error", err,
			)
			if !waitForLeaderRetry(ctx, l.retryInterval) {
				return nil
			}
			continue
		}
		if !acquired {
			if !waitForLeaderRetry(ctx, l.retryInterval) {
				return nil
			}
			continue
		}

		slog.InfoContext(ctx, "leadership acquired", "name", l.name, "instance_id", l.instanceID)
		err = l.runTerm(ctx, lease, work)
		if ctx.Err() != nil {
			return nil
		}
		if err != nil {
			slog.WarnContext(ctx, "leadership term ended",
				"name", l.name,
				"instance_id", l.instanceID,
				"error", err,
			)
		}
		if !waitForLeaderRetry(ctx, l.retryInterval) {
			return nil
		}
	}
}

func (l *PostgresLeader) runTerm(
	ctx context.Context,
	lease leaderLease,
	work func(context.Context) error,
) error {
	workCtx, cancelWork := context.WithCancel(ctx)
	defer cancelWork()
	defer lease.Release()

	workDone := make(chan error, 1)
	go func() {
		workDone <- work(workCtx)
	}()

	ticker := time.NewTicker(l.healthInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			cancelWork()
			<-workDone
			return nil
		case err := <-workDone:
			return err
		case <-ticker.C:
			healthCtx, cancelHealth := context.WithTimeout(ctx, l.healthInterval)
			err := lease.HealthCheck(healthCtx)
			cancelHealth()
			if err == nil {
				continue
			}
			cancelWork()
			<-workDone
			return fmt.Errorf("leadership connection lost: %w", err)
		}
	}
}

func waitForLeaderRetry(ctx context.Context, interval time.Duration) bool {
	timer := time.NewTimer(interval)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func advisoryLockID(name string) int64 {
	hasher := fnv.New64a()
	_, _ = hasher.Write([]byte(name))
	return int64(hasher.Sum64() & uint64(^uint64(0)>>1))
}

type leaderLocker interface {
	TryAcquire(ctx context.Context) (leaderLease, bool, error)
}

type leaderLease interface {
	HealthCheck(ctx context.Context) error
	Release()
}

type postgresAdvisoryLocker struct {
	db     *sql.DB
	lockID int64
}

func (l *postgresAdvisoryLocker) TryAcquire(ctx context.Context) (leaderLease, bool, error) {
	if l == nil || l.db == nil {
		return nil, false, fmt.Errorf("database is not configured")
	}
	conn, err := l.db.Conn(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("open leadership connection: %w", err)
	}
	var acquired bool
	if err := conn.QueryRowContext(ctx, `SELECT pg_try_advisory_lock($1)`, l.lockID).Scan(&acquired); err != nil {
		_ = conn.Close()
		return nil, false, fmt.Errorf("acquire advisory lock: %w", err)
	}
	if !acquired {
		_ = conn.Close()
		return nil, false, nil
	}
	return &postgresAdvisoryLease{conn: conn, lockID: l.lockID}, true, nil
}

type postgresAdvisoryLease struct {
	conn   *sql.Conn
	lockID int64
}

func (l *postgresAdvisoryLease) HealthCheck(ctx context.Context) error {
	if l == nil || l.conn == nil {
		return fmt.Errorf("leadership connection is closed")
	}
	if err := l.conn.PingContext(ctx); err != nil {
		return fmt.Errorf("ping leadership connection: %w", err)
	}
	return nil
}

func (l *postgresAdvisoryLease) Release() {
	if l == nil || l.conn == nil {
		return
	}
	releaseCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := l.conn.ExecContext(releaseCtx, `SELECT pg_advisory_unlock($1)`, l.lockID); err != nil {
		slog.Warn("release leadership advisory lock failed", "lock_id", l.lockID, "error", err)
	}
	if err := l.conn.Close(); err != nil {
		slog.Warn("close leadership connection failed", "lock_id", l.lockID, "error", err)
	}
	l.conn = nil
}
