package git

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

func Clone(src, dst string) error {
	return run(exec.Command("git", "clone", src, dst))
}

func Checkout(repo, branch string) error {
	return run(exec.Command("git", "-C", repo, "checkout", "-B", branch))
}

func Fetch(repo string) error {
	return run(exec.Command("git", "-C", repo, "fetch", "origin"))
}

func Merge(repo, ref string) error {
	return run(exec.Command("git", "-C", repo, "merge", ref))
}

func BaseBranch(repo string) string {
	for _, b := range []string{"main", "master"} {
		if exec.Command("git", "-C", repo, "rev-parse", "--verify", b).Run() == nil {
			return b
		}
	}
	return ""
}

func Behind(repo, base string) bool {
	out, err := exec.Command("git", "-C", repo, "rev-list", "--count", "HEAD..origin/"+base).Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) != "0"
}

func DiffStat(repo, base, branch string) (added, deleted int) {
	out, err := exec.Command("git", "-C", repo, "diff", "--numstat", base, branch).Output()
	if err != nil {
		return 0, 0
	}

	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		a, _ := strconv.Atoi(fields[0])
		d, _ := strconv.Atoi(fields[1])
		added += a
		deleted += d
	}
	return added, deleted
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
