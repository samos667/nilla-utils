package nix

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

type FixedOutputStoreEntry struct {
	Path string
	Hash string
}

func AddPathToStore(path string) (*FixedOutputStoreEntry, error) {
	// Add path to nix store
	p, err := exec.Command(
		"nix-store", "--recursive", "--add-fixed", "sha256", path,
	).Output()
	if err != nil {
		if xerr, ok := err.(*exec.ExitError); ok {
			return nil, errors.New(string(xerr.Stderr))
		}
		return nil, err
	}

	// Trim output
	storePath := strings.TrimSpace(string(p))

	// Get hash of store path
	hash, err := GetStoreHash(storePath)
	if err != nil {
		return nil, err
	}

	return &FixedOutputStoreEntry{
		Path: storePath,
		Hash: strings.TrimSpace(string(hash)),
	}, nil
}

func AddGitPathToStore(path string) (*FixedOutputStoreEntry, error) {
	// Create nix code that loads git repo into nix store
	codetpl := `
		let
			path = builtins.toPath "%s";
		in builtins.fetchGit path
	`
	code := fmt.Sprintf(codetpl, path)

	// Evaluate code
	eval, err := exec.Command(
		"nix", "eval",
		"--extra-experimental-features", "nix-command",
		"--raw", "--impure",
		"--expr", code,
	).Output()
	if err != nil {
		if xerr, ok := err.(*exec.ExitError); ok {
			return nil, errors.New(string(xerr.Stderr))
		}
		return nil, err
	}

	// Trim output
	storePath := strings.TrimSpace(string(eval))

	// Get hash of store path
	hash, err := GetStoreHash(storePath)
	if err != nil {
		return nil, err
	}

	return &FixedOutputStoreEntry{
		Path: storePath,
		Hash: strings.TrimSpace(string(hash)),
	}, nil
}

func GetStoreHash(path string) ([]byte, error) {
	out, err := exec.Command(
		"nix-store", "--query", path, "--hash",
	).Output()
	if err != nil {
		if xerr, ok := err.(*exec.ExitError); ok {
			return nil, errors.New(string(xerr.Stderr))
		}
		return nil, err
	}

	parts := bytes.Split(out, []byte{':'})

	return parts[len(parts)-1], nil
}

func GetStorePathName(path string) (string, error) {
	p := path
	if strings.HasPrefix(path, "/nix/store/") {
		p = strings.TrimPrefix(path, "/nix/store/")
	}

	parts := strings.SplitN(p, "-", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("Could not parse store path name \"%s\", wrong format", path)
	}

	return parts[1], nil
}

func ListAttrsInProject(file string, entry *FixedOutputStoreEntry, attr string) ([]string, error) {
	storePathName, err := GetStorePathName(entry.Path)
	if err != nil {
		return nil, err
	}

	// Create nix code that lists attr names
	code := fmt.Sprintf(
		`
			let
				source = builtins.path {path = "%s"; sha256 = "%s"; name = "%s";};
				project = import "${source}/%s";
			in
				builtins.attrNames (project.%s or {})
		`,
		entry.Path, entry.Hash, storePathName,
		file,
		attr,
	)

	// Execute code
	eval, err := exec.Command(
		"nix", "eval",
		"--extra-experimental-features", "nix-command",
		"--json", "--expr", code,
	).Output()
	if err != nil {
		if xerr, ok := err.(*exec.ExitError); ok {
			return nil, errors.New(string(xerr.Stderr))
		}
		return nil, err
	}

	// Parse json
	names := []string{}
	if err := json.Unmarshal(eval, &names); err != nil {
		return nil, err
	}

	return names, nil
}

func ExistsInProject(file string, entry *FixedOutputStoreEntry, name string) (bool, error) {
	storePathName, err := GetStorePathName(entry.Path)
	if err != nil {
		return false, err
	}

	// Create nix code that checks if a specified
	var code string
	if strings.Contains(name, ".") {
		parts := strings.Split(name, ".")
		last := parts[len(parts)-1]
		init := strings.Join(parts[:len(parts)-1], ".")

		code = fmt.Sprintf(
			`
				let
					source = builtins.path {path = "%s"; sha256 = "%s"; name = "%s";};
					project = import "${source}/%s";
				in
					(project.%s or {}) ? %s
			`,
			entry.Path, entry.Hash, storePathName,
			file,
			init, last,
		)
	} else {
		code = fmt.Sprintf(
			`
				let
					source = builtins.path {path = "%s"; sha256 = "%s"; name = "%s";};
					project = import "${source}/%s";
				in
					project ? %s
			`,
			entry.Path, entry.Hash, storePathName,
			file,
			name,
		)
	}

	// Execute code
	eval, err := exec.Command(
		"nix", "eval",
		"--extra-experimental-features", "nix-command",
		"--json", "--expr", code,
	).Output()
	if err != nil {
		if xerr, ok := err.(*exec.ExitError); ok {
			return false, errors.New(string(xerr.Stderr))
		}
		return false, err
	}

	val, err := strconv.ParseBool(strings.TrimSpace(string(eval)))
	if err != nil {
		return false, nil
	}

	return val, nil
}

type FetchGitOptions struct {
	Revision   string `json:"rev,omitempty"`
	Reference  string `json:"ref,omitempty"`
	Submodules bool   `json:"submodules"`
}

func FetchGit(url string, opts *FetchGitOptions) (*FixedOutputStoreEntry, error) {
	// Serialize options to json
	buf, err := json.Marshal(opts)
	if err != nil {
		return nil, err
	}

	// Template code for fetching git repo
	code := fmt.Sprintf(
		`
			let
				info = builtins.fromJSON ''%s'';
			in
				builtins.fetchGit (
					{ url = "%s"; }
					// (if info ? rev && info.rev != null then { inherit (info) rev; } else {})
					// (if info ? ref && info.ref != null then { inherit (info) ref; } else {})
					// (
						if info ? submodules && info.submodules != null
						then { inherit (info) submodules; } else {}
					)
				)
		`,
		buf, url,
	)

	// Execute code
	eval, err := exec.Command(
		"nix", "eval",
		"--extra-experimental-features", "nix-command",
		"--raw", "--impure", "--expr", code,
	).Output()
	if err != nil {
		if xerr, ok := err.(*exec.ExitError); ok {
			return nil, errors.New(string(xerr.Stderr))
		}
		return nil, err
	}

	// Trim output
	storePath := strings.TrimSpace(string(eval))

	// Get hash of store path
	hash, err := GetStoreHash(storePath)
	if err != nil {
		return nil, err
	}

	return &FixedOutputStoreEntry{
		Path: storePath,
		Hash: strings.TrimSpace(string(hash)),
	}, nil
}
