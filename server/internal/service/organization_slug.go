package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

type organizationSlugLookup interface {
	GetBySlug(ctx context.Context, slug string) (*model.Organization, error)
}

func nextAvailableOrganizationSlug(ctx context.Context, repo organizationSlugLookup, baseSlug string) (string, error) {
	if strings.TrimSpace(baseSlug) == "" {
		baseSlug = "organization"
	}
	for i := 0; ; i++ {
		slug := baseSlug
		if i > 0 {
			slug = fmt.Sprintf("%s-%d", baseSlug, i+1)
		}
		existing, err := repo.GetBySlug(ctx, slug)
		if err != nil {
			return "", err
		}
		if existing == nil {
			return slug, nil
		}
	}
}
