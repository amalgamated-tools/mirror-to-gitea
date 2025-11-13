package logger

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/jaedle/mirror-to-gitea/config"
)

type Logger struct {
	prefix string
}

func New() *Logger {
	return &Logger{prefix: ""}
}

func (l *Logger) Info(msg string, args ...interface{}) {
	timestamp := time.Now().Format(time.RFC3339)
	if len(args) > 0 {
		log.Printf("[%s] INFO: %s %v\n", timestamp, msg, args)
	} else {
		log.Printf("[%s] INFO: %s\n", timestamp, msg)
	}
}

func (l *Logger) Error(msg string, args ...interface{}) {
	timestamp := time.Now().Format(time.RFC3339)
	if len(args) > 0 {
		log.Printf("[%s] ERROR: %s %v\n", timestamp, msg, args)
	} else {
		log.Printf("[%s] ERROR: %s\n", timestamp, msg)
	}
}

func (l *Logger) ShowConfig(cfg *config.Config) {
	// Create a copy of config with redacted tokens
	redactedConfig := struct {
		GitHub struct {
			Username             string   `json:"username"`
			Token                string   `json:"token"`
			SkipForks            bool     `json:"skipForks"`
			PrivateRepositories  bool     `json:"privateRepositories"`
			MirrorIssues         bool     `json:"mirrorIssues"`
			MirrorStarred        bool     `json:"mirrorStarred"`
			MirrorOrganizations  bool     `json:"mirrorOrganizations"`
			UseSpecificUser      bool     `json:"useSpecificUser"`
			SingleRepo           string   `json:"singleRepo"`
			IncludeOrgs          []string `json:"includeOrgs"`
			ExcludeOrgs          []string `json:"excludeOrgs"`
			PreserveOrgStructure bool     `json:"preserveOrgStructure"`
			SkipStarredIssues    bool     `json:"skipStarredIssues"`
		} `json:"github"`
		Gitea struct {
			URL             string `json:"url"`
			Token           string `json:"token"`
			Organization    string `json:"organization"`
			Visibility      string `json:"visibility"`
			StarredReposOrg string `json:"starredReposOrg"`
		} `json:"gitea"`
		DryRun    bool     `json:"dryRun"`
		Delay     int      `json:"delay"`
		Include   []string `json:"include"`
		Exclude   []string `json:"exclude"`
		SingleRun bool     `json:"singleRun"`
	}{}

	redactedConfig.GitHub.Username = cfg.GitHub.Username
	redactedConfig.GitHub.Token = "[REDACTED]"
	redactedConfig.GitHub.SkipForks = cfg.GitHub.SkipForks
	redactedConfig.GitHub.PrivateRepositories = cfg.GitHub.PrivateRepositories
	redactedConfig.GitHub.MirrorIssues = cfg.GitHub.MirrorIssues
	redactedConfig.GitHub.MirrorStarred = cfg.GitHub.MirrorStarred
	redactedConfig.GitHub.MirrorOrganizations = cfg.GitHub.MirrorOrganizations
	redactedConfig.GitHub.UseSpecificUser = cfg.GitHub.UseSpecificUser
	redactedConfig.GitHub.SingleRepo = cfg.GitHub.SingleRepo
	redactedConfig.GitHub.IncludeOrgs = cfg.GitHub.IncludeOrgs
	redactedConfig.GitHub.ExcludeOrgs = cfg.GitHub.ExcludeOrgs
	redactedConfig.GitHub.PreserveOrgStructure = cfg.GitHub.PreserveOrgStructure
	redactedConfig.GitHub.SkipStarredIssues = cfg.GitHub.SkipStarredIssues

	redactedConfig.Gitea.URL = cfg.Gitea.URL
	redactedConfig.Gitea.Token = "[REDACTED]"
	redactedConfig.Gitea.Organization = cfg.Gitea.Organization
	redactedConfig.Gitea.Visibility = cfg.Gitea.Visibility
	redactedConfig.Gitea.StarredReposOrg = cfg.Gitea.StarredReposOrg

	redactedConfig.DryRun = cfg.DryRun
	redactedConfig.Delay = cfg.Delay
	redactedConfig.Include = cfg.Include
	redactedConfig.Exclude = cfg.Exclude
	redactedConfig.SingleRun = cfg.SingleRun

	configJSON, err := json.MarshalIndent(redactedConfig, "", "  ")
	if err != nil {
		l.Error("Failed to marshal config", err)
		return
	}

	fmt.Printf("Applied configuration:\n%s\n", string(configJSON))
}
