package coordination

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestPostgresLeaderAllowsOneWorkerAndFailsOver(t *testing.T) {
	locker := &fakeLeaderLocker{}
	first := testLeader(locker, "pod-1")
	second := testLeader(locker, "pod-2")
	firstCtx, cancelFirst := context.WithCancel(context.Background())
	secondCtx, cancelSecond := context.WithCancel(context.Background())
	defer cancelSecond()

	var active atomic.Int32
	var maximum atomic.Int32
	firstStarted := make(chan struct{}, 1)
	secondStarted := make(chan struct{}, 1)
	work := func(started chan<- struct{}) func(context.Context) error {
		return func(ctx context.Context) error {
			current := active.Add(1)
			defer active.Add(-1)
			for {
				previous := maximum.Load()
				if current <= previous || maximum.CompareAndSwap(previous, current) {
					break
				}
			}
			started <- struct{}{}
			<-ctx.Done()
			return nil
		}
	}

	firstDone := make(chan error, 1)
	secondDone := make(chan error, 1)
	go func() { firstDone <- first.Run(firstCtx, work(firstStarted)) }()
	select {
	case <-firstStarted:
	case <-time.After(time.Second):
		t.Fatal("first leader did not start")
	}
	go func() { secondDone <- second.Run(secondCtx, work(secondStarted)) }()
	select {
	case <-secondStarted:
		t.Fatal("second worker started while the first still held leadership")
	case <-time.After(40 * time.Millisecond):
	}

	cancelFirst()
	select {
	case err := <-firstDone:
		if err != nil {
			t.Fatalf("first leader returned error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("first leader did not stop")
	}
	select {
	case <-secondStarted:
	case <-time.After(time.Second):
		t.Fatal("second leader did not take over")
	}
	if maximum.Load() != 1 {
		t.Fatalf("maximum concurrent workers = %d, want 1", maximum.Load())
	}

	cancelSecond()
	select {
	case err := <-secondDone:
		if err != nil {
			t.Fatalf("second leader returned error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("second leader did not stop")
	}
}

func TestPostgresLeaderCancelsWorkWhenLeaseHealthFails(t *testing.T) {
	locker := &fakeLeaderLocker{}
	leader := testLeader(locker, "pod-1")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started := make(chan struct{})
	stopped := make(chan struct{})
	var startedOnce sync.Once
	var stoppedOnce sync.Once
	done := make(chan error, 1)
	go func() {
		done <- leader.Run(ctx, func(workCtx context.Context) error {
			startedOnce.Do(func() { close(started) })
			<-workCtx.Done()
			stoppedOnce.Do(func() { close(stopped) })
			return nil
		})
	}()
	<-started
	locker.FailHealth(errors.New("connection lost"))
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("leader work was not cancelled after health failure")
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("leader returned error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("leader did not stop")
	}
}

func TestAdvisoryLockIDIsStableAndNamespaced(t *testing.T) {
	first := advisoryLockID("agent-runtime-projection:helpin:v2")
	if first != advisoryLockID("agent-runtime-projection:helpin:v2") {
		t.Fatal("advisory lock id is not stable")
	}
	if first == advisoryLockID("agent-runtime-projection:helpin.stage:v2") {
		t.Fatal("distinct leader names produced the same lock id")
	}
}

func testLeader(locker leaderLocker, instanceID string) *PostgresLeader {
	return &PostgresLeader{
		locker:         locker,
		name:           "test-leader",
		instanceID:     instanceID,
		retryInterval:  5 * time.Millisecond,
		healthInterval: 5 * time.Millisecond,
	}
}

type fakeLeaderLocker struct {
	mu        sync.Mutex
	held      bool
	healthErr error
}

func (l *fakeLeaderLocker) TryAcquire(context.Context) (leaderLease, bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.held {
		return nil, false, nil
	}
	l.held = true
	return &fakeLeaderLease{locker: l}, true, nil
}

func (l *fakeLeaderLocker) FailHealth(err error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.healthErr = err
}

type fakeLeaderLease struct {
	locker *fakeLeaderLocker
}

func (l *fakeLeaderLease) HealthCheck(context.Context) error {
	l.locker.mu.Lock()
	defer l.locker.mu.Unlock()
	return l.locker.healthErr
}

func (l *fakeLeaderLease) Release() {
	l.locker.mu.Lock()
	defer l.locker.mu.Unlock()
	l.locker.held = false
	l.locker.healthErr = nil
}
