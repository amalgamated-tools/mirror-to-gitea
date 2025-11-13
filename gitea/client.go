package gitea

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/google/go-github/v66/github"
	"github.com/jaedle/mirror-to-gitea/config"
	ghrepo "github.com/jaedle/mirror-to-gitea/github"
)

type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

type Target struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"` // "user" or "organization"
}

type User struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
}

type Organization struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
}

type MigrateRepoRequest struct {
	AuthToken string `json:"auth_token,omitempty"`
	CloneAddr string `json:"clone_addr"`
	Mirror    bool   `json:"mirror"`
	RepoName  string `json:"repo_name"`
	UID       int64  `json:"uid"`
	Private   bool   `json:"private"`
}

type Issue struct {
	Title  string `json:"title"`
	Body   string `json:"body"`
	State  string `json:"state"`
	Closed bool   `json:"closed"`
}

type Label struct {
	Name  string `json:"name"`
	Color string `json:"color"`
}

type IssueResponse struct {
	Number int `json:"number"`
}

func NewClient(cfg *config.GiteaConfig) *Client {
	return &Client{
		baseURL: cfg.URL,
		token:   cfg.Token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) doRequest(method, path string, body interface{}) ([]byte, int, error) {
	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, 0, err
		}
		reqBody = bytes.NewBuffer(jsonData)
	}

	url := c.baseURL + path
	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, 0, err
	}

	req.Header.Set("Authorization", "token "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}

	return respBody, resp.StatusCode, nil
}

func (c *Client) GetUser() (*Target, error) {
	respBody, statusCode, err := c.doRequest("GET", "/api/v1/user", nil)
	if err != nil {
		return nil, err
	}

	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get user: status %d", statusCode)
	}

	var user User
	if err := json.Unmarshal(respBody, &user); err != nil {
		return nil, err
	}

	return &Target{
		ID:   user.ID,
		Name: user.Username,
		Type: "user",
	}, nil
}

func (c *Client) GetOrganization(orgName string) (*Target, error) {
	respBody, statusCode, err := c.doRequest("GET", "/api/v1/orgs/"+orgName, nil)
	if err != nil {
		return nil, err
	}

	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get organization %s: status %d", orgName, statusCode)
	}

	var org Organization
	if err := json.Unmarshal(respBody, &org); err != nil {
		return nil, err
	}

	return &Target{
		ID:   org.ID,
		Name: orgName,
		Type: "organization",
	}, nil
}

func (c *Client) CreateOrganization(orgName, visibility string, dryRun bool) error {
	if dryRun {
		log.Printf("DRY RUN: Would create Gitea organization: %s (%s)", orgName, visibility)
		return nil
	}

	// Check if organization already exists
	_, statusCode, _ := c.doRequest("GET", "/api/v1/orgs/"+orgName, nil)
	if statusCode == http.StatusOK {
		log.Printf("Organization %s already exists", orgName)
		return nil
	}

	// Create the organization
	createReq := map[string]interface{}{
		"username":   orgName,
		"visibility": visibility,
	}

	_, statusCode, err := c.doRequest("POST", "/api/v1/orgs", createReq)
	if err != nil {
		return err
	}

	if statusCode == http.StatusCreated || statusCode == http.StatusUnprocessableEntity {
		log.Printf("Created organization: %s", orgName)
		return nil
	}

	return fmt.Errorf("failed to create organization %s: status %d", orgName, statusCode)
}

func (c *Client) IsRepositoryMirrored(repoName string, target *Target) (bool, error) {
	path := fmt.Sprintf("/api/v1/repos/%s/%s", target.Name, repoName)
	_, statusCode, _ := c.doRequest("GET", path, nil)
	return statusCode == http.StatusOK, nil
}

func (c *Client) MirrorRepository(repo *ghrepo.Repository, target *Target, githubToken string) error {
	migrateReq := MigrateRepoRequest{
		AuthToken: githubToken,
		CloneAddr: repo.URL,
		Mirror:    true,
		RepoName:  repo.Name,
		UID:       target.ID,
		Private:   repo.Private,
	}

	_, statusCode, err := c.doRequest("POST", "/api/v1/repos/migrate", migrateReq)
	if err != nil {
		return err
	}

	if statusCode != http.StatusCreated {
		return fmt.Errorf("failed to mirror repository %s: status %d", repo.Name, statusCode)
	}

	log.Printf("Successfully mirrored: %s", repo.Name)
	return nil
}

func (c *Client) StarRepository(repoName string, target *Target, dryRun bool) error {
	if dryRun {
		log.Printf("DRY RUN: Would star repository in Gitea: %s/%s", target.Name, repoName)
		return nil
	}

	path := fmt.Sprintf("/api/v1/user/starred/%s/%s", target.Name, repoName)
	_, statusCode, err := c.doRequest("PUT", path, nil)
	if err != nil {
		return err
	}

	if statusCode != http.StatusNoContent {
		return fmt.Errorf("failed to star repository %s/%s: status %d", target.Name, repoName, statusCode)
	}

	log.Printf("Successfully starred repository in Gitea: %s/%s", target.Name, repoName)
	return nil
}

func (c *Client) MirrorIssues(ctx context.Context, ghClient *github.Client, repo *ghrepo.Repository, target *Target, githubToken string, dryRun bool) error {
	if !repo.HasIssues {
		log.Printf("Repository %s doesn't have issues enabled. Skipping issues mirroring.", repo.Name)
		return nil
	}

	if dryRun {
		log.Printf("DRY RUN: Would mirror issues for repository: %s", repo.Name)
		return nil
	}

	// Fetch issues from GitHub
	issues, err := c.fetchGitHubIssues(ctx, ghClient, repo)
	if err != nil {
		return err
	}

	log.Printf("Found %d issues for %s", len(issues), repo.Name)

	// Create issues one by one to maintain order
	for _, issue := range issues {
		if err := c.createGiteaIssue(issue, repo, target); err != nil {
			log.Printf("Error creating issue '%s': %v", issue.GetTitle(), err)
		}
	}

	log.Printf("Completed mirroring issues for %s", repo.Name)
	return nil
}

func (c *Client) fetchGitHubIssues(ctx context.Context, ghClient *github.Client, repo *ghrepo.Repository) ([]*github.Issue, error) {
	opt := &github.IssueListByRepoOptions{
		State:       "all",
		ListOptions: github.ListOptions{PerPage: 100},
	}

	var allIssues []*github.Issue
	for {
		issues, resp, err := ghClient.Issues.ListByRepo(ctx, repo.Owner, repo.Name, opt)
		if err != nil {
			return nil, fmt.Errorf("error fetching issues for %s/%s: %w", repo.Owner, repo.Name, err)
		}
		allIssues = append(allIssues, issues...)
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}

	return allIssues, nil
}

func (c *Client) createGiteaIssue(issue *github.Issue, repo *ghrepo.Repository, target *Target) error {
	body := fmt.Sprintf("*Originally created by @%s on %s*\n\n%s",
		issue.GetUser().GetLogin(),
		issue.GetCreatedAt().Format("2006-01-02"),
		issue.GetBody())

	giteaIssue := Issue{
		Title:  issue.GetTitle(),
		Body:   body,
		State:  issue.GetState(),
		Closed: issue.GetState() == "closed",
	}

	path := fmt.Sprintf("/api/v1/repos/%s/%s/issues", target.Name, repo.Name)
	respBody, statusCode, err := c.doRequest("POST", path, giteaIssue)
	if err != nil {
		return err
	}

	if statusCode != http.StatusCreated {
		return fmt.Errorf("failed to create issue: status %d", statusCode)
	}

	var issueResp IssueResponse
	if err := json.Unmarshal(respBody, &issueResp); err != nil {
		return err
	}

	log.Printf("Created issue #%d: %s", issueResp.Number, issue.GetTitle())

	// Add labels if the issue has any
	if len(issue.Labels) > 0 {
		for _, label := range issue.Labels {
			c.addLabelToIssue(repo, target, issueResp.Number, label.GetName())
		}
	}

	return nil
}

func (c *Client) addLabelToIssue(repo *ghrepo.Repository, target *Target, issueNumber int, labelName string) {
	// First try to create the label if it doesn't exist
	labelPath := fmt.Sprintf("/api/v1/repos/%s/%s/labels", target.Name, repo.Name)
	label := Label{
		Name:  labelName,
		Color: generateRandomColor(),
	}
	c.doRequest("POST", labelPath, label)

	// Then add the label to the issue
	issueLabelPath := fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d/labels", target.Name, repo.Name, issueNumber)
	labelList := map[string][]string{
		"labels": {labelName},
	}
	if _, statusCode, err := c.doRequest("POST", issueLabelPath, labelList); err != nil || statusCode != http.StatusOK {
		log.Printf("Error adding label %s to issue: %v", labelName, err)
	}
}

func generateRandomColor() string {
	return fmt.Sprintf("%06x", rand.Intn(0xFFFFFF))
}
