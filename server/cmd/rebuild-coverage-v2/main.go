package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/service"
	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type commandOptions struct {
	workspaceID string
	apply       bool
	resumeID    string
	rollbackID  string
}

func parseOptions(args []string) (commandOptions, error) {
	flags := flag.NewFlagSet("rebuild-coverage-v2", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var options commandOptions
	flags.StringVar(&options.workspaceID, "workspace-id", "", "required workspace UUID")
	flags.BoolVar(&options.apply, "apply", false, "archive v1 rows and enqueue v2 work")
	flags.StringVar(&options.resumeID, "resume", "", "resume/report an applied rebuild audit")
	flags.StringVar(&options.rollbackID, "rollback", "", "restore v1 statuses from a rebuild audit")
	if err := flags.Parse(args); err != nil {
		return options, err
	}
	if strings.TrimSpace(options.workspaceID) == "" {
		return options, fmt.Errorf("--workspace-id is required")
	}
	modes := 0
	if options.apply {
		modes++
	}
	if options.resumeID != "" {
		modes++
	}
	if options.rollbackID != "" {
		modes++
	}
	if modes > 1 {
		return options, fmt.Errorf("--apply, --resume, and --rollback are mutually exclusive")
	}
	return options, nil
}

func main() {
	_ = godotenv.Load()
	options, err := parseOptions(os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}
	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}
	db, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{})
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	rebuild := service.NewSupportCoverageRebuildService(repository.NewCoverageV2Repository(db))
	result, err := rebuild.Run(ctx, service.CoverageRebuildOptions{WorkspaceID: options.workspaceID, Apply: options.apply, ResumeID: options.resumeID, RollbackID: options.rollbackID})
	if err != nil {
		log.Fatal(err)
	}
	encoded, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(encoded))
}
