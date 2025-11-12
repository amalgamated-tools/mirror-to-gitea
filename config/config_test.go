package config

import (
	"os"
	"testing"
)

func TestConfiguration(t *testing.T) {
	// Clean up environment variables before each test
	cleanup := func() {
		vars := []string{
			"DELAY", "DRY_RUN", "GITEA_TOKEN", "GITEA_URL",
			"GITHUB_TOKEN", "GITHUB_USERNAME", "MIRROR_PRIVATE_REPOSITORIES",
			"SKIP_FORKS", "MIRROR_ISSUES", "MIRROR_STARRED", "MIRROR_ORGANIZATIONS",
			"SINGLE_REPO", "GITEA_ORGANIZATION", "GITEA_ORG_VISIBILITY",
			"GITEA_STARRED_ORGANIZATION", "INCLUDE_ORGS", "EXCLUDE_ORGS",
			"PRESERVE_ORG_STRUCTURE", "SKIP_STARRED_ISSUES", "USE_SPECIFIC_USER",
			"INCLUDE", "EXCLUDE", "SINGLE_RUN",
		}
		for _, v := range vars {
			os.Unsetenv(v)
		}
	}

	provideMandatory := func() {
		os.Setenv("GITHUB_USERNAME", "test-username")
		os.Setenv("GITEA_URL", "https://gitea.url")
		os.Setenv("GITEA_TOKEN", "secret-gitea-token")
	}

	t.Run("reads configuration with default values", func(t *testing.T) {
		cleanup()
		provideMandatory()

		cfg, err := Load()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if cfg.GitHub.Username != "test-username" {
			t.Errorf("expected username 'test-username', got %s", cfg.GitHub.Username)
		}

		if cfg.GitHub.Token != "" {
			t.Errorf("expected empty token, got %s", cfg.GitHub.Token)
		}

		if cfg.GitHub.SkipForks {
			t.Error("expected SkipForks to be false")
		}

		if cfg.Gitea.URL != "https://gitea.url" {
			t.Errorf("expected URL 'https://gitea.url', got %s", cfg.Gitea.URL)
		}

		if cfg.Gitea.Token != "secret-gitea-token" {
			t.Errorf("expected token 'secret-gitea-token', got %s", cfg.Gitea.Token)
		}

		if cfg.Delay != 3600 {
			t.Errorf("expected delay 3600, got %d", cfg.Delay)
		}
	})

	t.Run("requires gitea url", func(t *testing.T) {
		cleanup()
		provideMandatory()
		os.Unsetenv("GITEA_URL")

		_, err := Load()
		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("requires gitea token", func(t *testing.T) {
		cleanup()
		provideMandatory()
		os.Unsetenv("GITEA_TOKEN")

		_, err := Load()
		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("requires github username", func(t *testing.T) {
		cleanup()
		provideMandatory()
		os.Unsetenv("GITHUB_USERNAME")

		_, err := Load()
		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("reads github token", func(t *testing.T) {
		cleanup()
		provideMandatory()
		os.Setenv("GITHUB_TOKEN", "test-github-token")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if cfg.GitHub.Token != "test-github-token" {
			t.Errorf("expected token 'test-github-token', got %s", cfg.GitHub.Token)
		}
	})

	t.Run("dry run flag treats true as true", func(t *testing.T) {
		cleanup()
		provideMandatory()
		os.Setenv("DRY_RUN", "true")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !cfg.DryRun {
			t.Error("expected DryRun to be true")
		}
	})

	t.Run("dry run flag treats 1 as true", func(t *testing.T) {
		cleanup()
		provideMandatory()
		os.Setenv("DRY_RUN", "1")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !cfg.DryRun {
			t.Error("expected DryRun to be true")
		}
	})

	t.Run("dry run flag treats missing as false", func(t *testing.T) {
		cleanup()
		provideMandatory()

		cfg, err := Load()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if cfg.DryRun {
			t.Error("expected DryRun to be false")
		}
	})

	t.Run("skip forks flag treats true as true", func(t *testing.T) {
		cleanup()
		provideMandatory()
		os.Setenv("SKIP_FORKS", "true")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !cfg.GitHub.SkipForks {
			t.Error("expected SkipForks to be true")
		}
	})

	t.Run("skip forks flag treats 1 as true", func(t *testing.T) {
		cleanup()
		provideMandatory()
		os.Setenv("SKIP_FORKS", "1")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !cfg.GitHub.SkipForks {
			t.Error("expected SkipForks to be true")
		}
	})

	t.Run("skip forks flag treats missing as false", func(t *testing.T) {
		cleanup()
		provideMandatory()

		cfg, err := Load()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if cfg.GitHub.SkipForks {
			t.Error("expected SkipForks to be false")
		}
	})

	t.Run("mirror private repositories flag treats true as true", func(t *testing.T) {
		cleanup()
		provideMandatory()
		os.Setenv("GITHUB_TOKEN", "test-token")
		os.Setenv("MIRROR_PRIVATE_REPOSITORIES", "true")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !cfg.GitHub.PrivateRepositories {
			t.Error("expected PrivateRepositories to be true")
		}
	})

	t.Run("mirror private repositories flag treats 1 as true", func(t *testing.T) {
		cleanup()
		provideMandatory()
		os.Setenv("GITHUB_TOKEN", "test-token")
		os.Setenv("MIRROR_PRIVATE_REPOSITORIES", "1")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !cfg.GitHub.PrivateRepositories {
			t.Error("expected PrivateRepositories to be true")
		}
	})

	t.Run("mirror private repositories flag treats missing as false", func(t *testing.T) {
		cleanup()
		provideMandatory()

		cfg, err := Load()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if cfg.GitHub.PrivateRepositories {
			t.Error("expected PrivateRepositories to be false")
		}
	})

	t.Run("requires github token on private repository mirroring", func(t *testing.T) {
		cleanup()
		provideMandatory()
		os.Setenv("MIRROR_PRIVATE_REPOSITORIES", "true")

		_, err := Load()
		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("parses delay", func(t *testing.T) {
		cleanup()
		provideMandatory()
		os.Setenv("DELAY", "1200")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if cfg.Delay != 1200 {
			t.Errorf("expected delay 1200, got %d", cfg.Delay)
		}
	})
}
