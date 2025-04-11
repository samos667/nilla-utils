package generation

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"github.com/arnarg/nilla-utils/internal/util"
)

func CurrentHomeGeneration() (*HomeGeneration, error) {
	// Check in /nix/var/nix/profiles
	if user := util.GetUser(); user != "" {
		perUser := fmt.Sprintf("/nix/var/nix/profiles/per-user/%s", user)
		gen, err := currentHomeGeneration(perUser)
		if err != nil {
			return nil, err
		}
		if gen != nil {
			return gen, nil
		}
	}

	// Check ~/.local/state/nix/profiles
	if home := util.GetHomeDir(); home != "" {
		homeProfile := fmt.Sprintf("%s/.local/state/nix/profiles", home)
		gen, err := currentHomeGeneration(homeProfile)
		if err != nil {
			return nil, err
		}
		if gen != nil {
			return gen, nil
		}
	}
	return nil, errors.New("current generation not found")
}

func currentHomeGeneration(dir string) (*HomeGeneration, error) {
	p := fmt.Sprintf("%s/home-manager", dir)

	// Check if it home-manager link exists
	info, err := os.Stat(p)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	if info.Mode()&fs.ModeSymlink != 0 {
		return nil, errors.New("main home-manager profile not symlink")
	}

	// Resolve link
	res, err := os.Readlink(p)
	if err != nil {
		return nil, err
	}

	// Get file info on resolved link
	absp, err := filepath.Abs(fmt.Sprintf("%s/%s", dir, res))
	if err != nil {
		return nil, err
	}

	linfo, err := os.Stat(absp)
	if err != nil {
		return nil, err
	}
	if linfo.Mode()&fs.ModeSymlink != 0 {
		return nil, errors.New("resolved home-manager profile not symlink")
	}

	return NewHomeGeneration(filepath.Dir(absp), linfo)
}

type HomeGeneration struct {
	ID        int
	BuildDate time.Time
	Version   string

	path string
}

func NewHomeGeneration(root string, info fs.FileInfo) (*HomeGeneration, error) {
	// Get ID from name
	strID := regexp.MustCompile(`^home-manager-(\d+)-link$`).FindStringSubmatch(info.Name())
	if strID[1] == "" {
		return nil, fmt.Errorf("generation path '%s' does not match pattern", info.Name())
	}
	id, err := strconv.Atoi(strID[1])
	if err != nil {
		return nil, err
	}

	// Build full path
	path := fmt.Sprintf("%s/%s", root, info.Name())

	// Read home-manager version
	homeVer, err := os.ReadFile(fmt.Sprintf("%s/hm-version", path))
	if err != nil {
		return nil, err
	}

	return &HomeGeneration{
		ID:        id,
		BuildDate: info.ModTime(),
		Version:   string(bytes.TrimSpace(homeVer)),
		path:      path,
	}, nil
}

func (g *HomeGeneration) Delete() error {
	if err := os.Remove(g.path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return nil
}

func ListHomeGenerations() ([]*HomeGeneration, error) {
	// Check in /nix/var/nix/profiles
	if user := util.GetUser(); user != "" {
		perUser := fmt.Sprintf("/nix/var/nix/profiles/per-user/%s", user)
		gens, err := listHomeGenerations(perUser)
		if err != nil {
			return nil, err
		}
		if len(gens) > 0 {
			return gens, nil
		}
	}

	// Check ~/.local/state/nix/profiles
	if home := util.GetHomeDir(); home != "" {
		homeProfile := fmt.Sprintf("%s/.local/state/nix/profiles", home)
		gens, err := listHomeGenerations(homeProfile)
		if err != nil {
			return nil, err
		}
		if len(gens) > 0 {
			return gens, nil
		}
	}

	return []*HomeGeneration{}, nil
}

func listHomeGenerations(dir string) ([]*HomeGeneration, error) {
	// List files in dir
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []*HomeGeneration{}, nil
		}
		return nil, err
	}

	// Iterate over entries and build list of generations
	generations := []*HomeGeneration{}
	regex := regexp.MustCompile(`^home-manager-\d+-link$`)
	for _, e := range entries {
		if e.Type()&fs.ModeSymlink != 0 && regex.MatchString(e.Name()) {
			info, err := e.Info()
			if err != nil {
				return nil, err
			}

			generation, err := NewHomeGeneration(dir, info)
			if err != nil {
				return nil, err
			}

			generations = append(generations, generation)
		}
	}

	return generations, nil
}
