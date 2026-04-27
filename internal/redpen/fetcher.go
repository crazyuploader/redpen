package redpen

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// ListPRs fetches PRs for a repository. When authors is non-empty it uses the
// GitHub Search API to avoid enumerating all PRs in the repo — the old approach
// that triggered rate limits when the repo had thousands of PRs.
func ListPRs(c *GhClient, owner, repo, state string, authors map[string]bool) ([]RawPR, error) {
	if len(authors) > 0 {
		return searchPRsByAuthors(c, owner, repo, state, authors)
	}
	url := fmt.Sprintf("%s/repos/%s/%s/pulls?state=%s&per_page=100&sort=updated&direction=desc",
		apiBase, owner, repo, state)
	return FetchAll[RawPR](c, url)
}

// rawSearchPR maps the GitHub Search API PR item, which buries merged_at inside
// a pull_request sub-object instead of exposing it at the top level.
type rawSearchPR struct {
	Number      int       `json:"number"`
	Title       string    `json:"title"`
	State       string    `json:"state"`
	HTMLURL     string    `json:"html_url"`
	User        GhUser    `json:"user"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	PullRequest struct {
		MergedAt *time.Time `json:"merged_at"`
	} `json:"pull_request"`
}

func (s rawSearchPR) toRawPR() RawPR {
	return RawPR{
		Number:    s.Number,
		Title:     s.Title,
		State:     s.State,
		HTMLURL:   s.HTMLURL,
		User:      s.User,
		CreatedAt: s.CreatedAt,
		UpdatedAt: s.UpdatedAt,
		MergedAt:  s.PullRequest.MergedAt,
	}
}

type searchEnvelope struct {
	TotalCount int           `json:"total_count"`
	Items      []rawSearchPR `json:"items"`
}

// searchPRsByAuthors issues one Search API query per author and merges results.
// The Search API targets only the specified authors — no full repo enumeration needed.
func searchPRsByAuthors(c *GhClient, owner, repo, state string, authors map[string]bool) ([]RawPR, error) {
	stateFilter := ""
	switch state {
	case "open":
		stateFilter = "+is:open"
	case "closed":
		stateFilter = "+is:closed"
	}

	seen := make(map[int]bool)
	var all []RawPR

	for author := range authors {
		q := fmt.Sprintf("is:pr+repo:%s/%s+author:%s%s", owner, repo, author, stateFilter)
		url := fmt.Sprintf("%s/search/issues?q=%s&per_page=100&sort=updated&order=desc", apiBase, q)

		prs, err := fetchSearchPages(c, url)
		if err != nil {
			return nil, fmt.Errorf("search PRs for %s: %w", author, err)
		}
		log.Info().Str("author", author).Int("count", len(prs)).Msg("search API found PRs")

		for _, pr := range prs {
			if !seen[pr.Number] {
				seen[pr.Number] = true
				all = append(all, pr)
			}
		}
	}
	return all, nil
}

func fetchSearchPages(c *GhClient, initialURL string) ([]RawPR, error) {
	var all []RawPR
	url := initialURL
	for url != "" {
		b, link, err := c.get(url)
		if err != nil {
			return nil, err
		}
		var page searchEnvelope
		if err := json.Unmarshal(b, &page); err != nil {
			return nil, fmt.Errorf("unmarshal search %s: %w (body: %.200s)", url, err, b)
		}
		for _, item := range page.Items {
			all = append(all, item.toRawPR())
		}
		url = nextURL(link)
		if url != "" {
			time.Sleep(80 * time.Millisecond)
		}
	}
	return all, nil
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

var suggestionRe = regexp.MustCompile("(?s)```suggestion[^\n]*\n(.*?)\n```")

func extractSuggestions(body string) (suggestions []Suggestion, cleanBody string) {
	matches := suggestionRe.FindAllStringSubmatch(body, -1)
	if len(matches) == 0 {
		return nil, body
	}
	for _, m := range matches {
		suggestions = append(suggestions, Suggestion{Text: strings.TrimRight(m[1], "\r\n")})
	}
	cleanBody = strings.TrimSpace(suggestionRe.ReplaceAllString(body, ""))
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
	if r.Body == "" {
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
		log.Warn().Err(err).Int("pr", pr.Number).Msg("review comments fetch failed")
	}
	for _, rc := range rawComments {
		data.ReviewComments = append(data.ReviewComments, toReviewComment(rc))
	}

	rawReviews, err := listReviews(c, owner, repo, pr.Number)
	if err != nil {
		log.Warn().Err(err).Int("pr", pr.Number).Msg("reviews fetch failed")
	}
	for _, r := range rawReviews {
		if rv, ok := toReview(r); ok {
			data.Reviews = append(data.Reviews, rv)
		}
	}

	rawIssue, err := listIssueComments(c, owner, repo, pr.Number)
	if err != nil {
		log.Warn().Err(err).Int("pr", pr.Number).Msg("issue comments fetch failed")
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

// SplitSetLower is like SplitSet but lowercases all values (for case-insensitive matching).
func SplitSetLower(s string) map[string]bool {
	m := make(map[string]bool)
	for _, v := range strings.Split(s, ",") {
		if t := strings.TrimSpace(v); t != "" {
			m[strings.ToLower(t)] = true
		}
	}
	return m
}

// FilterOptions controls which comments survive ApplyFilters.
type FilterOptions struct {
	// Reviewers keeps only comments from these logins (empty = keep all).
	Reviewers map[string]bool
	// ReviewerTypes keeps only comments whose author type matches (case-insensitive).
	// GitHub values: "user", "bot", "organization".
	ReviewerTypes map[string]bool
}

// ApplyFilters returns a copy of PRData with only comments that pass all filters.
func ApplyFilters(pr PRData, opts FilterOptions) PRData {
	filtered := pr

	filtered.ReviewComments = nil
	for _, c := range pr.ReviewComments {
		if matchReviewer(opts.Reviewers, c.Reviewer) && matchType(opts.ReviewerTypes, c.ReviewerType) {
			filtered.ReviewComments = append(filtered.ReviewComments, c)
		}
	}

	filtered.Reviews = nil
	for _, r := range pr.Reviews {
		if matchReviewer(opts.Reviewers, r.Reviewer) && matchType(opts.ReviewerTypes, r.ReviewerType) {
			filtered.Reviews = append(filtered.Reviews, r)
		}
	}

	filtered.IssueComments = nil
	for _, c := range pr.IssueComments {
		if matchReviewer(opts.Reviewers, c.Author) && matchType(opts.ReviewerTypes, c.AuthorType) {
			filtered.IssueComments = append(filtered.IssueComments, c)
		}
	}

	return filtered
}

// HasAnyComments reports whether pr has at least one comment of any kind.
func HasAnyComments(pr PRData) bool {
	return len(pr.ReviewComments)+len(pr.Reviews)+len(pr.IssueComments) > 0
}

func matchReviewer(filter map[string]bool, login string) bool {
	return len(filter) == 0 || filter[login]
}

func matchType(filter map[string]bool, typ string) bool {
	return len(filter) == 0 || filter[strings.ToLower(typ)]
}

func LoadState(path string) State {
	s := State{PRs: make(map[string]PRStateEntry)}
	data, err := os.ReadFile(path)
	if err != nil {
		return s
	}
	if err := json.Unmarshal(data, &s); err != nil {
		log.Warn().Err(err).Str("path", path).Msg("state file corrupt, starting fresh")
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
