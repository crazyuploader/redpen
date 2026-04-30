# redpen

Fetch GitHub PR review comments into structured JSON and Markdown.

## Install

```sh
go install github.com/crazyuploader/redpen/cmd/redpen@latest
```

Or build from source:

```sh
make build
```

## Usage

```sh
# Fetch one author's PRs via Search API (no full repo enumeration)
redpen fetch --repo owner/repo --pr-author username --token $GITHUB_TOKEN

# Keep only comments from specific reviewers; drop PRs with no matches
redpen fetch --repo owner/repo --pr-author username \
  --comment-filter reviewer1,reviewer2 --skip-empty

# Human reviewer comments only, no bots
redpen fetch --repo owner/repo --reviewer-type user --skip-empty

# Ignore cache, re-fetch everything
redpen fetch --repo owner/repo --force

# Cap at 50 PRs
redpen fetch --repo owner/repo --limit 50
```

## Flags

| Flag               | Default         | Description                                          |
| ------------------ | --------------- | ---------------------------------------------------- |
| `--repo`           | required        | `owner/repo`                                         |
| `--token`          | `$GITHUB_TOKEN` | GitHub PAT                                           |
| `--pr-author`      | all             | PR author logins, comma-separated                    |
| `--comment-filter` | all             | Keep comments from these reviewer logins only        |
| `--reviewer-type`  | all             | Keep comments by type: `user`, `bot`, `organization` |
| `--skip-empty`     | false           | Drop PRs with zero matching comments after filtering |
| `--state`          | `all`           | `open`, `closed`, `all`                              |
| `--out`            | `./pr-reviews`  | Output directory                                     |
| `--limit`          | `0` (unlimited) | Max PRs to process                                   |
| `--parallelism`    | `4`             | Concurrent PR fetches                                |
| `--force`          | false           | Ignore cache                                         |
| `--log-level`      | `info`          | `debug`, `info`, `warn`, `error`                     |

## Config file

```yaml
# config.yaml  (gitignored — never commit this)
token: ghp_...
repo: owner/repo
pr-author: username
```

Copy `sample-config.yaml` to get started.

## Output

```
pr-reviews/
  comments.json       # structured PR + comment data
  dont-do-list.md     # human-readable review guide
  report.html         # interactive HTML report (searchable, filterable)
  .state.json         # fetch cache state
  cache/              # per-PR JSON cache
```

The HTML report (`report.html`) includes:

- Searchable comments (filter by typing)
- Clickable table of contents for quick navigation
- Syntax-highlighted code context
- Inline suggestions displayed prominently
- Responsive design

## Development

```sh
# Auto-reload on file changes (requires air)
air

# Run tests
make test

# Lint
make lint
```
