package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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
		commentFilter := redpen.SplitSet(commentFilterStr)

		state := redpen.LoadState(statePath)
		if force {
			log.Printf("--force: ignoring cached state, re-fetching all PRs")
			state = redpen.State{PRs: make(map[string]redpen.PRStateEntry)}
		}

		log.Printf("listing PRs in %s/%s (state=%s)...", owner, repoName, stateStr)
		allPRs, err := redpen.ListPRs(client, owner, repoName, stateStr)
		if err != nil {
			return fmt.Errorf("failed to list PRs: %w", err)
		}
		log.Printf("found %d PRs total", len(allPRs))

		if len(prAuthorFilter) > 0 {
			filtered := allPRs[:0]
			for _, p := range allPRs {
				if prAuthorFilter[p.User.Login] {
					filtered = append(filtered, p)
				}
			}
			allPRs = filtered
			log.Printf("after author filter (%s): %d PRs", prAuthor, len(allPRs))
		}

		if limit > 0 && len(allPRs) > limit {
			allPRs = allPRs[:limit]
			log.Printf("limiting to %d PRs", limit)
		}

		// Parallel fetch
		var prDataList []redpen.PRData
		var mu sync.Mutex
		
		// Use a semaphore to limit parallelism
		sem := make(chan struct{}, parallelism)
		var wg sync.WaitGroup

		for i, p := range allPRs {
			wg.Add(1)
			go func(i int, p redpen.RawPR) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				key := strconv.Itoa(p.Number)
				prefix := fmt.Sprintf("[%d/%d] PR #%d", i+1, len(allPRs), p.Number)

				cached, hasCached := redpen.LoadPRCache(outDir, p.Number)
				
				mu.Lock()
				stateEntry, inState := state.PRs[key]
				mu.Unlock()

				needsFetch := true
				if !force && hasCached && inState {
					if !p.UpdatedAt.After(stateEntry.UpdatedAt) {
						log.Printf("%s: using cache (not updated since %s)",
							prefix, stateEntry.FetchedAt.Format("2006-01-02 15:04"))
						
						mu.Lock()
						prDataList = append(prDataList, cached)
						mu.Unlock()
						
						needsFetch = false
					}
				}

				if needsFetch {
					log.Printf("%s: fetching — %s", prefix, p.Title)
					pd, err := redpen.FetchPRData(client, owner, repoName, p)
					if err != nil {
						log.Printf("%s: fetch error: %v — skipping", prefix, err)
						return
					}
					if err := redpen.SavePRCache(outDir, pd); err != nil {
						log.Printf("%s: cache write error: %v", prefix, err)
					}
					
					mu.Lock()
					state.PRs[key] = redpen.PRStateEntry{
						FetchedAt: time.Now().UTC(),
						UpdatedAt: p.UpdatedAt,
					}
					prDataList = append(prDataList, pd)
					mu.Unlock()
					
					// Be polite between PR fetches if not too many workers
					if parallelism <= 2 {
						time.Sleep(250 * time.Millisecond)
					}
				}
			}(i, p)
		}
		wg.Wait()

		// Sort for deterministic output
		sort.Slice(prDataList, func(i, j int) bool {
			return prDataList[i].Number < prDataList[j].Number
		})

		// Build output
		output := redpen.Output{
			Repo:        repo,
			GeneratedAt: time.Now().UTC(),
			Config: redpen.OutputConfig{
				PRAuthorFilter: prAuthor,
				CommentFilter:  commentFilterStr,
			},
		}

		for _, pr := range prDataList {
			filtered := redpen.ApplyCommentFilter(pr, commentFilter)
			output.PullRequests = append(output.PullRequests, filtered)
			output.Stats.TotalReviewComments += len(filtered.ReviewComments)
			output.Stats.TotalReviews += len(filtered.Reviews)
			output.Stats.TotalIssueComments += len(filtered.IssueComments)
		}
		output.Stats.TotalPRs = len(output.PullRequests)

		// Write JSON
		jsonPath := filepath.Join(outDir, "comments.json")
		jsonBytes, err := json.MarshalIndent(output, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal JSON: %w", err)
		}
		if err := os.WriteFile(jsonPath, jsonBytes, 0o644); err != nil {
			return fmt.Errorf("write JSON: %w", err)
		}
		log.Printf("✓ JSON  → %s", jsonPath)

		// Write Markdown
		mdPath := filepath.Join(outDir, "dont-do-list.md")
		if err := os.WriteFile(mdPath, []byte(redpen.GenerateMarkdown(output)), 0o644); err != nil {
			return fmt.Errorf("write markdown: %w", err)
		}
		log.Printf("✓ MD    → %s", mdPath)

		// Save state
		state.LastRun = time.Now().UTC()
		if err := redpen.SaveState(statePath, state); err != nil {
			log.Printf("⚠ failed to save state: %v", err)
		}
		log.Printf("✓ state → %s", statePath)

		log.Printf("done: %d PRs | %d inline comments | %d reviews | %d discussion comments",
			output.Stats.TotalPRs,
			output.Stats.TotalReviewComments,
			output.Stats.TotalReviews,
			output.Stats.TotalIssueComments,
		)

		return nil
	},
}

func init() {
	fetchCmd.Flags().String("token", os.Getenv("GITHUB_TOKEN"), "GitHub token")
	fetchCmd.Flags().String("repo", "", "GitHub repository (owner/repo)")
	fetchCmd.Flags().String("pr-author", "", "Filter by PR author")
	fetchCmd.Flags().String("comment-filter", "", "Filter comments by reviewer")
	fetchCmd.Flags().String("state", "all", "PR state (open, closed, all)")
	fetchCmd.Flags().String("out", "./pr-reviews", "Output directory")
	fetchCmd.Flags().Int("limit", 0, "Limit number of PRs to fetch")

	viper.BindPFlag("token", fetchCmd.Flags().Lookup("token"))
	viper.BindPFlag("repo", fetchCmd.Flags().Lookup("repo"))
	viper.BindPFlag("pr-author", fetchCmd.Flags().Lookup("pr-author"))
	viper.BindPFlag("comment-filter", fetchCmd.Flags().Lookup("comment-filter"))
	viper.BindPFlag("state", fetchCmd.Flags().Lookup("state"))
	viper.BindPFlag("out", fetchCmd.Flags().Lookup("out"))
	viper.BindPFlag("limit", fetchCmd.Flags().Lookup("limit"))

	rootCmd.AddCommand(fetchCmd)
}
