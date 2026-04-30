package redpen

import (
	"fmt"
	"html"
	"strings"
)

//nolint:staticcheck
func GenerateHTML(output Output) string {
	var b strings.Builder

	b.WriteString(`<!DOCTYPE html>
<html lang="en">
<head>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<title>PR Review Comments — ` + html.EscapeString(output.Repo) + `</title>
	<link rel="preconnect" href="https://fonts.googleapis.com">
	<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
	<link href="https://fonts.googleapis.com/css2?family=JetBrains+Mono:wght@400;500;600&family=Newsreader:ital,opsz,wght@0,6..72,400;0,6..72,500;0,6..72,600;1,6..72,400&display=swap" rel="stylesheet">
	<style>
		:root {
			--bg: #faf9f7;
			--surface: #ffffff;
			--surface-raised: #f5f4f2;
			--border: #e5e3df;
			--text: #1a1918;
			--text-muted: #6b6965;
			--accent: #c94a32;
			--accent-subtle: #fdf4f2;
			--code-bg: #f0eeeb;
			--diff-add: #e6f7ed;
			--diff-del: #fcebe9;
		}

		* { box-sizing: border-box; }

		body {
			font-family: 'Newsreader', Georgia, serif;
			font-size: 17px;
			line-height: 1.65;
			color: var(--text);
			background: var(--bg);
			margin: 0;
			padding: 0;
		}

		.container {
			max-width: 800px;
			margin: 0 auto;
			padding: 48px 24px;
		}

		header {
			margin-bottom: 48px;
			padding-bottom: 32px;
			border-bottom: 1px solid var(--border);
		}

		h1 {
			font-size: 28px;
			font-weight: 600;
			margin: 0 0 8px;
			letter-spacing: -0.02em;
		}

		.meta {
			font-family: 'JetBrains Mono', monospace;
			font-size: 13px;
			color: var(--text-muted);
		}

		.meta a {
			color: var(--accent);
			text-decoration: none;
		}

		.meta a:hover { text-decoration: underline; }

		.stats {
			display: flex;
			gap: 32px;
			margin-top: 24px;
			flex-wrap: wrap;
		}

		.stat {
			display: flex;
			flex-direction: column;
		}

		.stat-value {
			font-family: 'JetBrains Mono', monospace;
			font-size: 24px;
			font-weight: 600;
			color: var(--text);
		}

		.stat-label {
			font-size: 12px;
			text-transform: uppercase;
			letter-spacing: 0.08em;
			color: var(--text-muted);
			margin-top: 2px;
		}

		.filters {
			margin-top: 16px;
			font-size: 14px;
			color: var(--text-muted);
		}

		.search-box {
			width: 100%;
			padding: 12px 16px;
			font-family: 'JetBrains Mono', monospace;
			font-size: 14px;
			border: 1px solid var(--border);
			border-radius: 6px;
			background: var(--surface);
			margin-bottom: 32px;
			outline: none;
			transition: border-color 0.2s;
		}

		.search-box:focus {
			border-color: var(--accent);
		}

		.toc {
			background: var(--surface-raised);
			border-radius: 8px;
			padding: 20px 24px;
			margin-bottom: 32px;
		}

		.toc h2 {
			font-size: 13px;
			text-transform: uppercase;
			letter-spacing: 0.08em;
			color: var(--text-muted);
			margin: 0 0 12px;
		}

		.toc ul {
			margin: 0;
			padding: 0;
			list-style: none;
			display: flex;
			flex-wrap: wrap;
			gap: 8px;
		}

		.toc a {
			display: inline-block;
			font-family: 'JetBrains Mono', monospace;
			font-size: 13px;
			color: var(--text);
			background: var(--surface);
			padding: 4px 10px;
			border-radius: 4px;
			text-decoration: none;
			border: 1px solid var(--border);
		}

		.toc a:hover {
			border-color: var(--accent);
			color: var(--accent);
		}

		.pr {
			margin-bottom: 48px;
			scroll-margin-top: 24px;
		}

		.pr-header {
			display: flex;
			align-items: baseline;
			gap: 12px;
			margin-bottom: 16px;
		}

		.pr-number {
			font-family: 'JetBrains Mono', monospace;
			font-size: 14px;
			color: var(--text-muted);
		}

		.pr-title {
			font-size: 20px;
			font-weight: 600;
			margin: 0;
		}

		.pr-meta {
			font-family: 'JetBrains Mono', monospace;
			font-size: 12px;
			color: var(--text-muted);
			margin-bottom: 24px;
		}

		.section {
			margin-bottom: 24px;
		}

		.section-header {
			font-size: 14px;
			font-weight: 600;
			text-transform: uppercase;
			letter-spacing: 0.05em;
			color: var(--text-muted);
			margin-bottom: 16px;
			padding-bottom: 8px;
			border-bottom: 1px solid var(--border);
		}

		.comment {
			background: var(--surface);
			border: 1px solid var(--border);
			border-radius: 8px;
			padding: 20px;
			margin-bottom: 16px;
		}

		.comment-header {
			display: flex;
			justify-content: space-between;
			align-items: baseline;
			margin-bottom: 12px;
			flex-wrap: wrap;
			gap: 8px;
		}

		.reviewer {
			font-family: 'JetBrains Mono', monospace;
			font-size: 13px;
			font-weight: 500;
		}

		.comment-date {
			font-family: 'JetBrains Mono', monospace;
			font-size: 12px;
			color: var(--text-muted);
		}

		.file-info {
			font-family: 'JetBrains Mono', monospace;
			font-size: 12px;
			background: var(--code-bg);
			padding: 6px 10px;
			border-radius: 4px;
			margin-bottom: 12px;
			display: inline-block;
		}

		.code-context {
			background: var(--code-bg);
			border-radius: 6px;
			padding: 12px;
			font-family: 'JetBrains Mono', monospace;
			font-size: 12px;
			overflow-x: auto;
			margin-bottom: 12px;
			white-space: pre;
		}

		.code-context .diff-add { background: var(--diff-add); }
		.code-context .diff-del { background: var(--diff-del); }

		.comment-body {
			font-size: 16px;
			line-height: 1.6;
		}

		.comment-body p { margin: 0 0 12px; }
		.comment-body p:last-child { margin-bottom: 0; }

		.suggestion {
			background: var(--accent-subtle);
			border: 1px solid var(--accent);
			border-radius: 6px;
			padding: 12px;
			margin-top: 12px;
		}

		.suggestion-label {
			font-family: 'JetBrains Mono', monospace;
			font-size: 11px;
			text-transform: uppercase;
			letter-spacing: 0.05em;
			color: var(--accent);
			margin-bottom: 8px;
		}

		.suggestion pre {
			font-family: 'JetBrains Mono', monospace;
			font-size: 12px;
			margin: 0;
			white-space: pre-wrap;
		}

		.review {
			background: var(--surface);
			border: 1px solid var(--border);
			border-radius: 8px;
			padding: 16px;
			margin-bottom: 12px;
		}

		.review-header {
			display: flex;
			gap: 16px;
			font-family: 'JetBrains Mono', monospace;
			font-size: 12px;
			color: var(--text-muted);
			margin-bottom: 8px;
		}

		.review-body {
			font-size: 15px;
		}

		.discussion-comment {
			padding: 16px;
			margin-bottom: 12px;
			border-left: 3px solid var(--border);
			background: var(--surface-raised);
		}

		.discussion-header {
			font-family: 'JetBrains Mono', monospace;
			font-size: 12px;
			color: var(--text-muted);
			margin-bottom: 8px;
		}

		.hidden { display: none; }
	</style>
</head>
<body>
	<div class="container">
		<header>
			<h1>PR Review Comments</h1>
			<div class="meta">
				<a href="https://github.com/` + html.EscapeString(output.Repo) + `">` + html.EscapeString(output.Repo) + `</a>
				&nbsp;·&nbsp; ` + output.GeneratedAt.Format("Jan 2, 2006") + `
			</div>
			<div class="stats">
				<div class="stat">
					<span class="stat-value">` + fmt.Sprintf("%d", output.Stats.TotalPRs) + `</span>
					<span class="stat-label">Pull Requests</span>
				</div>
				<div class="stat">
					<span class="stat-value">` + fmt.Sprintf("%d", output.Stats.TotalReviewComments) + `</span>
					<span class="stat-label">Inline Comments</span>
				</div>
				<div class="stat">
					<span class="stat-value">` + fmt.Sprintf("%d", output.Stats.TotalReviews) + `</span>
					<span class="stat-label">Reviews</span>
				</div>
				<div class="stat">
					<span class="stat-value">` + fmt.Sprintf("%d", output.Stats.TotalIssueComments) + `</span>
					<span class="stat-label">Discussions</span>
				</div>
			</div>
			`)

	if output.Config.PRAuthorFilter != "" || output.Config.CommentFilter != "" {
		b.WriteString(`<div class="filters">`)
		if output.Config.PRAuthorFilter != "" {
			fmt.Fprintf(&b, "Author filter: <code>%s</code> ", html.EscapeString(output.Config.PRAuthorFilter))
		}
		if output.Config.CommentFilter != "" {
			fmt.Fprintf(&b, "Reviewer filter: <code>%s</code>", html.EscapeString(output.Config.CommentFilter))
		}
		b.WriteString(`</div>`)
	}

	b.WriteString(`</header>

		<input type="text" class="search-box" id="search" placeholder="Search comments...">

		<div class="toc">
			<h2>Pull Requests</h2>
			<ul>`)

	for _, pr := range output.PullRequests {
		fmt.Fprintf(&b, `<li><a href="#pr-%d">#%d</a></li>`, pr.Number, pr.Number)
	}

	b.WriteString(`</ul>
		</div>`)

	for _, pr := range output.PullRequests {
		fmt.Fprintf(&b, `<section class="pr" id="pr-%d" data-title="%s" data-number="%d">`, pr.Number, html.EscapeString(strings.ToLower(pr.Title)), pr.Number)

		fmt.Fprintf(&b, `
			<div class="pr-header">
				<span class="pr-number">#%d</span>
				<h2 class="pr-title">%s</h2>
			</div>
			<div class="pr-meta">
				<a href="%s" target="_blank">%s</a>
				&nbsp;·&nbsp; %s · %s
			</div>`,
			pr.Number,
			html.EscapeString(pr.Title),
			pr.URL, pr.URL,
			html.EscapeString(pr.Author),
			pr.State,
		)

		if len(pr.ReviewComments) > 0 {
			b.WriteString(`<div class="section">
				<div class="section-header">Inline Review Comments</div>`)
			for _, c := range pr.ReviewComments {
				b.WriteString(fmt.Sprintf(`<div class="comment" data-search="%s %s">`, html.EscapeString(strings.ToLower(c.Reviewer)), html.EscapeString(strings.ToLower(c.Body))))
				b.WriteString(fmt.Sprintf(`<div class="comment-header">
					<span class="reviewer">%s</span>
					<span class="comment-date">%s</span>
				</div>`, html.EscapeString(c.Reviewer), c.CreatedAt.Format("Jan 2, 2006")))

				if c.Path != "" {
					path := c.Path
					if c.Line != nil {
						path += fmt.Sprintf(":%d", *c.Line)
					}
					b.WriteString(fmt.Sprintf(`<div class="file-info">%s</div>`, html.EscapeString(path)))
				}

				if c.DiffHunk != "" {
					b.WriteString(fmt.Sprintf(`<pre class="code-context">%s</pre>`, html.EscapeString(truncateDiffHunkForHTML(c.DiffHunk, 15))))
				}

				bodyToShow := c.Body
				if c.HasSuggestion && c.BodyClean != "" {
					bodyToShow = c.BodyClean
				}
				b.WriteString(fmt.Sprintf(`<div class="comment-body">%s</div>`, formatCommentBody(bodyToShow)))

				if c.HasSuggestion {
					b.WriteString(`<div class="suggestion"><div class="suggestion-label">Suggested change</div>`)
					for _, s := range c.Suggestions {
						b.WriteString(fmt.Sprintf(`<pre>%s</pre>`, html.EscapeString(s.Text)))
					}
					b.WriteString(`</div>`)
				}

				b.WriteString(`</div>`)
			}
			b.WriteString(`</div>`)
		}

		if len(pr.Reviews) > 0 {
			b.WriteString(`<div class="section">
				<div class="section-header">Reviews</div>`)
			for _, r := range pr.Reviews {
				b.WriteString(fmt.Sprintf(`<div class="review" data-search="%s %s">`, html.EscapeString(strings.ToLower(r.Reviewer)), html.EscapeString(strings.ToLower(r.Body))))
				b.WriteString(fmt.Sprintf(`<div class="review-header">
					<span>%s</span>
					<span>%s</span>
					<span>%s</span>
				</div>`, html.EscapeString(r.Reviewer), r.State, r.SubmittedAt.Format("Jan 2, 2006")))
				if r.Body != "" {
					b.WriteString(fmt.Sprintf(`<div class="review-body">%s</div>`, formatCommentBody(r.Body)))
				}
				b.WriteString(`</div>`)
			}
			b.WriteString(`</div>`)
		}

		if len(pr.IssueComments) > 0 {
			b.WriteString(`<div class="section">
				<div class="section-header">Discussion</div>`)
			for _, c := range pr.IssueComments {
				b.WriteString(fmt.Sprintf(`<div class="discussion-comment" data-search="%s %s">`, html.EscapeString(strings.ToLower(c.Author)), html.EscapeString(strings.ToLower(c.Body))))
				b.WriteString(fmt.Sprintf(`<div class="discussion-header">%s · %s</div>`, html.EscapeString(c.Author), c.CreatedAt.Format("Jan 2, 2006")))
				b.WriteString(fmt.Sprintf(`<div>%s</div>`, formatCommentBody(c.Body)))
				b.WriteString(`</div>`)
			}
			b.WriteString(`</div>`)
		}

		b.WriteString(`</section>`)
	}

	b.WriteString(`</div>
	<script>
		const search = document.getElementById('search');
		const prs = document.querySelectorAll('.pr');
		const tocLinks = document.querySelectorAll('.toc a');

		function filter() {
			const q = search.value.toLowerCase();
			prs.forEach(pr => {
				const data = pr.dataset.search || '';
				const matches = q === '' || data.includes(q);
				pr.classList.toggle('hidden', !matches);
			});
			tocLinks.forEach(link => {
				const num = link.getAttribute('href').replace('#pr-', '');
				const pr = document.getElementById('pr-' + num);
				const visible = !pr.classList.contains('hidden');
				link.style.display = visible ? '' : 'none';
			});
		}

		search.addEventListener('input', filter);
	</script>
</body>
</html>`)

	return b.String()
}

func truncateDiffHunkForHTML(hunk string, maxLines int) string {
	lines := strings.Split(hunk, "\n")
	if len(lines) <= maxLines {
		return hunk
	}
	return strings.Join(lines[:maxLines], "\n") + "\n... [truncated]"
}

func formatCommentBody(body string) string {
	body = html.EscapeString(body)
	body = strings.ReplaceAll(body, "```", "")
	return strings.ReplaceAll(body, "\n\n", "</p><p>")
}
