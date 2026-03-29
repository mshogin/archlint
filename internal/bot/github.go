package bot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Issue represents a GitHub issue.
type Issue struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	State  string `json:"state"`
}

// GitHubClient abstracts GitHub API operations used by the bot.
type GitHubClient interface {
	// ListOpenIssues returns all open issues for owner/repo.
	ListOpenIssues(ctx context.Context, owner, repo string) ([]Issue, error)
	// PostComment adds a comment to the given issue number.
	PostComment(ctx context.Context, owner, repo string, number int, body string) error
	// CloseIssue closes the given issue.
	CloseIssue(ctx context.Context, owner, repo string, number int) error
}

// HTTPGitHubClient is the real GitHub API client using the REST API.
type HTTPGitHubClient struct {
	token      string
	httpClient *http.Client
	baseURL    string // overridable for tests
}

// NewHTTPGitHubClient creates a production GitHub API client.
func NewHTTPGitHubClient(token string) *HTTPGitHubClient {
	return &HTTPGitHubClient{
		token:      token,
		baseURL:    "https://api.github.com",
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *HTTPGitHubClient) do(ctx context.Context, method, url string, body interface{}) (*http.Response, error) {
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			return nil, err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, url, &buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	return c.httpClient.Do(req)
}

// ListOpenIssues fetches open issues for owner/repo, filtering out pull requests.
func (c *HTTPGitHubClient) ListOpenIssues(ctx context.Context, owner, repo string) ([]Issue, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/issues?state=open&per_page=100", c.baseURL, owner, repo)
	resp, err := c.do(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github list issues: status %d", resp.StatusCode)
	}

	// GitHub returns PRs in the issues endpoint; they have a "pull_request" field.
	// We decode into a generic slice to filter them.
	var raw []struct {
		Number      int    `json:"number"`
		Title       string `json:"title"`
		State       string `json:"state"`
		PullRequest *struct {
			URL string `json:"url"`
		} `json:"pull_request,omitempty"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode issues: %w", err)
	}

	var issues []Issue
	for _, r := range raw {
		if r.PullRequest != nil {
			continue // skip PRs
		}
		issues = append(issues, Issue{
			Number: r.Number,
			Title:  r.Title,
			State:  r.State,
		})
	}
	return issues, nil
}

// PostComment posts a comment body on an issue.
func (c *HTTPGitHubClient) PostComment(ctx context.Context, owner, repo string, number int, body string) error {
	url := fmt.Sprintf("%s/repos/%s/%s/issues/%d/comments", c.baseURL, owner, repo, number)
	payload := map[string]string{"body": body}
	resp, err := c.do(ctx, http.MethodPost, url, payload)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("github post comment: status %d", resp.StatusCode)
	}
	return nil
}

// CloseIssue closes an issue by patching its state to "closed".
func (c *HTTPGitHubClient) CloseIssue(ctx context.Context, owner, repo string, number int) error {
	url := fmt.Sprintf("%s/repos/%s/%s/issues/%d", c.baseURL, owner, repo, number)
	payload := map[string]string{"state": "closed"}
	resp, err := c.do(ctx, http.MethodPatch, url, payload)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("github close issue: status %d", resp.StatusCode)
	}
	return nil
}
