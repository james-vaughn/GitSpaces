package git

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

func Clone(ctx context.Context, src, dst string) error {
	return run(exec.CommandContext(ctx, "git", "clone", src, dst))
}

func Checkout(repo, branch, start string) error {
	args := []string{"-C", repo, "checkout", "-B", branch}
	if start != "" {
		args = append(args, "--no-track", start)
	}
	return run(exec.Command("git", args...))
}

func RemoteURL(repo string) (string, error) {
	out, err := output(exec.Command("git", "-C", repo, "remote", "get-url", "origin"))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func SetUpstream(repo, branch string) error {
	if err := run(exec.Command("git", "-C", repo, "config", "branch."+branch+".remote", "origin")); err != nil {
		return err
	}
	return run(exec.Command("git", "-C", repo, "config", "branch."+branch+".merge", "refs/heads/"+branch))
}

func Fetch(repo string) error {
	return run(exec.Command("git", "-C", repo, "fetch", "origin"))
}

func Merge(repo, ref string) error {
	return run(exec.Command("git", "-C", repo, "merge", ref))
}

func BaseBranch(repo string) string {
	for _, b := range []string{"main", "master"} {
		for _, ref := range []string{"refs/heads/" + b, "refs/remotes/origin/" + b} {
			if exec.Command("git", "-C", repo, "rev-parse", "--verify", ref).Run() == nil {
				return b
			}
		}
	}
	return ""
}

func Behind(repo, base string) bool {
	out, err := output(exec.Command("git", "-C", repo, "rev-list", "--count", "HEAD..origin/"+base))
	if err != nil {
		return false
	}
	return strings.TrimSpace(out) != "0"
}

func DiffStat(repo, base, branch string) (added, deleted int, err error) {
	out, err := output(exec.Command("git", "-C", repo, "diff", "--numstat", "origin/"+base, branch))
	if err != nil {
		return 0, 0, err
	}

	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		a, _ := strconv.Atoi(fields[0])
		d, _ := strconv.Atoi(fields[1])
		added += a
		deleted += d
	}
	return added, deleted, nil
}

func output(cmd *exec.Cmd) (string, error) {
	out, err := cmd.Output()
	if err == nil {
		return string(out), nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if msg := strings.TrimSpace(string(exitErr.Stderr)); msg != "" {
			return "", fmt.Errorf("%w: %s", err, msg)
		}
	}
	return "", err
}

func run(cmd *exec.Cmd) error {
	out, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}
	if msg := strings.TrimSpace(string(out)); msg != "" {
		return fmt.Errorf("%w: %s", err, msg)
	}
	return err
}
