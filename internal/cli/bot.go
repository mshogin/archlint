package cli

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mshogin/archlint/internal/bot"
	"github.com/spf13/cobra"
)

var (
	botOwner        string
	botRepo         string
	botToken        string
	botPollInterval time.Duration
	botScanTimeout  time.Duration
	botArchlintBin  string
)

var botCmd = &cobra.Command{
	Use:   "bot",
	Short: "Run the scan-by-issue GitHub bot",
	Long: `Poll GitHub issues with "scan: owner/repo" titles.
For each matching issue the bot:
  1. Clones the target repository
  2. Runs archlint scan
  3. Posts the result as a comment
  4. Closes the issue

Environment variables (override flags):
  GITHUB_TOKEN        GitHub personal access token
  BOT_OWNER           Repository owner
  BOT_REPO            Repository name

Examples:
  archlint bot --owner mshogin --repo archlint --token $GITHUB_TOKEN
  GITHUB_TOKEN=xxx archlint bot --owner mshogin --repo archlint`,
	RunE: runBot,
}

func init() {
	botCmd.Flags().StringVar(&botOwner, "owner", "", "GitHub repository owner")
	botCmd.Flags().StringVar(&botRepo, "repo", "", "GitHub repository name")
	botCmd.Flags().StringVar(&botToken, "token", "", "GitHub personal access token (or GITHUB_TOKEN env)")
	botCmd.Flags().DurationVar(&botPollInterval, "poll-interval", 60*time.Second, "Issue poll interval")
	botCmd.Flags().DurationVar(&botScanTimeout, "scan-timeout", 60*time.Second, "Max scan duration per repo")
	botCmd.Flags().StringVar(&botArchlintBin, "archlint-bin", "", "Path to archlint binary for scan (empty = in-process)")
	rootCmd.AddCommand(botCmd)
}

func runBot(cmd *cobra.Command, args []string) error {
	// Allow environment variable fallbacks.
	if token := os.Getenv("GITHUB_TOKEN"); token != "" && botToken == "" {
		botToken = token
	}
	if owner := os.Getenv("BOT_OWNER"); owner != "" && botOwner == "" {
		botOwner = owner
	}
	if repo := os.Getenv("BOT_REPO"); repo != "" && botRepo == "" {
		botRepo = repo
	}

	if botOwner == "" {
		return fmt.Errorf("--owner (or BOT_OWNER) is required")
	}
	if botRepo == "" {
		return fmt.Errorf("--repo (or BOT_REPO) is required")
	}
	if botToken == "" {
		return fmt.Errorf("--token (or GITHUB_TOKEN) is required")
	}

	cfg := bot.Config{
		Owner:        botOwner,
		Repo:         botRepo,
		Token:        botToken,
		PollInterval: botPollInterval,
		ScanTimeout:  botScanTimeout,
	}

	ghClient := bot.NewHTTPGitHubClient(botToken)
	scanner := bot.NewLocalScanner(botArchlintBin)
	b := bot.New(cfg, ghClient, scanner)

	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := b.Run(ctx); err != nil {
		// Context cancellation is a normal shutdown, not an error.
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("bot error: %w", err)
	}
	return nil
}
