package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type GitHubConfig struct {
	Username             string
	Token                string
	SkipForks            bool
	PrivateRepositories  bool
	MirrorIssues         bool
	MirrorStarred        bool
	MirrorOrganizations  bool
	UseSpecificUser      bool
	SingleRepo           string
	IncludeOrgs          []string
	ExcludeOrgs          []string
	PreserveOrgStructure bool
	SkipStarredIssues    bool
}

type GiteaConfig struct {
	URL             string
	Token           string
	Organization    string
	Visibility      string
	StarredReposOrg string
}

type Config struct {
	GitHub    GitHubConfig
	Gitea     GiteaConfig
	DryRun    bool
	Delay     int
	Include   []string
	Exclude   []string
	SingleRun bool
}

func readEnv(variable string) string {
	return os.Getenv(variable)
}

func mustReadEnv(variable string) (string, error) {
	val := os.Getenv(variable)
	if val == "" {
		return "", fmt.Errorf("invalid configuration, please provide %s", variable)
	}
	return val, nil
}

func readBoolean(variable string) bool {
	val := os.Getenv(variable)
	return val == "true" || val == "TRUE" || val == "1"
}

func readInt(variable string, defaultValue int) int {
	val := os.Getenv(variable)
	if val == "" {
		return defaultValue
	}
	intVal, err := strconv.Atoi(val)
	if err != nil {
		return defaultValue
	}
	return intVal
}

func splitAndTrim(s string) []string {
	if s == "" {
		return []string{}
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func Load() (*Config, error) {
	const defaultDelay = 3600
	const defaultInclude = "*"
	const defaultExclude = ""

	githubUsername, err := mustReadEnv("GITHUB_USERNAME")
	if err != nil {
		return nil, err
	}

	giteaURL, err := mustReadEnv("GITEA_URL")
	if err != nil {
		return nil, err
	}

	giteaToken, err := mustReadEnv("GITEA_TOKEN")
	if err != nil {
		return nil, err
	}

	githubToken := readEnv("GITHUB_TOKEN")
	privateRepositories := readBoolean("MIRROR_PRIVATE_REPOSITORIES")
	mirrorIssues := readBoolean("MIRROR_ISSUES")
	mirrorStarred := readBoolean("MIRROR_STARRED")
	mirrorOrganizations := readBoolean("MIRROR_ORGANIZATIONS")
	singleRepo := readEnv("SINGLE_REPO")

	// Validate GitHub token requirements
	if privateRepositories && githubToken == "" {
		return nil, fmt.Errorf("invalid configuration, mirroring private repositories requires setting GITHUB_TOKEN")
	}

	if (mirrorIssues || mirrorStarred || mirrorOrganizations || singleRepo != "") && githubToken == "" {
		return nil, fmt.Errorf("invalid configuration, mirroring issues, starred repositories, organizations, or a single repo requires setting GITHUB_TOKEN")
	}

	includeStr := readEnv("INCLUDE")
	if includeStr == "" {
		includeStr = defaultInclude
	}

	excludeStr := readEnv("EXCLUDE")
	if excludeStr == "" {
		excludeStr = defaultExclude
	}

	starredOrg := readEnv("GITEA_STARRED_ORGANIZATION")
	if starredOrg == "" {
		starredOrg = "github"
	}

	visibility := readEnv("GITEA_ORG_VISIBILITY")
	if visibility == "" {
		visibility = "public"
	}

	config := &Config{
		GitHub: GitHubConfig{
			Username:             githubUsername,
			Token:                githubToken,
			SkipForks:            readBoolean("SKIP_FORKS"),
			PrivateRepositories:  privateRepositories,
			MirrorIssues:         mirrorIssues,
			MirrorStarred:        mirrorStarred,
			MirrorOrganizations:  mirrorOrganizations,
			UseSpecificUser:      readBoolean("USE_SPECIFIC_USER"),
			SingleRepo:           singleRepo,
			IncludeOrgs:          splitAndTrim(readEnv("INCLUDE_ORGS")),
			ExcludeOrgs:          splitAndTrim(readEnv("EXCLUDE_ORGS")),
			PreserveOrgStructure: readBoolean("PRESERVE_ORG_STRUCTURE"),
			SkipStarredIssues:    readBoolean("SKIP_STARRED_ISSUES"),
		},
		Gitea: GiteaConfig{
			URL:             giteaURL,
			Token:           giteaToken,
			Organization:    readEnv("GITEA_ORGANIZATION"),
			Visibility:      visibility,
			StarredReposOrg: starredOrg,
		},
		DryRun:    readBoolean("DRY_RUN"),
		Delay:     readInt("DELAY", defaultDelay),
		Include:   splitAndTrim(includeStr),
		Exclude:   splitAndTrim(excludeStr),
		SingleRun: readBoolean("SINGLE_RUN"),
	}

	return config, nil
}
