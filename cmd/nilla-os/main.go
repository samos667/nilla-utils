package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"

	"github.com/arnarg/nilla-utils/internal/nix"
	"github.com/arnarg/nilla-utils/internal/tui"
	"github.com/urfave/cli/v3"
)

var version = "unknown"

var description = `[name]  Name of the NixOS system to build. If left empty it will use current hostname.`

type subCmd int

const (
	subCmdBuild subCmd = iota
	subCmdTest
	subCmdBoot
	subCmdSwitch
)

const SYSTEM_PROFILE = "/nix/var/nix/profiles/system"
const CURRENT_PROFILE = "/run/current-system"

func actionFuncFor(sub subCmd) cli.ActionFunc {
	return func(ctx context.Context, cmd *cli.Command) error {
		return run(ctx, cmd, sub)
	}
}

var app = &cli.Command{
	Name:            "nilla-os",
	Version:         version,
	Usage:           "A nilla cli plugin to work with NixOS configurations.",
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
			Usage:       "Build NixOS configuration",
			Description: fmt.Sprintf("Build NixOS configuration.\n\n%s", description),
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

		// Test
		{
			Name:        "test",
			Usage:       "Build NixOS configuration and activate it",
			Description: fmt.Sprintf("Build NixOS configuration and activate it.\n\n%s", description),
			ArgsUsage:   "[name]",
			Flags: []cli.Flag{
				&cli.BoolFlag{
					Name:    "confirm",
					Aliases: []string{"c"},
					Usage:   "Do not ask for confirmation",
				},
			},
			Action: actionFuncFor(subCmdTest),
		},

		// Boot
		{
			Name:        "boot",
			Usage:       "Build NixOS configuration and make it the boot default",
			Description: fmt.Sprintf("Build NixOS configuration and make it the boot default.\n\n%s", description),
			ArgsUsage:   "[name]",
			Flags: []cli.Flag{
				&cli.BoolFlag{
					Name:    "confirm",
					Aliases: []string{"c"},
					Usage:   "Do not ask for confirmation",
				},
			},
			Action: actionFuncFor(subCmdBoot),
		},

		// Switch
		{
			Name:        "switch",
			Usage:       "Build NixOS configuration, activate it and make it the boot default",
			Description: fmt.Sprintf("Build NixOS configuration, activate it and make it the boot default.\n\n%s", description),
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
			Description: "Work with NixOS generations",
			Commands: []*cli.Command{
				// List
				{
					Name:        "list",
					Aliases:     []string{"ls"},
					Description: "List NixOS generations",
					Action:      listGenerations,
				},
			},
		},
	},
	Action: func(ctx context.Context, cmd *cli.Command) error {
		if cmd.Bool("version") {
			cli.ShowVersion(cmd)
		}
		return nil
	},
}

func printSection(text string) {
	fmt.Fprintf(os.Stderr, "\033[32m>\033[0m %s\n", text)
}

func inferName(name string) (string, error) {
	if name == "" {
		hn, err := os.Hostname()
		if err != nil {
			return "", err
		}
		return hn, nil
	}
	return name, nil
}

func run(ctx context.Context, cmd *cli.Command, sc subCmd) error {
	// Try to infer name of the NixOS system
	name, err := inferName(cmd.Args().First())
	if err != nil {
		return err
	}

	// Attribute of NixOS configuration's toplevel
	attr := fmt.Sprintf("systems.nixos.%s.result.config.system.build.toplevel", name)

	//
	// NixOS configuration build
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
	diff := exec.Command("nvd", "diff", CURRENT_PROFILE, string(out))
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
	// Activate NixOS configuration
	//
	if sc == subCmdTest || sc == subCmdSwitch {
		fmt.Fprintln(os.Stderr)
		printSection("Activating configuration")

		// Run switch_to_configuration
		switchp := fmt.Sprintf("%s/bin/switch-to-configuration", out)
		switchc := exec.Command("sudo", switchp, "test")
		switchc.Stderr = os.Stderr
		switchc.Stdout = os.Stdout

		// This error should be ignored during switch so that
		// it can continue onto setting up the bootloader below
		if err := switchc.Run(); err != nil && sc != subCmdSwitch {
			return err
		}
	}

	//
	// Set NixOS configuration in bootloader
	//
	if sc == subCmdBoot || sc == subCmdSwitch {
		// Set profile
		_, err := nix.Command("build").
			Args([]string{
				"--no-link",
				"--profile", SYSTEM_PROFILE,
				string(out),
			}).
			Privileged(true).
			Run(context.Background())
		if err != nil {
			return err
		}

		fmt.Fprintln(os.Stderr)
		printSection("Adding configuration to bootloader")

		// Run switch_to_configuration
		switchp := fmt.Sprintf("%s/bin/switch-to-configuration", out)
		switchc := exec.Command("sudo", switchp, "boot")
		switchc.Stderr = os.Stderr
		switchc.Stdout = os.Stdout

		return switchc.Run()
	}

	return nil
}

func main() {
	if err := app.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}
