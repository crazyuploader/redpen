package redpen

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// ErrGhNotInstalled means the gh CLI is not on the host PATH.
var ErrGhNotInstalled = errors.New("gh CLI not installed")

// GhCLIToken returns a token from the GitHub CLI (`gh auth token`).
// The token is the one gh already holds for the logged-in account, so it
// works for private repos the account can access.
func GhCLIToken() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	path, err := exec.LookPath("gh")
	if err != nil {
		return "", ErrGhNotInstalled
	}
	out, err := exec.CommandContext(ctx, path, "auth", "token").Output()
	if err != nil {
		return "", fmt.Errorf("gh auth token failed (run `gh auth login`): %w", err)
	}
	tok := strings.TrimSpace(string(out))
	if tok == "" {
		return "", fmt.Errorf("gh auth token returned empty token")
	}
	return tok, nil
}
