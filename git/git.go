package git

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type Repository struct {
	Path      string
	RemoteUrl string
}

func NewRepository(logger *slog.Logger, path, remoteUrl string) (*Repository, error) {
	repo := &Repository{
		Path:      path,
		RemoteUrl: remoteUrl,
	}

	gitDir := filepath.Join(path, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		logger.Info("Git repository not found, initializing...", "path", path)
		if err := os.MkdirAll(path, os.ModePerm); err != nil {
			return nil, fmt.Errorf("failed to create repository directory: %w", err)
		}

		initCmd := exec.Command("git", "init")
		initCmd.Dir = path
		if err := initCmd.Run(); err != nil {
			return nil, fmt.Errorf("git init failed: %w", err)
		}

		if remoteUrl != "" {
			logger.Info("Adding remote origin", "url", remoteUrl)
			remoteCmd := exec.Command("git", "remote", "add", "origin", remoteUrl)
			remoteCmd.Dir = path
			if err := remoteCmd.Run(); err != nil {
				logger.Warn("git remote add failed (maybe remote already exists?)", "error", err)
			}
		}
	}

	return repo, nil
}

func (r *Repository) GetCurrentBranch() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = r.Path
	output, err := cmd.CombinedOutput()
	if err != nil {
		if strings.Contains(string(output), "unknown revision or path not in the working tree") {
			cmd := exec.Command("git", "symbolic-ref", "--short", "HEAD")
			cmd.Dir = r.Path
			output, err := cmd.CombinedOutput()
			if err != nil {
				return "", fmt.Errorf("failed to get current branch with symbolic-ref after rev-parse failed: %w, output: %s", err, string(output))
			}
			return strings.TrimSpace(string(output)), nil
		}
		return "", fmt.Errorf("failed to get current branch: %w, output: %s", err, string(output))
	}
	return strings.TrimSpace(string(output)), nil
}

func (r *Repository) Checkout(logger *slog.Logger, branchName string) error {
	logger.Info("Switching to branch", "branch", branchName, "repo", r.Path)

	checkoutCmd := exec.Command("git", "checkout", branchName)
	checkoutCmd.Dir = r.Path
	output, err := checkoutCmd.CombinedOutput()

	if err == nil {
		logger.Info("Switched to existing branch", "branch", branchName, "output", string(output))
		return nil
	}

	logger.Info("Failed to checkout branch, attempting to create it", "branch", branchName, "error", err, "output", string(output))

	createCmd := exec.Command("git", "switch", "-c", branchName)
	createCmd.Dir = r.Path
	output, err = createCmd.CombinedOutput()

	if err != nil {
		return fmt.Errorf("failed to create and switch to branch '%s': %w, output: %s", branchName, err, string(output))
	}

	logger.Info("Created and switched to new branch", "branch", branchName, "output", string(output))
	return nil
}

func (r *Repository) Commit(logger *slog.Logger, authorName, authorEmail, message string, commitDate time.Time) (bool, error) {

	addCmd := exec.Command("git", "add", ".")
	addCmd.Dir = r.Path
	output, err := addCmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("git add failed: %w, output: %s", err, string(output))
	}

	statusCmd := exec.Command("git", "status", "--porcelain")
	statusCmd.Dir = r.Path
	output, err = statusCmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("git status failed: %w, output: %s", err, string(output))
	}
	if len(output) == 0 {
		logger.Warn("git commit: nothing to commit")
		return false, nil
	}

	author := fmt.Sprintf("%s <%s>", authorName, authorEmail)
	commitDateStr := commitDate.Format(time.RFC3339)

	commitCmd := exec.Command("git", "commit", "-m", message, "--author", author, "--date", commitDateStr)
	commitCmd.Dir = r.Path
	output, err = commitCmd.CombinedOutput()

	if err != nil {
		if strings.Contains(string(output), "nothing to commit") {
			logger.Warn("git commit: nothing to commit")
			return false, nil
		}
		return false, fmt.Errorf("git commit failed: %w, output: %s", err, string(output))
	}

	logger.Info("Git commit successful", "output", "")
	return true, nil
}

func (r *Repository) Push(logger *slog.Logger, remote, branch string) error {
	logger.Info("Pushing to remote", "repo", r.Path, "remote", remote, "branch", branch)
	pushCmd := exec.Command("git", "push", remote, branch)
	pushCmd.Dir = r.Path
	output, err := pushCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git push failed: %w, output: %s", err, string(output))
	}
	logger.Info("Git push successful", "output", string(output))
	return nil
}

func (r *Repository) Tag(logger *slog.Logger, tagName string) error {
	logger.Info("Creating git tag", "repo", r.Path, "tag", tagName)
	tagCmd := exec.Command("git", "tag", tagName)
	tagCmd.Dir = r.Path
	output, err := tagCmd.CombinedOutput()
	if err != nil {
		// git tag returns status 1 if tag already exists
		if strings.Contains(string(output), "already exists") {
			logger.Warn("git tag already exists", "tag", tagName, "output", string(output))
			return nil
		}
		return fmt.Errorf("git tag failed: %w, output: %s", err, string(output))
	}
	logger.Info("Git tag successful", "output", string(output))
	return nil
}

func (r *Repository) PushTags(logger *slog.Logger, remote string) error {
	logger.Info("Pushing tags to remote", "repo", r.Path, "remote", remote)
	pushCmd := exec.Command("git", "push", remote, "--tags")
	pushCmd.Dir = r.Path
	output, err := pushCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git push --tags failed: %w, output: %s", err, string(output))
	}
	logger.Info("Git push tags successful", "output", string(output))
	return nil
}
