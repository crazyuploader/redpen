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
# Fetch PRs by a specific author (uses GitHub Search API — no full repo enumeration)
redpen fetch --repo owner/repo --pr-author username --token $GITHUB_TOKEN

# Filter comments to a specific reviewer, skip PRs with no matching comments
redpen fetch --repo owner/repo --pr-author username \
  --comment-filter reviewer1,reviewer2 --skip-empty

# Only human reviewer comments (exclude bots)
redpen fetch --repo owner/repo --reviewer-type user --skip-empty

# Re-fetch everything, ignoring cache
redpen fetch --repo owner/repo --force

# Limit to 50 PRs
redpen fetch --repo owner/repo --limit 50
```

## Flags

| Flag | Default | Description |
|---|---|---|
| `--repo` | required | `owner/repo` |
| `--token` | `$GITHUB_TOKEN` | GitHub PAT |
| `--pr-author` | all | Comma-separated PR author logins |
| `--comment-filter` | all | Keep only comments from these reviewer logins |
| `--reviewer-type` | all | Keep only comments by type: `user`, `bot`, `organization` |
| `--skip-empty` | false | Skip PRs with zero matching comments after all filters |
| `--state` | `all` | `open`, `closed`, `all` |
| `--out` | `./pr-reviews` | Output directory |
| `--limit` | `0` (unlimited) | Max PRs to process |
| `--parallelism` | `4` | Concurrent PR fetches |
| `--force` | false | Ignore cache |
| `--log-level` | `info` | `debug`, `info`, `warn`, `error` |

## Config file

```yaml
# config.yaml  (never commit this — it's in .gitignore)
token: ghp_...
repo: owner/repo
pr-author: username
```

## Output

```
pr-reviews/
  comments.json       # structured PR + comment data
  dont-do-list.md     # human-readable review guide
  .state.json         # fetch cache state
  cache/              # per-PR JSON cache
```

## Development

```sh
# Auto-reload on file changes (requires air)
air

# Run tests
make test

# Lint
make lint
```
