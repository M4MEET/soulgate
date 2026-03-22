package core

import (
	"fmt"

	"github.com/M4MEET/soulgate/internal/integrations"
	"github.com/M4MEET/soulgate/internal/integrations/analytics"
	"github.com/M4MEET/soulgate/internal/integrations/aws"
	"github.com/M4MEET/soulgate/internal/integrations/database"
	"github.com/M4MEET/soulgate/internal/integrations/docker"
	"github.com/M4MEET/soulgate/internal/integrations/github"
	"github.com/M4MEET/soulgate/internal/integrations/google"
	"github.com/M4MEET/soulgate/internal/integrations/notion"
	"github.com/M4MEET/soulgate/internal/integrations/slack"
)

// registerDefaultIntegrations registers all built-in integrations into the
// provided registry. It returns the first error encountered, if any.
//
// The registration list lives here rather than in the integrations package
// itself because every sub-package (analytics, aws, …) already imports the
// parent integrations package for shared types; pulling sub-packages back into
// the parent would create an import cycle.
func registerDefaultIntegrations(reg *integrations.Registry) error {
	entries := []struct {
		integration integrations.Integration
		name        string
	}{
		{github.New(), "github"},
		{slack.New(), "slack"},
		{database.NewPostgres(), "postgres"},
		{google.NewDrive(), "google_drive"},
		{google.NewGmail(), "gmail"},
		{docker.New(), "docker"},
		{aws.NewS3(), "aws_s3"},
		{notion.New(), "notion"},
		{analytics.New(), "analytics"},
	}

	for _, e := range entries {
		if err := reg.Register(e.integration); err != nil {
			return fmt.Errorf("failed to register %s integration: %w", e.name, err)
		}
	}

	return nil
}
