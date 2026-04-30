package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/sync/errgroup"

	"github.com/crazyuploader/redpen/internal/redpen"
)

var fetchCmd = &cobra.Command{
	Use:   "fetch",
	Short: "Fetch PR review comments",
	RunE: func(cmd *cobra.Command, args []string) error {
		token := viper.GetString("token")
		repo := viper.GetString("repo")
		prAuthor := viper.GetString("pr-author")
		commentFilterStr := viper.GetString("comment-filter")
		reviewerTypeStr := viper.GetString("reviewer-type")
		skipEmpty := viper.GetBool("skip-empty")
		stateStr := viper.GetString("state")
		outDir := viper.GetString("out")
		limit := viper.GetInt("limit")
		force := viper.GetBool("force")
		parallelism := viper.GetInt("parallelism")

		if repo == "" {
			return fmt.Errorf("repo is required (use --repo or REDPEN_REPO)")
		}

		parts := strings.SplitN(repo, "/", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return fmt.Errorf("invalid repo %q — expected owner/repo format", repo)
		}
		owner, repoName := parts[0], parts[1]

		if err := os.MkdirAll(outDir, 0o755); err != nil {
			return fmt.Errorf("cannot create output dir %s: %w", outDir, err)
		}

		statePath := filepath.Join(outDir, ".state.json")
		client := redpen.NewGhClient(token)
		prAuthorFilter := redpen.SplitSet(prAuthor)
		filterOpts := redpen.FilterOptions{
			Reviewers:     redpen.SplitSet(commentFilterStr),
			ReviewerTypes: redpen.SplitSetLower(reviewerTypeStr),
		}

		// Log available rate limit budget before any API calls.
		if rl, err := redpen.FetchRateLimit(client); err != nil {
			log.Warn().Err(err).Msg("could not fetch rate limit info")
		} else {
			log.Info().
				Int("core_remaining", rl.Core.Remaining).
				Int("core_limit", rl.Core.Limit).
				Int("search_remaining", rl.Search.Remaining).
				Int("search_limit", rl.Search.Limit).
				Time("core_reset", time.Unix(rl.Core.Reset, 0)).
				Msg("GitHub API rate limit")
		}

		state := redpen.LoadState(statePath)
		if force {
			log.Info().Msg("--force: ignoring cache, re-fetching all PRs")
			state = redpen.State{PRs: make(map[string]redpen.PRStateEntry)}
		}

		// When pr-author is set, the Search API fetches only that author's PRs
		// directly — no full repo enumeration required.
		log.Info().
			Str("repo", fmt.Sprintf("%s/%s", owner, repoName)).
			Str("state", stateStr).
			Strs("authors", mapKeys(prAuthorFilter)).
			Msg("listing PRs")

		allPRs, err := redpen.ListPRs(client, owner, repoName, stateStr, prAuthorFilter)
		if err != nil {
			return fmt.Errorf("failed to list PRs: %w", err)
		}
		log.Info().Int("count", len(allPRs)).Msg("PRs found")

		if limit > 0 && len(allPRs) > limit {
			allPRs = allPRs[:limit]
			log.Info().Int("limit", limit).Msg("capped PR list")
		}

		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		var (
			mu         sync.Mutex
			prDataList []redpen.PRData
		)

		g, ctx := errgroup.WithContext(ctx)
		g.SetLimit(parallelism)

		for i, p := range allPRs {
			g.Go(func() error {
				select {
				case <-ctx.Done():
					return nil
				default:
				}

				key := strconv.Itoa(p.Number)
				prLog := log.With().Int("pr", p.Number).Int("n", i+1).Int("total", len(allPRs)).Logger()

				cached, hasCached := redpen.LoadPRCache(outDir, p.Number)

				mu.Lock()
				stateEntry, inState := state.PRs[key]
				mu.Unlock()

				if !force && hasCached && inState && !p.UpdatedAt.After(stateEntry.UpdatedAt) {
					prLog.Debug().Time("cached_at", stateEntry.FetchedAt).Msg("cache hit")
					mu.Lock()
					prDataList = append(prDataList, cached)
					mu.Unlock()
					return nil
				}

				prLog.Info().Str("title", p.Title).Msg("fetching")
				pd, err := redpen.FetchPRData(client, owner, repoName, p)
				if err != nil {
					prLog.Error().Err(err).Msg("fetch failed, skipping")
					return nil
				}
				if err := redpen.SavePRCache(outDir, pd); err != nil {
					prLog.Warn().Err(err).Msg("cache write failed")
				}

				mu.Lock()
				state.PRs[key] = redpen.PRStateEntry{
					FetchedAt: time.Now().UTC(),
					UpdatedAt: p.UpdatedAt,
				}
				prDataList = append(prDataList, pd)
				mu.Unlock()

				return nil
			})
		}

		if err := g.Wait(); err != nil {
			return fmt.Errorf("fetch workers: %w", err)
		}

		sort.Slice(prDataList, func(i, j int) bool {
			return prDataList[i].Number < prDataList[j].Number
		})

		output := redpen.Output{
			Repo:        repo,
			GeneratedAt: time.Now().UTC(),
			Config: redpen.OutputConfig{
				PRAuthorFilter:     prAuthor,
				CommentFilter:      commentFilterStr,
				ReviewerTypeFilter: reviewerTypeStr,
				SkipEmpty:          skipEmpty,
			},
		}

		skipped := 0
		for _, pr := range prDataList {
			filtered := redpen.ApplyFilters(pr, filterOpts)
			if skipEmpty && !redpen.HasAnyComments(filtered) {
				skipped++
				continue
			}
			output.PullRequests = append(output.PullRequests, filtered)
			output.Stats.TotalReviewComments += len(filtered.ReviewComments)
			output.Stats.TotalReviews += len(filtered.Reviews)
			output.Stats.TotalIssueComments += len(filtered.IssueComments)
		}
		output.Stats.TotalPRs = len(output.PullRequests)
		if skipped > 0 {
			log.Info().Int("skipped", skipped).Msg("PRs skipped (no matching comments)")
		}

		jsonPath := filepath.Join(outDir, "comments.json")
		jsonBytes, err := json.MarshalIndent(output, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal JSON: %w", err)
		}
		if err := os.WriteFile(jsonPath, jsonBytes, 0o644); err != nil {
			return fmt.Errorf("write JSON: %w", err)
		}
		log.Info().Str("path", jsonPath).Msg("JSON written")

		mdPath := filepath.Join(outDir, "dont-do-list.md")
		if err := os.WriteFile(mdPath, []byte(redpen.GenerateMarkdown(output)), 0o644); err != nil {
			return fmt.Errorf("write markdown: %w", err)
		}
		log.Info().Str("path", mdPath).Msg("markdown written")

		htmlPath := filepath.Join(outDir, "report.html")
		if err := os.WriteFile(htmlPath, []byte(redpen.GenerateHTML(output)), 0o644); err != nil {
			return fmt.Errorf("write html: %w", err)
		}
		log.Info().Str("path", htmlPath).Msg("html written")

		state.LastRun = time.Now().UTC()
		if err := redpen.SaveState(statePath, state); err != nil {
			log.Warn().Err(err).Msg("failed to save state")
		} else {
			log.Info().Str("path", statePath).Msg("state saved")
		}

		log.Info().
			Int("prs", output.Stats.TotalPRs).
			Int("inline_comments", output.Stats.TotalReviewComments).
			Int("reviews", output.Stats.TotalReviews).
			Int("discussion_comments", output.Stats.TotalIssueComments).
			Msg("done")

		return nil
	},
}

func mapKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func init() {
	fetchCmd.Flags().String("token", os.Getenv("GITHUB_TOKEN"), "GitHub token")
	fetchCmd.Flags().String("repo", "", "GitHub repository (owner/repo)")
	fetchCmd.Flags().String("pr-author", "", "Filter by PR author (comma-separated)")
	fetchCmd.Flags().String("comment-filter", "", "Filter comments by reviewer login (comma-separated)")
	fetchCmd.Flags().String("reviewer-type", "", "Filter comments by author type: user, bot, organization (comma-separated)")
	fetchCmd.Flags().Bool("skip-empty", false, "Skip PRs with no matching comments after all filters")
	fetchCmd.Flags().String("state", "all", "PR state (open, closed, all)")
	fetchCmd.Flags().String("out", "./pr-reviews", "Output directory")
	fetchCmd.Flags().Int("limit", 0, "Limit number of PRs to fetch (0 = unlimited)")

	mustBindPFlag("token", fetchCmd.Flags().Lookup("token"))
	mustBindPFlag("repo", fetchCmd.Flags().Lookup("repo"))
	mustBindPFlag("pr-author", fetchCmd.Flags().Lookup("pr-author"))
	mustBindPFlag("comment-filter", fetchCmd.Flags().Lookup("comment-filter"))
	mustBindPFlag("reviewer-type", fetchCmd.Flags().Lookup("reviewer-type"))
	mustBindPFlag("skip-empty", fetchCmd.Flags().Lookup("skip-empty"))
	mustBindPFlag("state", fetchCmd.Flags().Lookup("state"))
	mustBindPFlag("out", fetchCmd.Flags().Lookup("out"))
	mustBindPFlag("limit", fetchCmd.Flags().Lookup("limit"))

	rootCmd.AddCommand(fetchCmd)
}
