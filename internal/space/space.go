package space

import (
	"os"
	"path/filepath"
)

const spacesRoot = "Spaces"

func Dir(root string) string {
	return filepath.Join(root, spacesRoot)
}

func Path(root, name string) string {
	return filepath.Join(root, spacesRoot, name)
}

func RepoPath(spaceDir, repo string) string {
	return filepath.Join(spaceDir, repo)
}

func Repos(root string) []string {
	var names []string
	for _, name := range dirs(root) {
		if name == spacesRoot {
			continue
		}
		if isGitRepo(filepath.Join(root, name)) {
			names = append(names, name)
		}
	}
	return names
}

func Names(root string) []string {
	return dirs(Dir(root))
}

func ReposIn(spaceDir string) []string {
	return dirs(spaceDir)
}

func ContainsRepos(root, name string, repos []string) bool {
	dir := Path(root, name)
	for _, r := range repos {
		if !Exists(RepoPath(dir, r)) {
			return false
		}
	}
	return true
}

func Create(root, name string) (string, error) {
	dir := Path(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

func Delete(root, name string) error {
	return os.RemoveAll(Path(root, name))
}

func RemoveRepo(spaceDir, repo string) error {
	return os.RemoveAll(RepoPath(spaceDir, repo))
}

func Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func dirs(path string) []string {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil
	}

	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names
}

func isGitRepo(path string) bool {
	_, err := os.Stat(filepath.Join(path, ".git"))
	return err == nil
}
