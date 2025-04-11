package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"

	"github.com/arnarg/nilla-utils/internal/nix"
	"github.com/arnarg/nilla-utils/internal/tui"
	"github.com/arnarg/nilla-utils/internal/util"
	"github.com/urfave/cli/v3"
)

var version = "unknown"

var description = `[name]  Name of the home-manager system to build. If left empty it will try "$USER@<hostname>" and "$USER".`

type subCmd int

const (
	subCmdBuild subCmd = iota
	subCmdSwitch
)

var (
	errNoUserFound               = errors.New("no user found")
	errHomeConfigurationNotFound = errors.New("home configuration not found")
	errHomeCurrentGenNotFound    = errors.New("current generation not found")
)

func actionFuncFor(sub subCmd) cli.ActionFunc {
	return func(ctx context.Context, cmd *cli.Command) error {
		return run(ctx, cmd, sub)
	}
}

var app = &cli.Command{
	Name:            "nilla-home",
	Version:         version,
	Usage:           "A nilla cli plugin to work with home-manager configurations.",
	HideVersion:     true,
	HideHelpCommand: true,
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:        "version",
			Aliases:     []string{"V"},
			Usage:       "Print version",
			HideDefault: true,
			Local:       true,
		},
		&cli.BoolFlag{
			Name:        "verbose",
			Aliases:     []string{"v"},
			Usage:       "Set log level to verbose",
			HideDefault: true,
		},
	},
	Commands: []*cli.Command{
		// Build
		{
			Name:        "build",
			Usage:       "Build Home Manager configuration",
			Description: fmt.Sprintf("Build Home Manager configuration.\n\n%s", description),
			ArgsUsage:   "[name]",
			Flags: []cli.Flag{
				&cli.BoolFlag{
					Name:  "no-link",
					Usage: "Do not create symlinks to the build results",
				},
				&cli.BoolFlag{
					Name:  "print-out-paths",
					Usage: "Print the resulting output paths",
				},
				&cli.StringFlag{
					Name:    "out-link",
					Aliases: []string{"o"},
					Usage:   "Use path as prefix for the symlinks to the build results",
				},
			},
			Action: actionFuncFor(subCmdBuild),
		},

		// Switch
		{
			Name:        "switch",
			Usage:       "Build Home Manager configuration and activate it",
			Description: fmt.Sprintf("Build Home Manager configuration and activate it.\n\n%s", description),
			ArgsUsage:   "[name]",
			Flags: []cli.Flag{
				&cli.BoolFlag{
					Name:    "confirm",
					Aliases: []string{"c"},
					Usage:   "Do not ask for confirmation",
				},
			},
			Action: actionFuncFor(subCmdSwitch),
		},

		// Generations
		{
			Name:        "generations",
			Aliases:     []string{"gen"},
			Usage:       "Work with home-manager generations",
			Description: "Work with home-manager generations",
			Commands: []*cli.Command{
				// List
				{
					Name:        "list",
					Aliases:     []string{"ls"},
					Usage:       "List home-manager generations",
					Description: "List home-manager generations",
					Action:      listGenerations,
				},

				// Clean
				{
					Name:        "clean",
					Aliases:     []string{"c"},
					Usage:       "Delete and garbage collect NixOS generations",
					Description: "Delete and garbage collect NixOS generations",
					Flags: []cli.Flag{
						&cli.UintFlag{
							Name:    "keep",
							Aliases: []string{"k"},
							Usage:   "Number of generations to keep",
							Value:   1,
						},
						&cli.BoolFlag{
							Name:    "confirm",
							Aliases: []string{"c"},
							Usage:   "Do not ask for confirmation",
						},
					},
					Action: cleanGenerations,
				},
			},
		},
	},
	Action: func(ctx context.Context, cmd *cli.Command) error {
		if cmd.Args().Len() < 1 {
			cli.ShowAppHelp(cmd)
		}
		if cmd.Bool("version") {
			cli.ShowVersion(cmd)
		}
		return nil
	},
}

func printSection(text string) {
	fmt.Fprintf(os.Stderr, "\033[32m>\033[0m %s\n", text)
}

func inferNames(name string) ([]string, error) {
	if name == "" {
		names := []string{}

		user := util.GetUser()
		if user == "" {
			return nil, errNoUserFound
		}

		if hn, err := os.Hostname(); err == nil {
			names = append(names, fmt.Sprintf("%s@%s", user, hn))
		}

		return append(names, user), nil
	}
	return []string{name}, nil
}

func findHomeConfiguration(names []string) (string, error) {
	for _, name := range names {
		code := fmt.Sprintf("x: x ? \"%s\"", name)
		out, err := exec.Command(
			"nix", "eval", "-f", "nilla.nix", "systems.home", "--apply", code,
		).Output()
		if err != nil {
			continue
		}
		if string(bytes.TrimSpace(out)) == "true" {
			return name, nil
		}
	}
	return "", errHomeConfigurationNotFound
}

func findCurrentGeneration() (string, error) {
	// Check in /nix/var/nix/profiles
	if user := util.GetUser(); user != "" {
		perUser := fmt.Sprintf("/nix/var/nix/profiles/per-user/%s/home-manager", user)
		if _, err := os.Stat(perUser); err == nil {
			return perUser, nil
		} else if !errors.Is(err, fs.ErrNotExist) {
			return "", err
		}
	}

	// Check ~/.local/state/nix/profiles
	if home := util.GetHomeDir(); home != "" {
		homeProfile := fmt.Sprintf("%s/.local/state/nix/profiles/home-manager", home)
		if _, err := os.Stat(homeProfile); err == nil {
			return homeProfile, nil
		} else if !errors.Is(err, fs.ErrNotExist) {
			return "", err
		}
	}
	return "", errHomeCurrentGenNotFound
}

func run(ctx context.Context, cmd *cli.Command, sc subCmd) error {
	// Try to find current generation
	current, err := findCurrentGeneration()
	if err != nil {
		return err
	}

	// Try to infer names to try for the home-manager configuration
	names, err := inferNames(cmd.Args().First())
	if err != nil {
		return err
	}

	// Find home configuration from candidates
	name, err := findHomeConfiguration(names)
	if err != nil {
		return err
	}

	// Attribute of home-manager's activation package
	attr := fmt.Sprintf("systems.home.%s.result.config.home.activationPackage", name)

	//
	// Home Manager configuration build
	//
	// Build args for nix build
	nargs := []string{"-f", "nilla.nix", attr}

	// Add extra args depending on the sub command
	if sc == subCmdBuild {
		if cmd.Bool("no-link") {
			nargs = append(nargs, "--no-link")
		}
		if cmd.String("out-link") != "" {
			nargs = append(nargs, "--out-link", cmd.String("out-link"))
		}
	} else {
		// All sub-commands except build should not
		// create a result link
		nargs = append(nargs, "--no-link")
	}

	// Run nix build
	printSection("Building configuration")
	out, err := nix.Command("build").
		Args(nargs).
		Reporter(tui.NewBuildReporter(cmd.Bool("verbose"))).
		Run(ctx)
	if err != nil {
		return err
	}

	//
	// Run generation diff using nvd
	//
	fmt.Fprintln(os.Stderr)
	printSection("Comparing changes")

	// Run nvd diff
	diff := exec.Command("nvd", "diff", current, string(out))
	diff.Stderr = os.Stderr
	diff.Stdout = os.Stderr
	if err := diff.Run(); err != nil {
		return err
	}

	// Build can exit now
	if sc == subCmdBuild {
		return nil
	}

	//
	// Ask Confirmation
	//
	if !cmd.Bool("confirm") {
		doContinue, err := tui.RunConfirm("Do you want to continue?")
		if err != nil {
			return err
		}
		if !doContinue {
			return nil
		}
	}

	//
	// Activate Home Manager configuration
	//
	if sc == subCmdSwitch {
		fmt.Fprintln(os.Stderr)
		printSection("Activating configuration")

		// Run switch_to_configuration
		switchp := fmt.Sprintf("%s/activate", out)
		switchc := exec.Command(switchp)
		switchc.Stderr = os.Stderr
		switchc.Stdout = os.Stdout

		if err := switchc.Run(); err != nil {
			return err
		}
	}

	return nil
}

func main() {
	if err := app.Run(context.Background(), os.Args); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
