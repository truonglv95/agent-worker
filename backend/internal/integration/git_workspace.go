package integration

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// GitWorkspace manages cloning, committing and pushing to a remote git repository
type GitWorkspace struct {
	RepoURL   string
	Token     string
	LocalPath string
}

func NewGitWorkspace(repoURL, token, localPath string) *GitWorkspace {
	return &GitWorkspace{
		RepoURL:   repoURL,
		Token:     token,
		LocalPath: localPath,
	}
}

func (gw *GitWorkspace) execGit(args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = gw.LocalPath
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %v failed: %s (%w)", args, string(out), err)
	}
	return nil
}

func (gw *GitWorkspace) Clone() error {
	if _, err := os.Stat(gw.LocalPath); err == nil {
		// Already exists
		return nil
	}

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(gw.LocalPath), 0755); err != nil {
		return fmt.Errorf("failed to create root workspace dir: %w", err)
	}

	// We need to inject the token into the URL for auth
	// Example: https://github.com/user/repo -> https://x-access-token:TOKEN@github.com/user/repo
	authURL := gw.RepoURL
	if len(gw.Token) > 0 {
		authURL = fmt.Sprintf("https://x-access-token:%s@%s", gw.Token, gw.RepoURL[8:])
	}

	cmd := exec.Command("git", "clone", authURL, gw.LocalPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git clone failed: %s (%w)", string(out), err)
	}
	return nil
}

func (gw *GitWorkspace) CheckoutBranch(branch string) error {
	// try to checkout existing
	if err := gw.execGit("checkout", branch); err == nil {
		return nil
	}
	// fallback to checkout -b
	return gw.execGit("checkout", "-b", branch)
}

func (gw *GitWorkspace) WriteFiles(files map[string]string) error {
	for path, content := range files {
		fullPath := filepath.Join(gw.LocalPath, path)
		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			return err
		}
	}
	return nil
}

func (gw *GitWorkspace) CommitAndPush(message string, branch string) error {
	if err := gw.execGit("add", "."); err != nil {
		return err
	}

	// It might error if there's nothing to commit
	_ = gw.execGit("commit", "-m", message)

	return gw.execGit("push", "-u", "origin", branch)
}
