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
	Name:    "nilla-os",
	Version: version,
	Usage:   "A nilla cli plugin to work with NixOS configurations.",
	Commands: []*cli.Command{
		// Build
		{
			Name:        "build",
			Usage:       "Build NixOS configuration",
			Description: "Build NixOS configuration",
			ArgsUsage:   "[system name]",
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
			Description: "Build NixOS configuration and activate it",
			ArgsUsage:   "[system name]",
			Action:      actionFuncFor(subCmdTest),
		},

		// Boot
		{
			Name:        "boot",
			Usage:       "Build NixOS configuration and make it the boot default",
			Description: "Build NixOS configuration and make it the boot default",
			ArgsUsage:   "[system name]",
			Action:      actionFuncFor(subCmdBoot),
		},

		// Switch
		{
			Name:        "switch",
			Usage:       "Build NixOS configuration, activate it and make it the boot default",
			Description: "Build NixOS configuration, activate it and make it the boot default",
			ArgsUsage:   "[system name]",
			Action:      actionFuncFor(subCmdSwitch),
		},
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
