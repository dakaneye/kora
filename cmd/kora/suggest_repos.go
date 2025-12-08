package main

import (
	"context"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"github.com/dakaneye/kora/internal/auth/github"
	"github.com/dakaneye/kora/internal/config"
)

// Suggest-repos command flags
var (
	suggestDaysFlag int
	suggestTopFlag  int
)

var suggestReposCmd = &cobra.Command{
	Use:   "suggest-repos",
	Short: "Suggest repositories to watch based on your activity",
	Long: `Analyze your recent GitHub activity to suggest repositories you might
want to add to your watched_repos configuration.

The command examines:
  - PRs you've reviewed or commented on
  - Issues you've participated in
  - PRs you've authored

Repositories where you're active but not directly assigned are surfaced
as suggestions. Add suggested repos to your config manually:

  datasources:
    github:
      watched_repos:
        - owner/repo

Examples:
  # Suggest based on last 30 days of activity (default)
  kora suggest-repos

  # Suggest based on last 90 days
  kora suggest-repos --days 90

  # Show top 5 suggestions
  kora suggest-repos --top 5`,
	RunE: runSuggestRepos,
}

func init() {
	suggestReposCmd.Flags().IntVar(&suggestDaysFlag, "days", 30, "number of days of activity to analyze")
	suggestReposCmd.Flags().IntVar(&suggestTopFlag, "top", 10, "number of top suggestions to show")

	rootCmd.AddCommand(suggestReposCmd)
}

// repoActivity tracks activity counts for a repository.
type repoActivity struct {
	Repo            string
	ReviewCount     int
	CommentCount    int
	MentionCount    int
	TotalInteractions int
}

// runSuggestRepos analyzes user's GitHub activity and suggests repos to watch.
func runSuggestRepos(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	// Load configuration
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Authenticate with GitHub
	authProvider := github.NewGitHubAuthProvider("")
	if err := authProvider.Authenticate(ctx); err != nil {
		return fmt.Errorf("github auth: %w", err)
	}

	// Get credential for API calls
	cred, err := authProvider.GetCredential(ctx)
	if err != nil {
		return fmt.Errorf("get credential: %w", err)
	}

	ghCred, ok := cred.(interface {
		ExecuteAPI(ctx context.Context, endpoint string, args ...string) ([]byte, error)
	})
	if !ok {
		return fmt.Errorf("credential does not support API execution")
	}

	// Calculate time window
	since := time.Now().AddDate(0, 0, -suggestDaysFlag)

	fmt.Fprintf(os.Stderr, "Analyzing your GitHub activity from the last %d days...\n", suggestDaysFlag)

	// Get current user's repos that are already watched
	watchedSet := make(map[string]bool)
	for _, repo := range cfg.Datasources.GitHub.WatchedRepos {
		watchedSet[repo] = true
	}

	// Analyze activity using GitHub search
	activity, err := analyzeGitHubActivity(ctx, ghCred, since, cfg.Datasources.GitHub.Orgs)
	if err != nil {
		return fmt.Errorf("analyze activity: %w", err)
	}

	// Filter out already-watched repos and repos where user is primary (author/assignee)
	var suggestions []repoActivity
	for _, ra := range activity {
		if watchedSet[ra.Repo] {
			continue // Already watching
		}
		suggestions = append(suggestions, ra)
	}

	// Sort by total interactions descending
	sort.Slice(suggestions, func(i, j int) bool {
		return suggestions[i].TotalInteractions > suggestions[j].TotalInteractions
	})

	// Limit to top N
	if len(suggestions) > suggestTopFlag {
		suggestions = suggestions[:suggestTopFlag]
	}

	// Display results
	if len(suggestions) == 0 {
		fmt.Println("No new repository suggestions found.")
		fmt.Println("\nYou may already be watching the repos you're most active in,")
		fmt.Println("or you haven't had significant activity in other repos recently.")
		return nil
	}

	fmt.Printf("\nSuggested repositories to watch (based on %d days of activity):\n\n", suggestDaysFlag)
	for i, s := range suggestions {
		fmt.Printf("%2d. %s\n", i+1, s.Repo)
		fmt.Printf("    %d reviews, %d comments, %d mentions (%d total)\n",
			s.ReviewCount, s.CommentCount, s.MentionCount, s.TotalInteractions)
	}

	fmt.Println("\nTo watch these repos, add them to ~/.kora/config.yaml:")
	fmt.Println("\n  datasources:")
	fmt.Println("    github:")
	fmt.Println("      watched_repos:")
	for _, s := range suggestions {
		fmt.Printf("        - %s\n", s.Repo)
	}

	return nil
}

// analyzeGitHubActivity searches for repos where user has been active.
func analyzeGitHubActivity(
	ctx context.Context,
	cred interface {
		ExecuteAPI(ctx context.Context, endpoint string, args ...string) ([]byte, error)
	},
	since time.Time,
	orgs []string,
) ([]repoActivity, error) {
	activityMap := make(map[string]*repoActivity)

	// Helper to increment activity
	addActivity := func(repo string, reviews, comments, mentions int) {
		if ra, exists := activityMap[repo]; exists {
			ra.ReviewCount += reviews
			ra.CommentCount += comments
			ra.MentionCount += mentions
			ra.TotalInteractions = ra.ReviewCount + ra.CommentCount + ra.MentionCount
		} else {
			activityMap[repo] = &repoActivity{
				Repo:            repo,
				ReviewCount:     reviews,
				CommentCount:    comments,
				MentionCount:    mentions,
				TotalInteractions: reviews + comments + mentions,
			}
		}
	}

	// Build org filter if needed
	orgFilter := ""
	if len(orgs) > 0 {
		for _, org := range orgs {
			orgFilter += fmt.Sprintf(" org:%s", org)
		}
	}

	sinceDate := since.Format("2006-01-02")

	// Search for PRs where user reviewed
	reviewQuery := fmt.Sprintf("reviewed-by:@me type:pr updated:>=%s%s", sinceDate, orgFilter)
	repos, err := searchReposFromQuery(ctx, cred, reviewQuery)
	if err != nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "Warning: review search failed: %v\n", err)
		}
	} else {
		for repo, count := range repos {
			addActivity(repo, count, 0, 0)
		}
	}

	// Search for PRs where user commented
	commentQuery := fmt.Sprintf("commenter:@me type:pr updated:>=%s%s", sinceDate, orgFilter)
	repos, err = searchReposFromQuery(ctx, cred, commentQuery)
	if err != nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "Warning: comment search failed: %v\n", err)
		}
	} else {
		for repo, count := range repos {
			addActivity(repo, 0, count, 0)
		}
	}

	// Search for issues/PRs where user is mentioned
	mentionQuery := fmt.Sprintf("mentions:@me updated:>=%s%s", sinceDate, orgFilter)
	repos, err = searchReposFromQuery(ctx, cred, mentionQuery)
	if err != nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "Warning: mention search failed: %v\n", err)
		}
	} else {
		for repo, count := range repos {
			addActivity(repo, 0, 0, count)
		}
	}

	// Convert map to slice
	result := make([]repoActivity, 0, len(activityMap))
	for _, ra := range activityMap {
		result = append(result, *ra)
	}

	return result, nil
}

// searchReposFromQuery executes a GitHub search and extracts repo counts.
func searchReposFromQuery(
	ctx context.Context,
	cred interface {
		ExecuteAPI(ctx context.Context, endpoint string, args ...string) ([]byte, error)
	},
	query string,
) (map[string]int, error) {
	// Use gh api to search
	args := []string{
		"-X", "GET",
		"-f", "q=" + query,
		"-f", "per_page=100",
	}

	data, err := cred.ExecuteAPI(ctx, "search/issues", args...)
	if err != nil {
		return nil, err
	}

	// Parse response to extract repos
	return parseSearchResultRepos(data)
}

// parseSearchResultRepos extracts repo names and counts from search results.
func parseSearchResultRepos(data []byte) (map[string]int, error) {
	// Simple JSON parsing without external dependencies
	// The response format is: {"items": [{"repository_url": "https://api.github.com/repos/owner/repo", ...}, ...]}
	repos := make(map[string]int)

	// Use simple string scanning for repository_url fields
	// Format: "repository_url":"https://api.github.com/repos/owner/repo"
	dataStr := string(data)
	searchPos := 0
	for {
		// Find next repository_url
		idx := indexOf(dataStr[searchPos:], `"repository_url":"https://api.github.com/repos/`)
		if idx == -1 {
			break
		}
		searchPos += idx + len(`"repository_url":"https://api.github.com/repos/`)

		// Extract owner/repo
		endIdx := indexOf(dataStr[searchPos:], `"`)
		if endIdx == -1 {
			break
		}
		repo := dataStr[searchPos : searchPos+endIdx]
		repos[repo]++
		searchPos += endIdx
	}

	return repos, nil
}

// indexOf returns the index of substr in s, or -1 if not found.
func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
