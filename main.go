package main

import (
	"context"
	"log"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/jaedle/mirror-to-gitea/config"
	"github.com/jaedle/mirror-to-gitea/gitea"
	ghrepo "github.com/jaedle/mirror-to-gitea/github"
	"github.com/jaedle/mirror-to-gitea/logger"
	"github.com/google/go-github/v66/github"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("invalid configuration: %v", err)
	}

	lgr := logger.New()
	lgr.ShowConfig(cfg)

	ctx := context.Background()

	// Create Gitea client
	giteaClient := gitea.NewClient(&cfg.Gitea)

	// Create Gitea organization if specified
	if cfg.Gitea.Organization != "" {
		if err := giteaClient.CreateOrganization(cfg.Gitea.Organization, cfg.Gitea.Visibility, cfg.DryRun); err != nil {
			log.Printf("Warning: Failed to create Gitea organization %s: %v", cfg.Gitea.Organization, err)
		}
	}

	// Create the starred repositories organization if mirror starred is enabled
	if cfg.GitHub.MirrorStarred && cfg.Gitea.StarredReposOrg != "" {
		if err := giteaClient.CreateOrganization(cfg.Gitea.StarredReposOrg, cfg.Gitea.Visibility, cfg.DryRun); err != nil {
			log.Printf("Warning: Failed to create Gitea starred organization %s: %v", cfg.Gitea.StarredReposOrg, err)
		}
	}

	// Create GitHub client
	ghClient := ghrepo.NewClient(cfg.GitHub.Token)

	// Get GitHub repositories
	githubRepos, err := ghrepo.GetRepositories(ctx, ghClient, ghrepo.FetchOptions{
		Username:             cfg.GitHub.Username,
		PrivateRepositories:  cfg.GitHub.PrivateRepositories,
		SkipForks:            cfg.GitHub.SkipForks,
		MirrorStarred:        cfg.GitHub.MirrorStarred,
		MirrorOrganizations:  cfg.GitHub.MirrorOrganizations,
		SingleRepo:           cfg.GitHub.SingleRepo,
		IncludeOrgs:          cfg.GitHub.IncludeOrgs,
		ExcludeOrgs:          cfg.GitHub.ExcludeOrgs,
		PreserveOrgStructure: cfg.GitHub.PreserveOrgStructure,
		UseSpecificUser:      cfg.GitHub.UseSpecificUser,
	})
	if err != nil {
		log.Fatalf("Failed to fetch GitHub repositories: %v", err)
	}

	// Apply include/exclude filters
	filteredRepos := filterRepositories(githubRepos, cfg.Include, cfg.Exclude)
	log.Printf("Found %d repositories to mirror", len(filteredRepos))

	// Get Gitea user information
	giteaUser, err := giteaClient.GetUser()
	if err != nil {
		log.Fatalf("Failed to get Gitea user: %v", err)
	}

	// Create a map to store organization targets if preserving structure
	orgTargets := make(map[string]*gitea.Target)
	if cfg.GitHub.PreserveOrgStructure {
		// Get unique organization names from repositories
		uniqueOrgs := make(map[string]bool)
		for _, repo := range filteredRepos {
			if repo.Organization != "" {
				uniqueOrgs[repo.Organization] = true
			}
		}

		// Create or get each organization in Gitea
		for orgName := range uniqueOrgs {
			log.Printf("Preparing Gitea organization for GitHub organization: %s", orgName)

			if err := giteaClient.CreateOrganization(orgName, cfg.Gitea.Visibility, cfg.DryRun); err != nil {
				log.Printf("Error creating Gitea organization %s: %v", orgName, err)
				continue
			}

			orgTarget, err := giteaClient.GetOrganization(orgName)
			if err != nil {
				log.Printf("Error getting Gitea organization %s: %v", orgName, err)
				continue
			}

			orgTargets[orgName] = orgTarget
		}
	}

	// Mirror repositories
	for _, repo := range filteredRepos {
		if err := mirrorRepository(ctx, repo, cfg, giteaClient, ghClient, giteaUser, orgTargets); err != nil {
			log.Printf("Error mirroring repository %s: %v", repo.Name, err)
		}
	}

	log.Println("Mirroring process completed")
}

func filterRepositories(repos []*ghrepo.Repository, include, exclude []string) []*ghrepo.Repository {
	var filtered []*ghrepo.Repository

	for _, repo := range repos {
		// Check include patterns
		includeMatch := false
		for _, pattern := range include {
			matched, err := doublestar.Match(pattern, repo.Name)
			if err == nil && matched {
				includeMatch = true
				break
			}
		}

		if !includeMatch {
			continue
		}

		// Check exclude patterns
		excludeMatch := false
		for _, pattern := range exclude {
			matched, err := doublestar.Match(pattern, repo.Name)
			if err == nil && matched {
				excludeMatch = true
				break
			}
		}

		if !excludeMatch {
			filtered = append(filtered, repo)
		}
	}

	return filtered
}

func mirrorRepository(
	ctx context.Context,
	repo *ghrepo.Repository,
	cfg *config.Config,
	giteaClient *gitea.Client,
	ghClient *github.Client,
	giteaUser *gitea.Target,
	orgTargets map[string]*gitea.Target,
) error {
	// Determine the target (user or organization)
	var giteaTarget *gitea.Target

	// For starred repositories, use the starred repos organization if configured
	if repo.Starred && cfg.Gitea.StarredReposOrg != "" {
		starredOrg, err := giteaClient.GetOrganization(cfg.Gitea.StarredReposOrg)
		if err == nil {
			log.Printf("Using organization \"%s\" for starred repository: %s", cfg.Gitea.StarredReposOrg, repo.Name)
			giteaTarget = starredOrg
		} else {
			log.Printf("Could not find organization \"%s\" for starred repositories, using default target", cfg.Gitea.StarredReposOrg)
			giteaTarget = getDefaultTarget(cfg, giteaClient, giteaUser)
		}
	} else if cfg.GitHub.PreserveOrgStructure && repo.Organization != "" {
		// Use the organization as target
		if target, ok := orgTargets[repo.Organization]; ok {
			giteaTarget = target
		} else {
			log.Printf("No Gitea organization found for %s, using default target", repo.Organization)
			giteaTarget = getDefaultTarget(cfg, giteaClient, giteaUser)
		}
	} else {
		// Use the specified organization or user
		giteaTarget = getDefaultTarget(cfg, giteaClient, giteaUser)
	}

	// Check if already mirrored
	isAlreadyMirrored, err := giteaClient.IsRepositoryMirrored(repo.Name, giteaTarget)
	if err != nil {
		return err
	}

	// Special handling for starred repositories
	if repo.Starred {
		if isAlreadyMirrored {
			log.Printf("Repository %s is already mirrored in %s %s; checking if it needs to be starred.", repo.Name, giteaTarget.Type, giteaTarget.Name)
			return giteaClient.StarRepository(repo.Name, giteaTarget, cfg.DryRun)
		}
		if cfg.DryRun {
			log.Printf("DRY RUN: Would mirror and star repository to %s %s: %s (starred)", giteaTarget.Type, giteaTarget.Name, repo.Name)
			return nil
		}
	} else if isAlreadyMirrored {
		log.Printf("Repository %s is already mirrored in %s %s; doing nothing.", repo.Name, giteaTarget.Type, giteaTarget.Name)
		return nil
	} else if cfg.DryRun {
		log.Printf("DRY RUN: Would mirror repository to %s %s: %s", giteaTarget.Type, giteaTarget.Name, repo.Name)
		return nil
	}

	log.Printf("Mirroring repository to %s %s: %s%s", giteaTarget.Type, giteaTarget.Name, repo.Name, func() string {
		if repo.Starred {
			return " (will be starred)"
		}
		return ""
	}())

	// Mirror the repository
	if err := giteaClient.MirrorRepository(repo, giteaTarget, cfg.GitHub.Token); err != nil {
		return err
	}

	// Star the repository if it's marked as starred
	if repo.Starred {
		if err := giteaClient.StarRepository(repo.Name, giteaTarget, cfg.DryRun); err != nil {
			log.Printf("Warning: Failed to star repository %s: %v", repo.Name, err)
		}
	}

	// Mirror issues if requested
	shouldMirrorIssues := cfg.GitHub.MirrorIssues && !(repo.Starred && cfg.GitHub.SkipStarredIssues)

	if shouldMirrorIssues && !cfg.DryRun {
		if err := giteaClient.MirrorIssues(ctx, ghClient, repo, giteaTarget, cfg.GitHub.Token, cfg.DryRun); err != nil {
			log.Printf("Warning: Failed to mirror issues for %s: %v", repo.Name, err)
		}
	} else if repo.Starred && cfg.GitHub.SkipStarredIssues {
		log.Printf("Skipping issues for starred repository: %s", repo.Name)
	}

	return nil
}

func getDefaultTarget(cfg *config.Config, giteaClient *gitea.Client, giteaUser *gitea.Target) *gitea.Target {
	if cfg.Gitea.Organization != "" {
		org, err := giteaClient.GetOrganization(cfg.Gitea.Organization)
		if err == nil {
			return org
		}
		log.Printf("Warning: Failed to get Gitea organization %s, using user instead: %v", cfg.Gitea.Organization, err)
	}
	return giteaUser
}
