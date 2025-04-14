package project

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/arnarg/nilla-utils/internal/nix"
	"github.com/arnarg/nilla-utils/internal/util"
	"github.com/charmbracelet/log"
)

// Resolve takes in a project uri and tries to resolve it into a nix store path.
// Currently supported URIs:
//   - Paths starting with `./`, `/` or `~`
//   - URI starting with `path:`
func Resolve(uri string) (*nix.FixedOutputStoreEntry, error) {
	var entry *nix.FixedOutputStoreEntry

	log.Infof("Looking for project at %s", uri)

	switch {
	// Standard path
	case isPath(uri):
		resolved, err := ResolvePath(uri)
		if err != nil {
			return nil, err
		}
		entry = resolved

	// With `path:` prefix
	case strings.HasPrefix(uri, "path:"):
		resolved, err := ResolvePath(strings.TrimPrefix(uri, "path:"))
		if err != nil {
			return nil, err
		}
		entry = resolved
	}

	if entry != nil {
		log.Debugf("Resolved project \"%s\"", entry.Path)
		return entry, nil
	}

	return nil, fmt.Errorf("unknown project uri pattern: '%s'", uri)
}

func ResolvePath(path string) (*nix.FixedOutputStoreEntry, error) {
	// Resolve tilde and relative path
	fullpath, err := resolveRealPath(path)
	if err != nil {
		return nil, err
	}

	// Look up the directory tree for nilla.nix
	resolved, err := searchUpForNillaNix(fullpath)
	if err != nil {
		return nil, err
	}

	log.Debugf("Found path %s", resolved)

	// Check if the resolved directory is a git directory
	if isGitDir(resolved) {
		return resolveGitPath(resolved)
	}

	// Add resolved path to nix store
	entry, err := nix.AddPathToStore(resolved)
	if err != nil {
		return nil, err
	}

	return entry, nil
}

func isPath(uri string) bool {
	return strings.HasPrefix(uri, ".") ||
		strings.HasPrefix(uri, "/") ||
		strings.HasPrefix(uri, "~")
}

func resolveRealPath(path string) (string, error) {
	if strings.HasPrefix(path, "~") {
		path = filepath.Join(util.GetHomeDir(), strings.TrimPrefix(path[1:], "/"))
	}

	return filepath.Abs(path)
}

func searchUpForNillaNix(path string) (string, error) {
	for {
		_, err := os.Stat(filepath.Join(path, "nilla.nix"))
		if err == nil {
			return path, nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}

		if dir := filepath.Dir(path); dir != path {
			path = dir
			continue
		}

		return "", fmt.Errorf("could not find find nilla.nix in %s", path)
	}
}

func isGitDir(path string) bool {
	info, err := os.Stat(filepath.Join(path, ".git"))
	if err != nil {
		return false
	}

	return info.IsDir()
}

func resolveGitPath(path string) (*nix.FixedOutputStoreEntry, error) {
	// Get untracked files
	untracked := getUntrackedFiles(path)
	if len(untracked) > 0 {
		log.Warnf("Untracked files in \"%s\" will not be available within Nix", path)
		for _, f := range untracked {
			log.Warnf("  %s", f)
		}
		log.Warn("If you experience issues, try adding these files to your git repository with `git add`")
	}

	// Add git path to nix store
	entry, err := nix.AddGitPathToStore(path)
	if err != nil {
		return nil, err
	}

	return entry, nil
}

func getUntrackedFiles(path string) []string {
	// Check if git is in path
	gitp, err := exec.LookPath("git")
	if err != nil {
		return []string{}
	}

	// Run git ls-files
	cmd := exec.Command(
		gitp, "ls-files", "--others", "--exclude-standard",
	)
	cmd.Dir = path
	out, err := cmd.Output()
	if err != nil {
		return []string{}
	}

	// Split list by lines
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")

	return slices.DeleteFunc(lines, func(line string) bool {
		return line == ""
	})
}
