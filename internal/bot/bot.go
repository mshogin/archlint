// Package bot implements the scan-by-issue GitHub bot.
// It polls GitHub issues with "scan: owner/repo" titles,
// runs archlint on the target repo, and posts results as comments.
package bot

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"
)

const (
	// ScanPrefix is the required issue title prefix.
	ScanPrefix = "scan:"
	// PollInterval is how often to check for new issues.
	PollInterval = 60 * time.Second
)

// Config holds bot configuration.
type Config struct {
	// Owner is the GitHub repository owner (e.g. "mshogin").
	Owner string
	// Repo is the GitHub repository name (e.g. "archlint").
	Repo string
	// Token is the GitHub personal access token.
	Token string
	// PollInterval overrides the default poll interval.
	PollInterval time.Duration
	// ScanTimeout is the max duration for a single scan (default 60s).
	ScanTimeout time.Duration
}

// Bot polls GitHub issues and runs scans.
type Bot struct {
	cfg     Config
	github  GitHubClient
	scanner Scanner
}

// New creates a new Bot with the provided config.
func New(cfg Config, github GitHubClient, scanner Scanner) *Bot {
	if cfg.PollInterval == 0 {
		cfg.PollInterval = PollInterval
	}
	if cfg.ScanTimeout == 0 {
		cfg.ScanTimeout = 60 * time.Second
	}
	return &Bot{cfg: cfg, github: github, scanner: scanner}
}

// Run starts the bot poll loop. It blocks until ctx is cancelled.
func (b *Bot) Run(ctx context.Context) error {
	log.Printf("bot: starting poll loop for %s/%s (interval: %s)",
		b.cfg.Owner, b.cfg.Repo, b.cfg.PollInterval)

	ticker := time.NewTicker(b.cfg.PollInterval)
	defer ticker.Stop()

	// Run immediately on start.
	if err := b.poll(ctx); err != nil {
		log.Printf("bot: poll error: %v", err)
	}

	for {
		select {
		case <-ctx.Done():
			log.Println("bot: context cancelled, stopping")
			return ctx.Err()
		case <-ticker.C:
			if err := b.poll(ctx); err != nil {
				log.Printf("bot: poll error: %v", err)
			}
		}
	}
}

// poll fetches open issues and processes any with the scan prefix.
func (b *Bot) poll(ctx context.Context) error {
	issues, err := b.github.ListOpenIssues(ctx, b.cfg.Owner, b.cfg.Repo)
	if err != nil {
		return fmt.Errorf("list issues: %w", err)
	}

	for _, issue := range issues {
		if !isScanIssue(issue.Title) {
			continue
		}
		if err := b.process(ctx, issue); err != nil {
			log.Printf("bot: issue #%d process error: %v", issue.Number, err)
		}
	}
	return nil
}

// isScanIssue returns true if the title starts with "scan:" (case-insensitive).
func isScanIssue(title string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(title)), ScanPrefix)
}

// ParseScanTarget extracts "owner/repo" from a "scan: owner/repo" title.
// Returns an error if the format is invalid.
func ParseScanTarget(title string) (owner, repo string, err error) {
	rest := strings.TrimSpace(title[len(ScanPrefix):])
	parts := strings.SplitN(rest, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid scan target %q: expected owner/repo", rest)
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), nil
}

// process handles a single scan issue end-to-end:
// parse target -> scan -> post comment -> close issue.
func (b *Bot) process(ctx context.Context, issue Issue) error {
	log.Printf("bot: processing issue #%d: %q", issue.Number, issue.Title)

	owner, repo, err := ParseScanTarget(issue.Title)
	if err != nil {
		comment := fmt.Sprintf(
			"**archlint bot**: could not parse scan target from title.\n\nExpected format: `scan: owner/repo`\n\nGot: `%s`",
			issue.Title,
		)
		_ = b.github.PostComment(ctx, b.cfg.Owner, b.cfg.Repo, issue.Number, comment)
		_ = b.github.CloseIssue(ctx, b.cfg.Owner, b.cfg.Repo, issue.Number)
		return fmt.Errorf("parse target: %w", err)
	}

	targetURL := fmt.Sprintf("https://github.com/%s/%s", owner, repo)

	scanCtx, cancel := context.WithTimeout(ctx, b.cfg.ScanTimeout)
	defer cancel()

	result, err := b.scanner.Scan(scanCtx, targetURL)
	if err != nil {
		comment := FormatErrorComment(owner, repo, err)
		_ = b.github.PostComment(ctx, b.cfg.Owner, b.cfg.Repo, issue.Number, comment)
		_ = b.github.CloseIssue(ctx, b.cfg.Owner, b.cfg.Repo, issue.Number)
		return fmt.Errorf("scan %s/%s: %w", owner, repo, err)
	}

	comment := FormatResultComment(owner, repo, result)
	if err := b.github.PostComment(ctx, b.cfg.Owner, b.cfg.Repo, issue.Number, comment); err != nil {
		return fmt.Errorf("post comment on #%d: %w", issue.Number, err)
	}

	if err := b.github.CloseIssue(ctx, b.cfg.Owner, b.cfg.Repo, issue.Number); err != nil {
		return fmt.Errorf("close issue #%d: %w", issue.Number, err)
	}

	log.Printf("bot: done #%d: %s/%s -> %d violations", issue.Number, owner, repo, result.TotalViolations)
	return nil
}
