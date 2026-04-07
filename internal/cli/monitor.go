package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

const defaultMonitoredReposFile = "monitored-repos.yaml"

// MonitoredRepo represents a repository entry in the monitored-repos.yaml config.
type MonitoredRepo struct {
	URL      string `yaml:"url"`
	Language string `yaml:"language"`
	Added    string `yaml:"added"`
	Status   string `yaml:"status"`
}

// MonitoredReposConfig is the top-level structure of monitored-repos.yaml.
type MonitoredReposConfig struct {
	Repos []MonitoredRepo `yaml:"repos"`
}

// AddRepoIssue represents a GitHub issue requesting repo monitoring.
type AddRepoIssue struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	Body   string `json:"body"`
}

var (
	monitorConfigFile string
	monitorLanguage   string
	monitorGHRepo     string
)

var monitorCmd = &cobra.Command{
	Use:   "monitor",
	Short: "Manage monitored repositories",
	Long: `Commands for managing the list of repositories scanned nightly by archlint.

The monitored repository list is stored in monitored-repos.yaml at the repo root.
Use subcommands to list, add, remove repos, run scans, or check GitHub for add-repo requests.`,
}

var monitorListCmd = &cobra.Command{
	Use:   "list",
	Short: "Show monitored repositories from config",
	RunE:  runMonitorList,
}

var monitorAddCmd = &cobra.Command{
	Use:   "add <repo-url>",
	Short: "Add a repository to the monitored list",
	Args:  cobra.ExactArgs(1),
	RunE:  runMonitorAdd,
}

var monitorRemoveCmd = &cobra.Command{
	Use:   "remove <repo-url>",
	Short: "Remove a repository from the monitored list",
	Args:  cobra.ExactArgs(1),
	RunE:  runMonitorRemove,
}

var monitorScanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan all monitored repositories",
	Long: `Scan all active monitored repositories using archlint-rs.
This is intended to be run nightly via cron or CI.`,
	RunE: runMonitorScan,
}

var monitorCheckIssuesCmd = &cobra.Command{
	Use:   "check-issues",
	Short: "Check GitHub for new add-repo issues awaiting approval",
	Long: `Fetches open GitHub issues with the 'add-repo' label and prints pending requests.
Uses the 'gh' CLI tool. Requires authentication via 'gh auth login'.

Example output:
  Issue #42: [Add repo] https://github.com/example/myrepo
  Body: Please monitor this repo.
  Parsed URL: https://github.com/example/myrepo`,
	RunE: runMonitorCheckIssues,
}

func init() {
	monitorCmd.PersistentFlags().StringVar(&monitorConfigFile, "config", defaultMonitoredReposFile, "Path to monitored-repos.yaml")
	monitorAddCmd.Flags().StringVar(&monitorLanguage, "language", "Go", "Programming language of the repository")
	monitorCheckIssuesCmd.Flags().StringVar(&monitorGHRepo, "repo", "mshogin/archlint", "GitHub repository to query for issues (owner/name)")

	monitorCmd.AddCommand(monitorListCmd)
	monitorCmd.AddCommand(monitorAddCmd)
	monitorCmd.AddCommand(monitorRemoveCmd)
	monitorCmd.AddCommand(monitorScanCmd)
	monitorCmd.AddCommand(monitorCheckIssuesCmd)

	rootCmd.AddCommand(monitorCmd)
}

// loadMonitoredRepos reads the monitored-repos.yaml file.
func loadMonitoredRepos(configPath string) (MonitoredReposConfig, error) {
	var cfg MonitoredReposConfig

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Return empty config if file doesn't exist yet.
			return cfg, nil
		}
		return cfg, fmt.Errorf("failed to read config %s: %w", configPath, err)
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("failed to parse config %s: %w", configPath, err)
	}

	return cfg, nil
}

// saveMonitoredRepos writes the config back to disk.
func saveMonitoredRepos(configPath string, cfg MonitoredReposConfig) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to serialize config: %w", err)
	}

	header := []byte("# Monitored repositories - scanned nightly by archlint\n")
	if err := os.WriteFile(configPath, append(header, data...), 0o644); err != nil {
		return fmt.Errorf("failed to write config %s: %w", configPath, err)
	}

	return nil
}

// resolveConfigPath returns the config path, searching repo root if path is relative.
func resolveConfigPath(configPath string) string {
	if filepath.IsAbs(configPath) {
		return configPath
	}
	// Try the current directory first.
	if _, err := os.Stat(configPath); err == nil {
		return configPath
	}
	return configPath
}

func runMonitorList(_ *cobra.Command, _ []string) error {
	cfgPath := resolveConfigPath(monitorConfigFile)
	cfg, err := loadMonitoredRepos(cfgPath)
	if err != nil {
		return err
	}

	if len(cfg.Repos) == 0 {
		fmt.Println("No monitored repositories configured.")
		fmt.Printf("Config: %s\n", cfgPath)
		return nil
	}

	fmt.Printf("Monitored repositories (%d):\n\n", len(cfg.Repos))
	for i, repo := range cfg.Repos {
		fmt.Printf("  %d. %s\n", i+1, repo.URL)
		fmt.Printf("     language: %s\n", repo.Language)
		fmt.Printf("     added:    %s\n", repo.Added)
		fmt.Printf("     status:   %s\n", repo.Status)
		fmt.Println()
	}

	fmt.Printf("Config: %s\n", cfgPath)
	return nil
}

func runMonitorAdd(_ *cobra.Command, args []string) error {
	repoURL := strings.TrimSpace(args[0])
	if repoURL == "" {
		return fmt.Errorf("repo URL cannot be empty")
	}

	cfgPath := resolveConfigPath(monitorConfigFile)
	cfg, err := loadMonitoredRepos(cfgPath)
	if err != nil {
		return err
	}

	// Check for duplicates.
	for _, r := range cfg.Repos {
		if r.URL == repoURL {
			return fmt.Errorf("repository %s is already monitored", repoURL)
		}
	}

	repo := MonitoredRepo{
		URL:      repoURL,
		Language: monitorLanguage,
		Added:    time.Now().Format("2006-01-02"),
		Status:   "active",
	}

	cfg.Repos = append(cfg.Repos, repo)

	if err := saveMonitoredRepos(cfgPath, cfg); err != nil {
		return err
	}

	fmt.Printf("Added: %s (language: %s)\n", repoURL, monitorLanguage)
	fmt.Printf("Config updated: %s\n", cfgPath)
	return nil
}

func runMonitorRemove(_ *cobra.Command, args []string) error {
	repoURL := strings.TrimSpace(args[0])
	if repoURL == "" {
		return fmt.Errorf("repo URL cannot be empty")
	}

	cfgPath := resolveConfigPath(monitorConfigFile)
	cfg, err := loadMonitoredRepos(cfgPath)
	if err != nil {
		return err
	}

	originalLen := len(cfg.Repos)
	filtered := cfg.Repos[:0]
	for _, r := range cfg.Repos {
		if r.URL != repoURL {
			filtered = append(filtered, r)
		}
	}

	if len(filtered) == originalLen {
		return fmt.Errorf("repository %s not found in monitored list", repoURL)
	}

	cfg.Repos = filtered

	if err := saveMonitoredRepos(cfgPath, cfg); err != nil {
		return err
	}

	fmt.Printf("Removed: %s\n", repoURL)
	fmt.Printf("Config updated: %s\n", cfgPath)
	return nil
}

func runMonitorScan(_ *cobra.Command, _ []string) error {
	cfgPath := resolveConfigPath(monitorConfigFile)
	cfg, err := loadMonitoredRepos(cfgPath)
	if err != nil {
		return err
	}

	active := make([]MonitoredRepo, 0, len(cfg.Repos))
	for _, r := range cfg.Repos {
		if r.Status == "active" {
			active = append(active, r)
		}
	}

	if len(active) == 0 {
		fmt.Println("No active repositories to scan.")
		return nil
	}

	fmt.Printf("Scanning %d active repository(ies)...\n\n", len(active))

	var scanErrors []string
	for _, repo := range active {
		fmt.Printf("Scanning: %s\n", repo.URL)

		// Use archlint-rs for scanning - invoke as subprocess.
		// archlint-rs binary is located at archlint-rs/target/release/archlint-rs
		// or on PATH. Fall back gracefully if not available.
		cmd := exec.Command("archlint-rs", "scan", repo.URL, "--language", strings.ToLower(repo.Language))
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			msg := fmt.Sprintf("  failed to scan %s: %v", repo.URL, err)
			fmt.Fprintln(os.Stderr, msg)
			scanErrors = append(scanErrors, msg)
		} else {
			fmt.Printf("  done: %s\n", repo.URL)
		}
		fmt.Println()
	}

	if len(scanErrors) > 0 {
		return fmt.Errorf("%d scan(s) failed", len(scanErrors))
	}

	fmt.Println("All scans completed successfully.")
	return nil
}

func runMonitorCheckIssues(_ *cobra.Command, _ []string) error {
	// Use gh CLI to list issues with add-repo label.
	args := []string{
		"issue", "list",
		"--repo", monitorGHRepo,
		"--label", "add-repo",
		"--state", "open",
		"--json", "number,title,body",
	}

	cmd := exec.Command("gh", args...)
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if ok := false; ok {
			_ = exitErr
		}
		return fmt.Errorf("gh CLI error: %w\nMake sure 'gh' is installed and authenticated via 'gh auth login'", err)
	}

	var issues []AddRepoIssue
	if err := json.Unmarshal(out, &issues); err != nil {
		return fmt.Errorf("failed to parse gh output: %w", err)
	}

	if len(issues) == 0 {
		fmt.Printf("No open 'add-repo' issues in %s.\n", monitorGHRepo)
		return nil
	}

	fmt.Printf("Pending add-repo requests in %s (%d):\n\n", monitorGHRepo, len(issues))

	for _, issue := range issues {
		url := parseRepoURLFromBody(issue.Body)
		fmt.Printf("Issue #%d: %s\n", issue.Number, issue.Title)
		if url != "" {
			fmt.Printf("  Parsed URL: %s\n", url)
		} else {
			fmt.Printf("  Body: %s\n", truncate(issue.Body, 200))
		}
		fmt.Println()
	}

	return nil
}

// parseRepoURLFromBody attempts to extract a GitHub repository URL from an issue body.
// It looks for lines containing github.com URLs.
func parseRepoURLFromBody(body string) string {
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "github.com/") {
			// Extract the URL - find the start of https:// or github.com/.
			for _, prefix := range []string{"https://github.com/", "http://github.com/", "github.com/"} {
				idx := strings.Index(line, prefix)
				if idx >= 0 {
					url := line[idx:]
					// Trim any trailing punctuation or whitespace.
					url = strings.FieldsFunc(url, func(r rune) bool {
						return r == ' ' || r == ')' || r == ']' || r == ',' || r == '"' || r == '\''
					})[0]
					if !strings.HasPrefix(url, "https://") && !strings.HasPrefix(url, "http://") {
						url = "https://" + url
					}
					return url
				}
			}
		}
	}
	return ""
}

// truncate returns s truncated to at most maxLen characters.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
