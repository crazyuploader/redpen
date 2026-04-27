package redpen

import (
	"fmt"
	"strconv"
	"strings"
)

// quoteBlock wraps multi-line text in markdown blockquote style.
func quoteBlock(text string) string {
	lines := strings.Split(text, "\n")
	for i, l := range lines {
		lines[i] = "> " + l
	}
	return strings.Join(lines, "\n")
}

// truncateDiffHunk keeps at most maxLines of a diff hunk for readability.
func truncateDiffHunk(hunk string, maxLines int) string {
	lines := strings.Split(hunk, "\n")
	if len(lines) <= maxLines {
		return hunk
	}
	return strings.Join(lines[:maxLines], "\n") + "\n... [diff truncated — showing first " + strconv.Itoa(maxLines) + " lines]"
}

// GenerateMarkdown generates an AI-friendly markdown guide from the output.
func GenerateMarkdown(output Output) string {
	var b strings.Builder

	w := func(s string) { b.WriteString(s) }
	wf := func(f string, args ...any) { fmt.Fprintf(&b, f, args...) }

	w("# PR Review Comments\n\n")
	wf("**Repo:** `%s`  \n", output.Repo)
	wf("**Generated:** %s  \n", output.GeneratedAt.Format("2006-01-02 15:04:05 UTC"))
	if output.Config.PRAuthorFilter != "" {
		wf("**PR Author Filter:** `%s`  \n", output.Config.PRAuthorFilter)
	}
	if output.Config.CommentFilter != "" {
		wf("**Comment Filter:** `%s`  \n", output.Config.CommentFilter)
	}
	wf("\n**Stats:**\n")
	wf("- Pull Requests: **%d**\n", output.Stats.TotalPRs)
	wf("- Inline Review Comments: **%d**\n", output.Stats.TotalReviewComments)
	wf("- Overall Reviews (with body): **%d**\n", output.Stats.TotalReviews)
	wf("- Discussion Comments: **%d**\n\n", output.Stats.TotalIssueComments)
	w("---\n\n")

	for _, pr := range output.PullRequests {
		wf("## PR #%d — %s\n\n", pr.Number, pr.Title)
		wf("| Field | Value |\n|---|---|\n")
		wf("| **Author** | `%s` |\n", pr.Author)
		wf("| **State** | %s |\n", pr.State)
		if pr.MergedAt != nil {
			wf("| **Merged** | %s |\n", pr.MergedAt.Format("2006-01-02"))
		}
		wf("| **Created** | %s |\n", pr.CreatedAt.Format("2006-01-02"))
		wf("| **URL** | %s |\n\n", pr.URL)

		// ── Inline review comments ──
		if len(pr.ReviewComments) > 0 {
			w("### 💬 Inline Review Comments\n\n")
			for i, c := range pr.ReviewComments {
				wf("#### Comment #%d\n\n", i+1)

				if c.IsReply {
					wf("> ↩ **Thread reply** to comment ID `%d`\n\n", *c.InReplyToID)
				}

				wf("**Reviewer:** `%s`", c.Reviewer)
				if c.AuthorAssociation != "" {
					wf(" (%s)", c.AuthorAssociation)
				}
				w("  \n")
				wf("**File:** `%s`  \n", c.Path)

				if c.Line != nil {
					wf("**Line:** %d  \n", *c.Line)
				} else if c.OriginalLine != nil {
					wf("**Original Line:** %d _(outdated/resolved)_  \n", *c.OriginalLine)
				}
				if c.StartLine != nil {
					wf("**Start Line:** %d  \n", *c.StartLine)
				}
				wf("**Date:** %s  \n", c.CreatedAt.Format("2006-01-02"))
				wf("**URL:** %s  \n\n", c.URL)

				// Diff hunk
				w("**Code context:**\n\n")
				w("```diff\n")
				w(truncateDiffHunk(c.DiffHunk, 25))
				w("\n```\n\n")

				// Comment body
				w("**Reviewer comment:**\n\n")
				bodyToShow := c.Body
				if c.HasSuggestion && c.BodyClean != "" {
					bodyToShow = c.BodyClean
				}
				w(quoteBlock(bodyToShow))
				w("\n\n")

				// Suggestions
				if c.HasSuggestion {
					w("**Suggested change:**\n\n")
					for _, s := range c.Suggestions {
						w("```suggestion\n")
						w(s.Text)
						w("\n```\n\n")
					}
				}

				w("---\n\n")
			}
		}

		// ── Overall reviews ──
		if len(pr.Reviews) > 0 {
			w("### 🔍 Overall Reviews\n\n")
			for _, r := range pr.Reviews {
				wf("**Reviewer:** `%s` | **State:** `%s` | **Date:** %s  \n",
					r.Reviewer, r.State, r.SubmittedAt.Format("2006-01-02"))
				wf("**URL:** %s  \n\n", r.URL)
				if r.Body != "" {
					w(quoteBlock(r.Body))
					w("\n\n")
				}
				w("---\n\n")
			}
		}

		// ── Discussion comments ──
		if len(pr.IssueComments) > 0 {
			w("### 🗣️ PR Discussion\n\n")
			for _, c := range pr.IssueComments {
				wf("**Author:** `%s` (%s) | **Date:** %s  \n",
					c.Author, c.AuthorAssociation, c.CreatedAt.Format("2006-01-02"))
				wf("**URL:** %s  \n\n", c.URL)
				w(quoteBlock(c.Body))
				w("\n\n")
				w("---\n\n")
			}
		}

		w("\n")
	}

	return b.String()
}
