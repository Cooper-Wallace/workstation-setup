package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	workstation "github.com/Cooper-Wallace/workstation-setup"
)

func TestMainHelpAndVersion(t *testing.T) {
	// No args -> print help, exit 0
	var outHelp bytes.Buffer
	var errHelp bytes.Buffer
	code := Main([]string{}, &outHelp, &errHelp)
	if code != 0 {
		t.Errorf("expected 0 on empty args, got %d", code)
	}
	if !strings.Contains(outHelp.String(), "Available Commands:") {
		t.Errorf("expected help output, got: %s", outHelp.String())
	}

	// -V / --version -> print version, exit 0
	var outVer bytes.Buffer
	codeVer := Main([]string{"--version"}, &outVer, &errHelp)
	if codeVer != 0 {
		t.Errorf("expected 0 on --version, got %d", codeVer)
	}
	if !strings.Contains(outVer.String(), "workstation-setup 1.0.0") {
		t.Errorf("expected workstation-setup 1.0.0, got: %s", outVer.String())
	}

	var outV bytes.Buffer
	codeV := Main([]string{"-V"}, &outV, &errHelp)
	if codeV != 0 || !strings.Contains(outV.String(), "1.0.0") {
		t.Errorf("expected version on -V, got %d, out: %s", codeV, outV.String())
	}

	// -h / --help
	var outH bytes.Buffer
	codeH := Main([]string{"--help"}, &outH, &errHelp)
	if codeH != 0 || !strings.Contains(outH.String(), "Usage:") {
		t.Errorf("expected usage on --help, got %d", codeH)
	}

	// Unknown subcommand -> exit 1
	var outErr bytes.Buffer
	var errBuf bytes.Buffer
	codeUnknown := Main([]string{"foobar"}, &outErr, &errBuf)
	if codeUnknown != 1 {
		t.Errorf("expected 1 on unknown subcommand, got %d", codeUnknown)
	}
	if !strings.Contains(errBuf.String(), "unknown command: foobar") {
		t.Errorf("expected unknown command error, got: %s", errBuf.String())
	}
}

func TestMainAuditSubcommand(t *testing.T) {
	tmpDir := t.TempDir()
	bashrc := filepath.Join(tmpDir, ".bashrc")
	_ = os.WriteFile(bashrc, []byte(workstation.GetStandardEnvBlock()+"\n"), 0644)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	_ = Main([]string{"audit", "--os=linux", "--profile=minimal", "--bashrc-path", bashrc, "-v"}, &stdout, &stderr)
	// Even if exit code is 0 or 1 depending on system binaries, audit ran
	if !strings.Contains(stdout.String(), "Workstation Setup Audit Engine") {
		t.Errorf("audit did not run properly. Output:\n%s\nStderr:\n%s", stdout.String(), stderr.String())
	}

	// Test invalid flag
	var errOut bytes.Buffer
	codeInvalid := Main([]string{"audit", "--invalid-flag"}, &stdout, &errOut)
	if codeInvalid != 1 {
		t.Errorf("expected exit 1 on invalid flag, got %d", codeInvalid)
	}
}

func TestMainApplySubcommand(t *testing.T) {
	tmpDir := t.TempDir()
	bashrc := filepath.Join(tmpDir, ".bashrc")
	profile := filepath.Join(tmpDir, ".profile")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Main([]string{
		"apply",
		"--os=linux",
		"--profile=minimal",
		"--stage=2",
		"--bashrc-path=" + bashrc,
		"--profile-path=" + profile,
		"--dry-run",
	}, &stdout, &stderr)

	if code != 0 {
		t.Errorf("apply failed with code %d. Output:\n%s", code, stdout.String())
	}
	if !strings.Contains(stdout.String(), "Workstation Setup Orchestrator") {
		t.Errorf("missing orchestrator header in apply output")
	}

	// Test setup alias
	var stdoutSetup bytes.Buffer
	var stderrSetup bytes.Buffer
	codeSetup := Main([]string{"setup", "--stage=2", "--dry-run", "--bashrc-path", bashrc}, &stdoutSetup, &stderrSetup)
	if codeSetup != 0 {
		t.Errorf("setup alias failed with code %d", codeSetup)
	}

	// Test invalid flag
	var errOut bytes.Buffer
	codeInvalid := Main([]string{"apply", "--bad-flag"}, &stdout, &errOut)
	if codeInvalid != 1 {
		t.Errorf("expected exit 1 on bad flag, got %d", codeInvalid)
	}
}

func TestMainEnvSubcommand(t *testing.T) {
	tmpDir := t.TempDir()
	bashrc := filepath.Join(tmpDir, ".bashrc")
	profile := filepath.Join(tmpDir, ".profile")
	dbPath := filepath.Join(tmpDir, "dev.db")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Main([]string{
		"env",
		"--bashrc-path", bashrc,
		"--profile-path", profile,
		"--db-path", dbPath,
		"--dry-run",
	}, &stdout, &stderr)

	if code != 0 {
		t.Errorf("env failed with code %d", code)
	}
	if !strings.Contains(stdout.String(), "MODIFIED") && !strings.Contains(stdout.String(), "UP-TO-DATE") {
		t.Errorf("unexpected env output: %s", stdout.String())
	}

	// Test invalid flag
	var errOut bytes.Buffer
	codeInvalid := Main([]string{"env", "--bad-flag"}, &stdout, &errOut)
	if codeInvalid != 1 {
		t.Errorf("expected exit 1 on bad flag, got %d", codeInvalid)
	}
}

func TestMainToolchainsSubcommand(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	_ = Main([]string{"toolchains", "--os=linux", "--profile=minimal", "--dry-run"}, &stdout, &stderr)

	if !strings.Contains(stdout.String(), "Auditing toolchains") {
		t.Errorf("toolchains did not run properly. Output:\n%s", stdout.String())
	}

	// Test invalid flag
	var errOut bytes.Buffer
	codeInvalid := Main([]string{"toolchains", "--bad-flag"}, &stdout, &errOut)
	if codeInvalid != 1 {
		t.Errorf("expected exit 1 on bad flag, got %d", codeInvalid)
	}
}
