package redpen

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// ListPRs fetches all PRs for a repository.
func ListPRs(c *GhClient, owner, repo, state string) ([]RawPR, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/pulls?state=%s&per_page=100&sort=updated&direction=desc",
		apiBase, owner, repo, state)
	return FetchAll[RawPR](c, url)
}

func listReviewComments(c *GhClient, owner, repo string, prNum int) ([]RawReviewComment, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/pulls/%d/comments?per_page=100", apiBase, owner, repo, prNum)
	return FetchAll[RawReviewComment](c, url)
}

func listReviews(c *GhClient, owner, repo string, prNum int) ([]RawReview, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/pulls/%d/reviews?per_page=100", apiBase, owner, repo, prNum)
	return FetchAll[RawReview](c, url)
}

func listIssueComments(c *GhClient, owner, repo string, prNum int) ([]RawIssueComment, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/issues/%d/comments?per_page=100", apiBase, owner, repo, prNum)
	return FetchAll[RawIssueComment](c, url)
}

// suggestionRe matches ```suggestion ... ``` blocks (GitHub's inline suggestion syntax).
var suggestionRe = regexp.MustCompile("(?s)```suggestion[^\n]*\n(.*?)\n```")

func extractSuggestions(body string) (suggestions []Suggestion, cleanBody string) {
	matches := suggestionRe.FindAllStringSubmatch(body, -1)
	if len(matches) == 0 {
		return nil, body
	}
	for _, m := range matches {
		suggestions = append(suggestions, Suggestion{Text: strings.TrimRight(m[1], "\r\n")})
	}
	cleaned := suggestionRe.ReplaceAllString(body, "")
	cleanBody = strings.TrimSpace(cleaned)
	return suggestions, cleanBody
}

func toReviewComment(r RawReviewComment) ReviewComment {
	suggestions, cleanBody := extractSuggestions(r.Body)
	c := ReviewComment{
		ID:                  r.ID,
		PullRequestReviewID: r.PullRequestReviewID,
		Path:                r.Path,
		Line:                r.Line,
		OriginalLine:        r.OriginalLine,
		StartLine:           r.StartLine,
		Side:                r.Side,
		DiffHunk:            r.DiffHunk,
		Body:                r.Body,
		HasSuggestion:       len(suggestions) > 0,
		Suggestions:         suggestions,
		Reviewer:            r.User.Login,
		ReviewerType:        r.User.Type,
		AuthorAssociation:   r.AuthorAssociation,
		CreatedAt:           r.CreatedAt,
		UpdatedAt:           r.UpdatedAt,
		URL:                 r.HTMLURL,
		InReplyToID:         r.InReplyToID,
		IsReply:             r.InReplyToID != nil,
	}
	if len(suggestions) > 0 {
		c.BodyClean = cleanBody
	}
	return c
}

func toReview(r RawReview) (Review, bool) {
	// Skip empty COMMENTED reviews — they're just containers for inline comments
	if r.Body == "" && r.State == "COMMENTED" {
		return Review{}, false
	}
	return Review{
		ID:           r.ID,
		State:        r.State,
		Body:         r.Body,
		Reviewer:     r.User.Login,
		ReviewerType: r.User.Type,
		SubmittedAt:  r.SubmittedAt,
		URL:          r.HTMLURL,
	}, true
}

func toIssueComment(r RawIssueComment) IssueComment {
	return IssueComment{
		ID:                r.ID,
		Body:              r.Body,
		Author:            r.User.Login,
		AuthorType:        r.User.Type,
		AuthorAssociation: r.AuthorAssociation,
		CreatedAt:         r.CreatedAt,
		UpdatedAt:         r.UpdatedAt,
		URL:               r.HTMLURL,
	}
}

// FetchPRData fetches all data for a single PR.
func FetchPRData(c *GhClient, owner, repo string, pr RawPR) (PRData, error) {
	data := PRData{
		Number:     pr.Number,
		Title:      pr.Title,
		Author:     pr.User.Login,
		AuthorType: pr.User.Type,
		State:      pr.State,
		URL:        pr.HTMLURL,
		CreatedAt:  pr.CreatedAt,
		UpdatedAt:  pr.UpdatedAt,
		MergedAt:   pr.MergedAt,
		FetchedAt:  time.Now().UTC(),
	}

	rawComments, err := listReviewComments(c, owner, repo, pr.Number)
	if err != nil {
		log.Printf("  ⚠  PR #%d review comments: %v", pr.Number, err)
	}
	for _, rc := range rawComments {
		data.ReviewComments = append(data.ReviewComments, toReviewComment(rc))
	}

	rawReviews, err := listReviews(c, owner, repo, pr.Number)
	if err != nil {
		log.Printf("  ⚠  PR #%d reviews: %v", pr.Number, err)
	}
	for _, r := range rawReviews {
		if rv, ok := toReview(r); ok {
			data.Reviews = append(data.Reviews, rv)
		}
	}

	rawIssue, err := listIssueComments(c, owner, repo, pr.Number)
	if err != nil {
		log.Printf("  ⚠  PR #%d issue comments: %v", pr.Number, err)
	}
	for _, ic := range rawIssue {
		data.IssueComments = append(data.IssueComments, toIssueComment(ic))
	}

	return data, nil
}

// SplitSet splits a comma-separated string into a map for fast lookup.
func SplitSet(s string) map[string]bool {
	m := make(map[string]bool)
	for _, v := range strings.Split(s, ",") {
		if t := strings.TrimSpace(v); t != "" {
			m[t] = true
		}
	}
	return m
}

// ApplyCommentFilter returns a copy of PRData with comments filtered by reviewer.
func ApplyCommentFilter(pr PRData, commentFilter map[string]bool) PRData {
	if len(commentFilter) == 0 {
		return pr
	}
	filtered := pr
	filtered.ReviewComments = nil
	for _, c := range pr.ReviewComments {
		if commentFilter[c.Reviewer] {
			filtered.ReviewComments = append(filtered.ReviewComments, c)
		}
	}
	filtered.Reviews = nil
	for _, r := range pr.Reviews {
		if commentFilter[r.Reviewer] {
			filtered.Reviews = append(filtered.Reviews, r)
		}
	}
	filtered.IssueComments = nil
	for _, c := range pr.IssueComments {
		if commentFilter[c.Author] {
			filtered.IssueComments = append(filtered.IssueComments, c)
		}
	}
	return filtered
}

// Idempotency and Cache helpers

func LoadState(path string) State {
	s := State{PRs: make(map[string]PRStateEntry)}
	data, err := os.ReadFile(path)
	if err != nil {
		return s
	}
	if err := json.Unmarshal(data, &s); err != nil {
		log.Printf("Warning: could not parse state file %s: %v — starting fresh", path, err)
		return State{PRs: make(map[string]PRStateEntry)}
	}
	return s
}

func SaveState(path string, s State) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func CacheDir(outDir string) string {
	return filepath.Join(outDir, "cache")
}

func CachePath(outDir string, prNum int) string {
	return filepath.Join(CacheDir(outDir), fmt.Sprintf("%d.json", prNum))
}

func SavePRCache(outDir string, pr PRData) error {
	if err := os.MkdirAll(CacheDir(outDir), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(pr, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(CachePath(outDir, pr.Number), data, 0o644)
}

func LoadPRCache(outDir string, prNum int) (PRData, bool) {
	data, err := os.ReadFile(CachePath(outDir, prNum))
	if err != nil {
		return PRData{}, false
	}
	var pr PRData
	if err := json.Unmarshal(data, &pr); err != nil {
		return PRData{}, false
	}
	return pr, true
}
