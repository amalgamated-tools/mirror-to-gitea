package github

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/google/go-github/v66/github"
	"golang.org/x/oauth2"
)

type Repository struct {
	Name         string
	URL          string
	Private      bool
	Fork         bool
	Owner        string
	FullName     string
	HasIssues    bool
	Organization string
	Starred      bool
}

type FetchOptions struct {
	Username             string
	PrivateRepositories  bool
	SkipForks            bool
	MirrorStarred        bool
	MirrorOrganizations  bool
	SingleRepo           string
	IncludeOrgs          []string
	ExcludeOrgs          []string
	PreserveOrgStructure bool
	UseSpecificUser      bool
}

func NewClient(token string) *github.Client {
	if token == "" {
		return github.NewClient(nil)
	}
	
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)
	return github.NewClient(tc)
}

func GetRepositories(ctx context.Context, client *github.Client, opts FetchOptions) ([]*Repository, error) {
	var repositories []*Repository

	// Check if we're mirroring a single repo
	if opts.SingleRepo != "" {
		repo, err := fetchSingleRepository(ctx, client, opts.SingleRepo)
		if err != nil {
			return nil, err
		}
		if repo != nil {
			repositories = append(repositories, repo)
		}
	} else {
		// Standard mirroring logic
		publicRepos, err := fetchPublicRepositories(ctx, client, opts.Username)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch public repositories: %w", err)
		}
		repositories = append(repositories, publicRepos...)

		if opts.PrivateRepositories {
			privateRepos, err := fetchPrivateRepositories(ctx, client)
			if err != nil {
				return nil, fmt.Errorf("failed to fetch private repositories: %w", err)
			}
			repositories = append(repositories, privateRepos...)
		}

		if opts.MirrorStarred {
			var username string
			if opts.UseSpecificUser {
				username = opts.Username
			}
			starredRepos, err := fetchStarredRepositories(ctx, client, username)
			if err != nil {
				return nil, fmt.Errorf("failed to fetch starred repositories: %w", err)
			}
			repositories = append(repositories, starredRepos...)
		}

		if opts.MirrorOrganizations {
			var username string
			if opts.UseSpecificUser {
				username = opts.Username
			}
			orgRepos, err := fetchOrganizationRepositories(ctx, client, username, opts.IncludeOrgs, opts.ExcludeOrgs, opts.PreserveOrgStructure, opts.PrivateRepositories)
			if err != nil {
				return nil, fmt.Errorf("failed to fetch organization repositories: %w", err)
			}
			repositories = append(repositories, orgRepos...)
		}

		// Filter duplicates
		repositories = filterDuplicates(repositories)
	}

	if opts.SkipForks {
		repositories = withoutForks(repositories)
	}

	return repositories, nil
}

func fetchSingleRepository(ctx context.Context, client *github.Client, repoURL string) (*Repository, error) {
	// Remove URL prefix if present and clean up
	repoPath := repoURL
	repoPath = strings.TrimPrefix(repoPath, "https://github.com/")
	repoPath = strings.TrimSuffix(repoPath, ".git")

	// Split into owner and repo
	parts := strings.Split(repoPath, "/")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid repository URL format: %s", repoURL)
	}

	owner, repoName := parts[0], parts[1]

	repo, _, err := client.Repositories.Get(ctx, owner, repoName)
	if err != nil {
		return nil, fmt.Errorf("error fetching single repository %s: %w", repoURL, err)
	}

	return toRepository(repo, false), nil
}

func fetchPublicRepositories(ctx context.Context, client *github.Client, username string) ([]*Repository, error) {
	opt := &github.RepositoryListOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}

	var allRepos []*github.Repository
	for {
		repos, resp, err := client.Repositories.List(ctx, username, opt)
		if err != nil {
			return nil, err
		}
		allRepos = append(allRepos, repos...)
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}

	return toRepositoryList(allRepos, false), nil
}

func fetchPrivateRepositories(ctx context.Context, client *github.Client) ([]*Repository, error) {
	opt := &github.RepositoryListOptions{
		Affiliation: "owner",
		Visibility:  "private",
		ListOptions: github.ListOptions{PerPage: 100},
	}

	var allRepos []*github.Repository
	for {
		repos, resp, err := client.Repositories.List(ctx, "", opt)
		if err != nil {
			return nil, err
		}
		allRepos = append(allRepos, repos...)
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}

	return toRepositoryList(allRepos, false), nil
}

func fetchStarredRepositories(ctx context.Context, client *github.Client, username string) ([]*Repository, error) {
	opt := &github.ActivityListStarredOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}

	var allRepos []*github.Repository
	
	if username != "" {
		// Use user-specific endpoint
		for {
			starred, resp, err := client.Activity.ListStarred(ctx, username, opt)
			if err != nil {
				return nil, err
			}
			for _, s := range starred {
				allRepos = append(allRepos, s.Repository)
			}
			if resp.NextPage == 0 {
				break
			}
			opt.Page = resp.NextPage
		}
	} else {
		// Use authenticated user endpoint
		for {
			starred, resp, err := client.Activity.ListStarred(ctx, "", opt)
			if err != nil {
				return nil, err
			}
			for _, s := range starred {
				allRepos = append(allRepos, s.Repository)
			}
			if resp.NextPage == 0 {
				break
			}
			opt.Page = resp.NextPage
		}
	}

	repos := toRepositoryList(allRepos, false)
	for _, repo := range repos {
		repo.Starred = true
	}

	return repos, nil
}

func fetchOrganizationRepositories(ctx context.Context, client *github.Client, username string, includeOrgs, excludeOrgs []string, preserveOrgStructure, privateRepoAccess bool) ([]*Repository, error) {
	opt := &github.ListOptions{PerPage: 100}

	var allOrgs []*github.Organization
	
	if username != "" {
		// Use user-specific endpoint
		for {
			orgs, resp, err := client.Organizations.List(ctx, username, opt)
			if err != nil {
				return nil, err
			}
			allOrgs = append(allOrgs, orgs...)
			if resp.NextPage == 0 {
				break
			}
			opt.Page = resp.NextPage
		}
	} else {
		// Use authenticated user endpoint
		for {
			orgs, resp, err := client.Organizations.List(ctx, "", opt)
			if err != nil {
				return nil, err
			}
			allOrgs = append(allOrgs, orgs...)
			if resp.NextPage == 0 {
				break
			}
			opt.Page = resp.NextPage
		}
	}

	// Filter organizations
	var orgsToProcess []*github.Organization
	for _, org := range allOrgs {
		orgName := org.GetLogin()
		
		// Check include list
		if len(includeOrgs) > 0 {
			include := false
			for _, includeName := range includeOrgs {
				if orgName == includeName {
					include = true
					break
				}
			}
			if !include {
				continue
			}
		}
		
		// Check exclude list
		exclude := false
		for _, excludeName := range excludeOrgs {
			if orgName == excludeName {
				exclude = true
				break
			}
		}
		if exclude {
			continue
		}
		
		orgsToProcess = append(orgsToProcess, org)
	}

	log.Printf("Processing repositories from %d organizations", len(orgsToProcess))

	var allOrgRepos []*Repository
	for _, org := range orgsToProcess {
		orgName := org.GetLogin()
		log.Printf("Fetching repositories for organization: %s", orgName)

		var orgRepos []*github.Repository
		
		if privateRepoAccess {
			// Use search API for both public and private repositories
			log.Printf("Using search API to fetch both public and private repositories for org: %s", orgName)
			searchQuery := fmt.Sprintf("org:%s", orgName)
			
			searchOpt := &github.SearchOptions{
				ListOptions: github.ListOptions{PerPage: 100},
			}
			
			for {
				result, resp, err := client.Search.Repositories(ctx, searchQuery, searchOpt)
				if err != nil {
					log.Printf("Error fetching repositories for org %s: %v", orgName, err)
					break
				}
				orgRepos = append(orgRepos, result.Repositories...)
				if resp.NextPage == 0 {
					break
				}
				searchOpt.Page = resp.NextPage
			}
			
			log.Printf("Found %d repositories (public and private) for org: %s", len(orgRepos), orgName)
		} else {
			// Use standard API for public repositories only
			repoOpt := &github.RepositoryListByOrgOptions{
				ListOptions: github.ListOptions{PerPage: 100},
			}
			
			for {
				repos, resp, err := client.Repositories.ListByOrg(ctx, orgName, repoOpt)
				if err != nil {
					log.Printf("Error fetching repositories for org %s: %v", orgName, err)
					break
				}
				orgRepos = append(orgRepos, repos...)
				if resp.NextPage == 0 {
					break
				}
				repoOpt.Page = resp.NextPage
			}
			
			log.Printf("Found %d public repositories for org: %s", len(orgRepos), orgName)
		}

		repos := toRepositoryList(orgRepos, preserveOrgStructure)
		if preserveOrgStructure {
			for _, repo := range repos {
				repo.Organization = orgName
			}
		}
		allOrgRepos = append(allOrgRepos, repos...)
	}

	return allOrgRepos, nil
}

func withoutForks(repositories []*Repository) []*Repository {
	var result []*Repository
	for _, repo := range repositories {
		if !repo.Fork {
			result = append(result, repo)
		}
	}
	return result
}

func filterDuplicates(repositories []*Repository) []*Repository {
	seen := make(map[string]bool)
	var result []*Repository

	for _, repo := range repositories {
		if !seen[repo.URL] {
			seen[repo.URL] = true
			result = append(result, repo)
		}
	}

	return result
}

func toRepository(repo *github.Repository, preserveOrg bool) *Repository {
	r := &Repository{
		Name:      repo.GetName(),
		URL:       repo.GetCloneURL(),
		Private:   repo.GetPrivate(),
		Fork:      repo.GetFork(),
		Owner:     repo.GetOwner().GetLogin(),
		FullName:  repo.GetFullName(),
		HasIssues: repo.GetHasIssues(),
	}
	return r
}

func toRepositoryList(repos []*github.Repository, preserveOrg bool) []*Repository {
	result := make([]*Repository, 0, len(repos))
	for _, repo := range repos {
		result = append(result, toRepository(repo, preserveOrg))
	}
	return result
}
