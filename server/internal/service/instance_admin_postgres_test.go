//go:build integration

package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// Races first signups across separate service instances (as separate API
// replicas would) against a migrated PostgreSQL database:
//
//	INSTANCE_ADMIN_TEST_DATABASE_URL=postgres://... go test -tags integration -run InstanceAdminPostgres ./internal/service/
//
// Point it only at a disposable database; it resets instance_settings.
func TestInstanceAdminPostgresConcurrentFirstSignup(t *testing.T) {
	dsn := os.Getenv("INSTANCE_ADMIN_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("INSTANCE_ADMIN_TEST_DATABASE_URL is not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	ctx := context.Background()
	if err := db.Exec(`DELETE FROM instance_settings`).Error; err != nil {
		t.Fatalf("reset settings: %v", err)
	}
	if err := db.Exec(`INSERT INTO instance_settings (singleton, signup_mode) VALUES (true, 'invite_only')`).Error; err != nil {
		t.Fatalf("seed fresh settings: %v", err)
	}

	const racers = 12
	suffix := uuid.NewString()[:8]
	var wg sync.WaitGroup
	var mu sync.Mutex
	var admins, rejected int
	var createdIDs []string
	start := make(chan struct{})
	for i := 0; i < racers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			// Each racer has its own service, so only the database serializes them.
			svc := NewInstanceService(repository.NewInstanceSettingsRepository(db), repository.NewUserRepository(db), InstanceServiceOptions{})
			email := fmt.Sprintf("racer-%s-%d@example.com", suffix, i)
			<-start
			user, admission, err := svc.CreateAccount(ctx, SignupCandidate{Email: email}, func(tx *gorm.DB, _ SignupAdmission) (*model.User, error) {
				return repository.NewUserRepository(tx).CreateUser(ctx, &model.User{Email: email, PasswordHash: "x", FullName: "Racer"})
			})
			mu.Lock()
			defer mu.Unlock()
			switch {
			case errors.Is(err, ErrSignupInviteOnly):
				rejected++
			case err != nil:
				t.Errorf("racer %d: %v", i, err)
			default:
				createdIDs = append(createdIDs, user.ID)
				if admission.ServerAdmin {
					admins++
				}
			}
		}(i)
	}
	close(start)
	wg.Wait()
	t.Cleanup(func() {
		if len(createdIDs) > 0 {
			db.Exec(`DELETE FROM users WHERE id IN ?`, createdIDs)
		}
	})

	if admins != 1 || len(createdIDs) != 1 || rejected != racers-1 {
		t.Fatalf("admins=%d created=%d rejected=%d; want exactly one admin account and %d rejections", admins, len(createdIDs), rejected, racers-1)
	}
	var stored int64
	db.Model(&model.User{}).Where("id IN ? AND is_server_admin", createdIDs).Count(&stored)
	if stored != 1 {
		t.Fatalf("stored admins = %d, want 1", stored)
	}
}
