package main

import (
	"fmt"
	"io"
	"os"

	"github.com/helpin-ai/helpin/server/internal/aiusage"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, output io.Writer) error {
	if len(args) != 1 || args[0] != "validate" {
		return fmt.Errorf("usage: ai-pricing validate")
	}
	catalog, err := aiusage.LoadCatalog()
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "pricing %s valid\n", catalog.PricingVersion); err != nil {
		return fmt.Errorf("write validation result: %w", err)
	}
	return nil
}
