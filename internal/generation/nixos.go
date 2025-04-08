package generation

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const PROFILES_DIR = "/nix/var/nix/profiles"

func CurrentNixOSGeneration() (*NixOSGeneration, error) {
	p := fmt.Sprintf("%s/system", PROFILES_DIR)

	// Check if it system link exists
	info, err := os.Stat(p)
	if err != nil {
		return nil, err
	}
	if info.Mode()&fs.ModeSymlink != 0 {
		return nil, errors.New("main system profile not symlink")
	}

	// Resolve link
	res, err := os.Readlink(p)
	if err != nil {
		return nil, err
	}

	// Get file info on resolved link
	absp, err := filepath.Abs(fmt.Sprintf("%s/%s", PROFILES_DIR, res))
	if err != nil {
		return nil, err
	}

	linfo, err := os.Stat(absp)
	if err != nil {
		return nil, err
	}
	if linfo.Mode()&fs.ModeSymlink != 0 {
		return nil, errors.New("resolved system profile not symlink")
	}

	return NewNixOSGeneration(filepath.Dir(absp), linfo)
}

type NixOSGeneration struct {
	ID            int
	BuildDate     time.Time
	Version       string
	KernelVersion string
}

func NewNixOSGeneration(root string, info fs.FileInfo) (*NixOSGeneration, error) {
	// Get ID from name
	strID := regexp.MustCompile(`^system-(\d+)-link$`).FindStringSubmatch(info.Name())
	if strID[1] == "" {
		return nil, fmt.Errorf("generation path '%s' does not match pattern", info.Name())
	}
	id, err := strconv.Atoi(strID[1])
	if err != nil {
		return nil, err
	}

	// Read NixOS version
	nixosVer, err := os.ReadFile(fmt.Sprintf("%s/%s/nixos-version", root, info.Name()))
	if err != nil {
		return nil, err
	}

	// Get kernel version
	kernelVer, err := getKernelVersion(fmt.Sprintf("%s/%s", root, info.Name()))
	if err != nil {
		return nil, err
	}

	return &NixOSGeneration{
		ID:            id,
		BuildDate:     info.ModTime(),
		Version:       string(nixosVer),
		KernelVersion: kernelVer,
	}, nil
}

func getKernelVersion(system string) (string, error) {
	// List directories in <system>/kernel-modules/lib/modules
	entries, err := os.ReadDir(fmt.Sprintf("%s/kernel-modules/lib/modules", system))
	if err != nil {
		return "", err
	}

	// Find the first sub-folder matching semver
	for _, e := range entries {
		if regexp.MustCompile(`^\d+\.\d+\.\d+$`).MatchString(e.Name()) {
			return strings.TrimSuffix(e.Name(), "/"), nil
		}
	}

	return "Unknown", nil
}

func ListNixOSGenerations() ([]*NixOSGeneration, error) {
	// List files in root
	entries, err := os.ReadDir(PROFILES_DIR)
	if err != nil {
		return nil, err
	}

	// Iterate over entries and build list of generations
	generations := []*NixOSGeneration{}
	regex := regexp.MustCompile(`^system-\d+-link$`)
	for _, e := range entries {
		if e.Type()&fs.ModeSymlink != 0 && regex.MatchString(e.Name()) {
			info, err := e.Info()
			if err != nil {
				return nil, err
			}

			generation, err := NewNixOSGeneration(PROFILES_DIR, info)
			if err != nil {
				return nil, err
			}

			generations = append(generations, generation)
		}
	}

	return generations, nil
}
