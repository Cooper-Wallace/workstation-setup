package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	workstation "github.com/Cooper-Wallace/workstation-setup"
)

func main() {
	os.Exit(Main(os.Args[1:], os.Stdout, os.Stderr))
}

const mainHelp = `workstation-setup: Cross-Platform Meta-Orchestrator, Toolchain Auditor, and Shell Environment Bridge.

Usage:
  workstation-setup <command> [flags]

Available Commands:
  audit       Audit workstation configuration across all 4 layers
  apply       Execute workstation configuration stages (alias: setup)
  env         Standalone Shell Environment Bridge injector
  toolchains  Standalone toolchain auditor and installer

Flags:
  -h, --help     Show help information
  -V, --version  Show version information
`

// Main is the testable CLI entrypoint.
func Main(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stdout, mainHelp)
		return 0
	}

	switch args[0] {
	case "-V", "--version", "-version":
		fmt.Fprintf(stdout, "workstation-setup %s\n", workstation.Version)
		return 0
	case "-h", "--help", "help":
		fmt.Fprint(stdout, mainHelp)
		return 0
	case "audit":
		return runAudit(args[1:], stdout, stderr)
	case "apply", "setup":
		return runApply(args[1:], stdout, stderr)
	case "env":
		return runEnv(args[1:], stdout, stderr)
	case "toolchains":
		return runToolchains(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown command: %s\n\n", args[0])
		fmt.Fprint(stderr, mainHelp)
		return 1
	}
}

func runAudit(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("audit", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var (
		osFlag      string
		profileFlag string
		bashrcFlag  string
		verboseFlag bool
		configFlag  string
	)

	fs.StringVar(&osFlag, "os", "auto", "Override operating system detection")
	fs.StringVar(&profileFlag, "profile", "dev", "Toolchain profile tier to audit")
	fs.StringVar(&bashrcFlag, "bashrc-path", workstation.DefaultBashrcPath(), "Path to user .bashrc file")
	fs.BoolVar(&verboseFlag, "v", false, "Display verbose installation commands")
	fs.BoolVar(&verboseFlag, "verbose", false, "Display verbose installation commands")
	fs.StringVar(&configFlag, "config-path", "", "Path to custom toolchains catalog")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	return workstation.CmdAudit(stdout, workstation.AuditOptions{
		OS:         osFlag,
		Profile:    profileFlag,
		BashrcPath: bashrcFlag,
		Verbose:    verboseFlag,
		ConfigPath: configFlag,
	})
}

func runApply(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("apply", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var (
		osFlag             string
		profileFlag        string
		stageFlag          string
		bashrcFlag         string
		profilePathFlag    string
		installMissingFlag bool
		dryRunFlag         bool
		configFlag         string
	)

	fs.StringVar(&osFlag, "os", "auto", "Override operating system detection")
	fs.StringVar(&profileFlag, "profile", "dev", "Toolchain profile tier to apply")
	fs.StringVar(&stageFlag, "stage", "all", "Specific stage to execute")
	fs.StringVar(&bashrcFlag, "bashrc-path", workstation.DefaultBashrcPath(), "Path to user .bashrc file")
	fs.StringVar(&profilePathFlag, "profile-path", workstation.DefaultProfilePath(), "Path to user .profile file")
	fs.BoolVar(&installMissingFlag, "install-missing", false, "Execute installation commands for missing toolchains")
	fs.BoolVar(&dryRunFlag, "dry-run", false, "Simulate execution without modifying files or installing packages")
	fs.StringVar(&configFlag, "config-path", "", "Path to custom toolchains catalog")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	return workstation.CmdApply(stdout, workstation.ApplyOptions{
		OS:             osFlag,
		Profile:        profileFlag,
		Stage:          stageFlag,
		BashrcPath:     bashrcFlag,
		ProfilePath:    profilePathFlag,
		InstallMissing: installMissingFlag,
		DryRun:         dryRunFlag,
		ConfigPath:     configFlag,
	})
}

func runEnv(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("env", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var (
		bashrcFlag      string
		profilePathFlag string
		dbPathFlag      string
		dryRunFlag      bool
	)

	fs.StringVar(&bashrcFlag, "bashrc-path", workstation.DefaultBashrcPath(), "Path to user .bashrc file")
	fs.StringVar(&profilePathFlag, "profile-path", workstation.DefaultProfilePath(), "Path to user .profile file")
	fs.StringVar(&dbPathFlag, "db-path", workstation.DefaultDBPath(), "Path to sqlite dev database")
	fs.BoolVar(&dryRunFlag, "dry-run", false, "Simulate injection without modifying files")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	return workstation.CmdEnv(stdout, workstation.EnvOptions{
		BashrcPath:  bashrcFlag,
		ProfilePath: profilePathFlag,
		DBPath:      dbPathFlag,
		DryRun:      dryRunFlag,
	})
}

func runToolchains(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("toolchains", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var (
		osFlag      string
		profileFlag string
		installFlag bool
		dryRunFlag  bool
		configFlag  string
	)

	fs.StringVar(&osFlag, "os", "auto", "Override operating system detection")
	fs.StringVar(&profileFlag, "profile", "dev", "Toolchain profile tier")
	fs.BoolVar(&installFlag, "install", false, "Install missing toolchains")
	fs.BoolVar(&dryRunFlag, "dry-run", false, "Simulate installation without running commands")
	fs.StringVar(&configFlag, "config-path", "", "Path to custom toolchains catalog")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	return workstation.CmdToolchains(stdout, workstation.ToolchainsOptions{
		OS:         osFlag,
		Profile:    profileFlag,
		Install:    installFlag,
		DryRun:     dryRunFlag,
		ConfigPath: configFlag,
	})
}
