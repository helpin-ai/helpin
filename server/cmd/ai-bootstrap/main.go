// Command ai-bootstrap imports explicitly named credentials and migrates current
// workspace agent defaults. It does not restart services or enable strict policy.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/service"
	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type credentialFlags []string

func (f *credentialFlags) String() string         { return strings.Join(*f, ",") }
func (f *credentialFlags) Set(value string) error { *f = append(*f, value); return nil }

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	envFile := flag.String("env-file", "", "explicit dotenv path (default: process environment only)")
	workspace := flag.String("workspace", "", "workspace UUID to bootstrap")
	funding := flag.String("funding", "customer", "customer, or managed in an EE build")
	rotate := flag.Bool("rotate-credentials", false, "replace the explicitly supplied bootstrap credentials")
	apply := flag.Bool("apply", false, "commit changes (default is a rolled-back preview)")
	var credentials credentialFlags
	flag.Var(&credentials, "credential", "explicit provider=ENVIRONMENT_VARIABLE mapping; repeat for each provider")
	flag.Parse()
	if *workspace == "" || (*funding != "customer" && *funding != "managed") {
		return fmt.Errorf("workspace and valid funding are required")
	}
	if *funding == "managed" && !managedBootstrapEnabled {
		return fmt.Errorf("managed credentials require an EE build (-tags ee)")
	}
	if *envFile != "" {
		if err := godotenv.Load(*envFile); err != nil {
			return fmt.Errorf("load explicit configuration: %w", err)
		}
	}
	keys := map[string]string{}
	for _, mapping := range credentials {
		provider, env, ok := strings.Cut(mapping, "=")
		if !ok || env == "" {
			return fmt.Errorf("credential mappings must name a provider and environment variable")
		}
		value := os.Getenv(env)
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("an explicitly mapped credential environment variable is unset")
		}
		if _, exists := keys[provider]; exists {
			return fmt.Errorf("duplicate provider mapping")
		}
		keys[provider] = value
	}
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	defer sqlDB.Close()
	bootstrap, err := service.NewAIProfileBootstrapService(repository.NewAIProfileBootstrapRepository(db), os.Getenv("AI_CONNECTION_ENCRYPTION_KEY"))
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	result, err := bootstrap.Apply(ctx, service.AIProfileBootstrapOptions{WorkspaceID: *workspace, Funding: *funding, Credentials: keys, DryRun: !*apply, RotateCredentials: *rotate})
	if err != nil {
		return fmt.Errorf("bootstrap: %w", err)
	}
	return json.NewEncoder(os.Stdout).Encode(struct {
		Applied bool `json:"applied"`
		*service.AIProfileBootstrapResult
	}{Applied: *apply, AIProfileBootstrapResult: result})
}
