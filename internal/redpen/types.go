package redpen

import (
	"time"
)

// GitHub API raw types
type GhUser struct {
	Login string `json:"login"`
	Type  string `json:"type"` // "User" | "Bot" | "Organization"
}

type RawPR struct {
	Number    int        `json:"number"`
	Title     string     `json:"title"`
	State     string     `json:"state"`
	HTMLURL   string     `json:"html_url"`
	User      GhUser     `json:"user"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	MergedAt  *time.Time `json:"merged_at"`
}

type RawReviewComment struct {
	ID                  int64     `json:"id"`
	PullRequestReviewID int64     `json:"pull_request_review_id"`
	DiffHunk            string    `json:"diff_hunk"`
	Path                string    `json:"path"`
	Line                *int      `json:"line"`
	OriginalLine        *int      `json:"original_line"`
	StartLine           *int      `json:"start_line"`
	OriginalStartLine   *int      `json:"original_start_line"`
	Side                string    `json:"side"`
	Body                string    `json:"body"`
	User                GhUser    `json:"user"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
	HTMLURL             string    `json:"html_url"`
	InReplyToID         *int64    `json:"in_reply_to_id"`
	AuthorAssociation   string    `json:"author_association"`
	SubjectType         string    `json:"subject_type"`
}

type RawReview struct {
	ID          int64     `json:"id"`
	State       string    `json:"state"` // APPROVED | CHANGES_REQUESTED | COMMENTED | DISMISSED
	Body        string    `json:"body"`
	User        GhUser    `json:"user"`
	SubmittedAt time.Time `json:"submitted_at"`
	HTMLURL     string    `json:"html_url"`
}

type RawIssueComment struct {
	ID                int64     `json:"id"`
	Body              string    `json:"body"`
	User              GhUser    `json:"user"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
	HTMLURL           string    `json:"html_url"`
	AuthorAssociation string    `json:"author_association"`
}

// Clean output models

// Suggestion is a parsed ```suggestion block from a review comment body.
type Suggestion struct {
	Text string `json:"text"`
}

// ReviewComment is an inline diff comment on a specific file/line.
type ReviewComment struct {
	ID                  int64        `json:"id"`
	PullRequestReviewID int64        `json:"pull_request_review_id"`
	Path                string       `json:"path"`
	Line                *int         `json:"line,omitempty"`
	OriginalLine        *int         `json:"original_line,omitempty"`
	StartLine           *int         `json:"start_line,omitempty"`
	Side                string       `json:"side,omitempty"`
	DiffHunk            string       `json:"diff_hunk"`
	Body                string       `json:"body"`
	BodyClean           string       `json:"body_clean,omitempty"` // body with suggestion blocks stripped
	HasSuggestion       bool         `json:"has_suggestion"`
	Suggestions         []Suggestion `json:"suggestions,omitempty"`
	Reviewer            string       `json:"reviewer"`
	ReviewerType        string       `json:"reviewer_type"`
	AuthorAssociation   string       `json:"author_association"`
	CreatedAt           time.Time    `json:"created_at"`
	UpdatedAt           time.Time    `json:"updated_at"`
	URL                 string       `json:"url"`
	InReplyToID         *int64       `json:"in_reply_to_id,omitempty"`
	IsReply             bool         `json:"is_reply"`
}

// Review is an overall PR review (APPROVED / CHANGES_REQUESTED / etc.) with an optional body.
type Review struct {
	ID           int64     `json:"id"`
	State        string    `json:"state"`
	Body         string    `json:"body"`
	Reviewer     string    `json:"reviewer"`
	ReviewerType string    `json:"reviewer_type"`
	SubmittedAt  time.Time `json:"submitted_at"`
	URL          string    `json:"url"`
}

// IssueComment is a general discussion comment on the PR thread.
type IssueComment struct {
	ID                int64     `json:"id"`
	Body              string    `json:"body"`
	Author            string    `json:"author"`
	AuthorType        string    `json:"author_type"`
	AuthorAssociation string    `json:"author_association"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
	URL               string    `json:"url"`
}

// PRData is the full aggregated data for one pull request.
type PRData struct {
	Number         int             `json:"number"`
	Title          string          `json:"title"`
	Author         string          `json:"author"`
	AuthorType     string          `json:"author_type"`
	State          string          `json:"state"`
	URL            string          `json:"url"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
	MergedAt       *time.Time      `json:"merged_at,omitempty"`
	FetchedAt      time.Time       `json:"fetched_at"`
	ReviewComments []ReviewComment `json:"review_comments"`
	Reviews        []Review        `json:"reviews"`
	IssueComments  []IssueComment  `json:"issue_comments"`
}

// OutputConfig records what filters were active when output was generated.
type OutputConfig struct {
	PRAuthorFilter string `json:"pr_author_filter"`
	CommentFilter  string `json:"comment_filter"`
}

// OutputStats is a summary count.
type OutputStats struct {
	TotalPRs            int `json:"total_prs"`
	TotalReviewComments int `json:"total_review_comments"`
	TotalReviews        int `json:"total_reviews"`
	TotalIssueComments  int `json:"total_issue_comments"`
}

// Output is the root structure written to comments.json.
type Output struct {
	Repo         string       `json:"repo"`
	GeneratedAt  time.Time    `json:"generated_at"`
	Config       OutputConfig `json:"config"`
	Stats        OutputStats  `json:"stats"`
	PullRequests []PRData     `json:"pull_requests"`
}

// PRStateEntry records when a PR was fetched and what its updated_at was.
type PRStateEntry struct {
	FetchedAt time.Time `json:"fetched_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// State is persisted to .state.json in the output directory.
type State struct {
	LastRun time.Time               `json:"last_run"`
	PRs     map[string]PRStateEntry `json:"prs"` // key = PR number as string
}
