package workstation

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func resetGlobals() {
	LookPath = exec.LookPath
	ProcSwapsPath = "/proc/swaps"
	UserHomeDir = os.UserHomeDir
	RunCommand = defaultRunCommand
	RunCmdInteractive = defaultRunCmdInteractive
	ForceColor = false
}

func TestOSDetectionAndOverrides(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"linux", "linux"},
		{"ubuntu", "linux"},
		{"debian", "linux"},
		{"macos", "macos"},
		{"darwin", "macos"},
		{"osx", "macos"},
		{"windows", "windows"},
		{"win32", "windows"},
		{"freebsd", "freebsd"},
	}

	for _, tc := range tests {
		actual := DetectOS(tc.input)
		if actual != tc.expected {
			t.Errorf("DetectOS(%q) = %q; expected %q", tc.input, actual, tc.expected)
		}
	}

	autoOS := DetectOS("auto")
	if autoOS != "linux" && autoOS != "macos" && autoOS != "windows" {
		t.Errorf("DetectOS(\"auto\") returned unexpected OS: %q", autoOS)
	}

	emptyOS := DetectOS("")
	if emptyOS != "linux" && emptyOS != "macos" && emptyOS != "windows" {
		t.Errorf("DetectOS(\"\") returned unexpected OS: %q", emptyOS)
	}
}

func TestToolchainsCatalogValidity(t *testing.T) {
	cat, err := LoadToolchainsCatalog("")
	if err != nil {
		t.Fatalf("LoadToolchainsCatalog(\"\") failed: %v", err)
	}

	if cat.Version != "1.0.0" {
		t.Errorf("expected version 1.0.0, got %q", cat.Version)
	}

	expectedTools := []string{
		"git", "gh", "zram", "uv", "node", "starship", "zoxide", "fzf", "eza",
		"go", "docker", "kubectl", "k3s", "helm", "k9s",
	}

	for _, name := range expectedTools {
		tool, ok := cat.Tools[name]
		if !ok {
			t.Errorf("catalog missing expected tool %q", name)
			continue
		}
		if tool.Binary == "" {
			t.Errorf("tool %q missing binary field", name)
		}
		if tool.Profile != "minimal" && tool.Profile != "dev" && tool.Profile != "full" {
			t.Errorf("tool %q has invalid profile %q", name, tool.Profile)
		}
		if tool.Install == nil {
			t.Errorf("tool %q missing install map", name)
		} else {
			if _, ok := tool.Install["linux"]; !ok {
				t.Errorf("tool %q missing linux install cmd", name)
			}
			if _, ok := tool.Install["macos"]; !ok {
				t.Errorf("tool %q missing macos install cmd", name)
			}
			if _, ok := tool.Install["windows"]; !ok {
				t.Errorf("tool %q missing windows install cmd", name)
			}
		}
	}

	// Test loading from explicit file path
	catFile, err := LoadToolchainsCatalog("config/toolchains.json")
	if err != nil {
		t.Fatalf("LoadToolchainsCatalog(\"config/toolchains.json\") failed: %v", err)
	}
	if len(catFile.Tools) < 15 {
		t.Errorf("expected at least 15 tools from file, got %d", len(catFile.Tools))
	}

	// Test non-existent file
	_, errMissing := LoadToolchainsCatalog("/non/existent/path.json")
	if errMissing == nil {
		t.Errorf("expected error loading non-existent file")
	}

	// Test invalid JSON file
	tmpDir := t.TempDir()
	invalidJSONPath := filepath.Join(tmpDir, "invalid.json")
	_ = os.WriteFile(invalidJSONPath, []byte("invalid json"), 0644)
	_, errInvalid := LoadToolchainsCatalog(invalidJSONPath)
	if errInvalid == nil {
		t.Errorf("expected error loading invalid JSON file")
	}
}

func TestProfileFiltering(t *testing.T) {
	cat, err := LoadToolchainsCatalog("")
	if err != nil {
		t.Fatalf("LoadToolchainsCatalog failed: %v", err)
	}

	minimalTools := FilterToolsByProfile(cat, "minimal")
	if _, ok := minimalTools["git"]; !ok {
		t.Errorf("minimal profile should contain git")
	}
	if _, ok := minimalTools["gh"]; !ok {
		t.Errorf("minimal profile should contain gh")
	}
	if _, ok := minimalTools["zram"]; !ok {
		t.Errorf("minimal profile should contain zram")
	}
	if _, ok := minimalTools["uv"]; ok {
		t.Errorf("minimal profile should not contain uv")
	}
	if _, ok := minimalTools["zoxide"]; ok {
		t.Errorf("minimal profile should not contain zoxide")
	}

	devTools := FilterToolsByProfile(cat, "dev")
	if _, ok := devTools["git"]; !ok {
		t.Errorf("dev profile should contain git")
	}
	if _, ok := devTools["uv"]; !ok {
		t.Errorf("dev profile should contain uv")
	}
	if _, ok := devTools["node"]; !ok {
		t.Errorf("dev profile should contain node")
	}
	if _, ok := devTools["starship"]; !ok {
		t.Errorf("dev profile should contain starship")
	}
	if _, ok := devTools["go"]; !ok {
		t.Errorf("dev profile should contain go")
	}
	if _, ok := devTools["docker"]; !ok {
		t.Errorf("dev profile should contain docker")
	}
	if _, ok := devTools["zoxide"]; ok {
		t.Errorf("dev profile should not contain zoxide")
	}

	fullTools := FilterToolsByProfile(cat, "full")
	if _, ok := fullTools["git"]; !ok {
		t.Errorf("full profile should contain git")
	}
	if _, ok := fullTools["uv"]; !ok {
		t.Errorf("full profile should contain uv")
	}
	if _, ok := fullTools["zoxide"]; !ok {
		t.Errorf("full profile should contain zoxide")
	}
	if _, ok := fullTools["fzf"]; !ok {
		t.Errorf("full profile should contain fzf")
	}
	if _, ok := fullTools["eza"]; !ok {
		t.Errorf("full profile should contain eza")
	}
	if _, ok := fullTools["kubectl"]; !ok {
		t.Errorf("full profile should contain kubectl")
	}

	// Unknown profile defaults to dev (minimal + dev)
	defaultTools := FilterToolsByProfile(cat, "unknown-tier")
	if _, ok := defaultTools["git"]; !ok {
		t.Errorf("default profile should contain git")
	}
	if _, ok := defaultTools["uv"]; !ok {
		t.Errorf("default profile should contain uv")
	}
	if _, ok := defaultTools["zoxide"]; ok {
		t.Errorf("default profile should not contain zoxide")
	}

	// Nil catalog handling
	nilTools := FilterToolsByProfile(nil, "dev")
	if len(nilTools) != 0 {
		t.Errorf("nil catalog should return empty map")
	}
}

func TestConfigureShellEnvironment(t *testing.T) {
	tmpDir := t.TempDir()
	bashrc := filepath.Join(tmpDir, ".bashrc")
	profile := filepath.Join(tmpDir, ".profile")
	dbPath := filepath.Join(tmpDir, "dev.db")

	_ = os.WriteFile(bashrc, []byte("# Existing bashrc\nexport TEST=1\n"), 0644)
	_ = os.WriteFile(profile, []byte("# Existing profile\n"), 0644)

	// First run: should inject block and touch dev.db
	changed, msg := ConfigureShellEnvironment(bashrc, profile, dbPath, false)
	if !changed {
		t.Errorf("first run should report changed=true, got false: %s", msg)
	}

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Errorf("dev.db should exist after configure")
	}

	bashrcContent, _ := os.ReadFile(bashrc)
	bashrcStr := string(bashrcContent)
	if !strings.Contains(bashrcStr, EnvBlockStart) || !strings.Contains(bashrcStr, EnvBlockEnd) {
		t.Errorf(".bashrc missing environment block")
	}
	if !strings.Contains(bashrcStr, "GITHUB_PERSONAL_ACCESS_TOKEN") {
		t.Errorf(".bashrc missing GITHUB_PERSONAL_ACCESS_TOKEN")
	}
	if !strings.Contains(bashrcStr, "SQLITE_DEV_DB") {
		t.Errorf(".bashrc missing SQLITE_DEV_DB")
	}
	if !strings.Contains(bashrcStr, "AGENTKIT_AGENTVAULT_PATH") {
		t.Errorf(".bashrc missing AGENTKIT_AGENTVAULT_PATH")
	}
	if strings.Count(bashrcStr, EnvBlockStart) != 1 {
		t.Errorf(".bashrc has multiple start blocks")
	}

	profileContent, _ := os.ReadFile(profile)
	if !strings.Contains(string(profileContent), EnvBlockStart) {
		t.Errorf(".profile missing environment block")
	}

	// Second run: idempotent
	changed2, msg2 := ConfigureShellEnvironment(bashrc, profile, dbPath, false)
	if changed2 {
		t.Errorf("second run should report changed=false, got true: %s", msg2)
	}

	bashrcContentAfter, _ := os.ReadFile(bashrc)
	if string(bashrcContentAfter) != bashrcStr {
		t.Errorf("content changed unexpectedly on idempotent second run")
	}
}

func TestConfigureShellEnvironmentDryRun(t *testing.T) {
	tmpDir := t.TempDir()
	bashrc := filepath.Join(tmpDir, ".bashrc")
	dbPath := filepath.Join(tmpDir, "dev.db")

	initial := "# Clean bashrc\n"
	_ = os.WriteFile(bashrc, []byte(initial), 0644)

	changed, msg := ConfigureShellEnvironment(bashrc, "", dbPath, true)
	if !changed {
		t.Errorf("dry-run should indicate change would occur: %s", msg)
	}

	if _, err := os.Stat(dbPath); !os.IsNotExist(err) {
		t.Errorf("dev.db should not be created in dry-run mode")
	}

	afterContent, _ := os.ReadFile(bashrc)
	if string(afterContent) != initial {
		t.Errorf("bashrc was modified during dry-run")
	}
}

func TestConfigureShellEnvironmentUpdateOutdated(t *testing.T) {
	tmpDir := t.TempDir()
	bashrc := filepath.Join(tmpDir, ".bashrc")
	dbPath := filepath.Join(tmpDir, "dev.db")

	outdated := fmt.Sprintf("# Before\n%s\nexport OLD_VAR=1\n%s\n# After\n", EnvBlockStart, EnvBlockEnd)
	_ = os.WriteFile(bashrc, []byte(outdated), 0644)

	// Dry run update
	changedDry, msgDry := ConfigureShellEnvironment(bashrc, "", dbPath, true)
	if !changedDry || !strings.Contains(msgDry, "Would update") {
		t.Errorf("expected would update message, got changed=%v msg=%s", changedDry, msgDry)
	}

	// Real update
	changed, msg := ConfigureShellEnvironment(bashrc, "", dbPath, false)
	if !changed || !strings.Contains(msg, "Updated standard env block") {
		t.Errorf("expected updated message, got changed=%v msg=%s", changed, msg)
	}

	updated, _ := os.ReadFile(bashrc)
	if !strings.Contains(string(updated), "AGENTKIT_AGENTVAULT_PATH") {
		t.Errorf("block was not updated properly")
	}
	if !strings.Contains(string(updated), "# Before") || !strings.Contains(string(updated), "# After") {
		t.Errorf("surrounding content was deleted")
	}
}

func TestConfigureShellEnvironmentCreateMissing(t *testing.T) {
	tmpDir := t.TempDir()
	bashrc := filepath.Join(tmpDir, "subdir", ".bashrc")
	dbPath := filepath.Join(tmpDir, "subdir", "dev.db")

	// Dry run create
	changedDry, msgDry := ConfigureShellEnvironment(bashrc, "", dbPath, true)
	if !changedDry || !strings.Contains(msgDry, "Would create") {
		t.Errorf("expected would create message, got: %s", msgDry)
	}
	if _, err := os.Stat(bashrc); !os.IsNotExist(err) {
		t.Errorf("file should not exist after dry run")
	}

	// Real create
	changed, msg := ConfigureShellEnvironment(bashrc, "", dbPath, false)
	if !changed || !strings.Contains(msg, "Created and injected") {
		t.Errorf("expected created message, got: %s", msg)
	}
	if _, err := os.Stat(bashrc); os.IsNotExist(err) {
		t.Errorf("file should exist after real run")
	}
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Errorf("dbPath should exist after real run")
	}
}

func TestCheckSwapStatusNonLinux(t *testing.T) {
	defer resetGlobals()

	okMac, msgMac := CheckSwapStatus("macos")
	if !okMac || msgMac != "Native compressed memory paging active" {
		t.Errorf("macos swap status failed: ok=%v msg=%s", okMac, msgMac)
	}

	okWin, msgWin := CheckSwapStatus("windows")
	if !okWin || msgWin != "Native compressed memory paging active" {
		t.Errorf("windows swap status failed: ok=%v msg=%s", okWin, msgWin)
	}
}

func TestCheckSwapStatusLinuxProcSwaps(t *testing.T) {
	defer resetGlobals()

	tmpDir := t.TempDir()
	swapsFile := filepath.Join(tmpDir, "swaps")
	content := "Filename\tType\tSize\tUsed\tPriority\n/dev/zram0\tpartition\t8388604\t0\t100\n"
	_ = os.WriteFile(swapsFile, []byte(content), 0644)
	ProcSwapsPath = swapsFile

	ok, msg := CheckSwapStatus("linux")
	if !ok || msg != "Active swap/zram detected" {
		t.Errorf("linux active swap failed: ok=%v msg=%s", ok, msg)
	}
}

func TestCheckSwapStatusLinuxNoSwap(t *testing.T) {
	defer resetGlobals()

	tmpDir := t.TempDir()
	swapsFile := filepath.Join(tmpDir, "swaps_empty")
	content := "Filename\tType\tSize\tUsed\tPriority\n"
	_ = os.WriteFile(swapsFile, []byte(content), 0644)
	ProcSwapsPath = swapsFile

	RunCommand = func(name string, args ...string) (string, string, int, error) {
		return "", "", 0, nil
	}

	ok, msg := CheckSwapStatus("linux")
	if ok || !strings.Contains(msg, "No active swap or zram detected") {
		t.Errorf("linux no swap failed: ok=%v msg=%s", ok, msg)
	}
}

func TestCheckSwapStatusLinuxSwaponFallback(t *testing.T) {
	defer resetGlobals()

	ProcSwapsPath = "/non/existent/proc/swaps"

	RunCommand = func(name string, args ...string) (string, string, int, error) {
		if name == "swapon" {
			return "NAME      TYPE      SIZE USED PRIO\n/dev/zram0 partition 7.7G   0B  100\n", "", 0, nil
		}
		return "", "", 1, nil
	}

	ok, msg := CheckSwapStatus("linux")
	if !ok || msg != "Active swap/zram detected" {
		t.Errorf("linux swapon fallback failed: ok=%v msg=%s", ok, msg)
	}
}

func TestResolveAgentVaultPathHierarchy(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. Explicit path
	explicit := filepath.Join(tmpDir, "explicit-vault")
	resolved := ResolveAgentVaultPath(explicit)
	if resolved != explicit {
		t.Errorf("expected explicit path %s, got %s", explicit, resolved)
	}

	// 2. AGENTKIT_AGENTVAULT_PATH
	primaryEnv := filepath.Join(tmpDir, "primary-vault")
	t.Setenv("AGENTKIT_AGENTVAULT_PATH", primaryEnv)
	t.Setenv("AGENTKIT_VAULT_PATH", "/ignored")
	resolvedPrimary := ResolveAgentVaultPath("")
	if resolvedPrimary != primaryEnv {
		t.Errorf("expected primary env path %s, got %s", primaryEnv, resolvedPrimary)
	}

	// 3. AGENTKIT_VAULT_PATH fallback
	secondaryEnv := filepath.Join(tmpDir, "secondary-vault")
	t.Setenv("AGENTKIT_AGENTVAULT_PATH", "")
	t.Setenv("AGENTKIT_VAULT_PATH", secondaryEnv)
	resolvedSecondary := ResolveAgentVaultPath("")
	if resolvedSecondary != secondaryEnv {
		t.Errorf("expected secondary env path %s, got %s", secondaryEnv, resolvedSecondary)
	}

	// 4. Default fallback
	t.Setenv("AGENTKIT_AGENTVAULT_PATH", "")
	t.Setenv("AGENTKIT_VAULT_PATH", "")
	resolvedDefault := ResolveAgentVaultPath("")
	if resolvedDefault != DefaultAgentVaultPath() {
		t.Errorf("expected default path %s, got %s", DefaultAgentVaultPath(), resolvedDefault)
	}
}

func TestEnsureObsidianGitignore(t *testing.T) {
	tmpDir := t.TempDir()
	vault := filepath.Join(tmpDir, "TestVault")
	_ = os.MkdirAll(vault, 0755)

	// File does not exist initially
	err := EnsureObsidianGitignore(vault)
	if err != nil {
		t.Fatalf("EnsureObsidianGitignore failed: %v", err)
	}

	gitignorePath := filepath.Join(vault, ".gitignore")
	data, _ := os.ReadFile(gitignorePath)
	content := string(data)
	for _, entry := range ObsidianGitignoreEntries {
		if !strings.Contains(content, entry) {
			t.Errorf("missing gitignore entry %s", entry)
		}
	}

	// Existing file with partial entries
	_ = os.WriteFile(gitignorePath, []byte(".obsidian/workspace.json\n# Custom comment\n"), 0644)
	err2 := EnsureObsidianGitignore(vault)
	if err2 != nil {
		t.Fatalf("EnsureObsidianGitignore on existing failed: %v", err2)
	}
	data2, _ := os.ReadFile(gitignorePath)
	content2 := string(data2)
	if strings.Count(content2, ".obsidian/workspace.json") != 1 {
		t.Errorf(".obsidian/workspace.json was duplicated")
	}
	if !strings.Contains(content2, "# Custom comment") {
		t.Errorf("existing comments preserved")
	}
	for _, entry := range ObsidianGitignoreEntries {
		if !strings.Contains(content2, entry) {
			t.Errorf("missing gitignore entry %s", entry)
		}
	}
}

func TestEnsureAgentVaultSetupDryRun(t *testing.T) {
	tmpDir := t.TempDir()

	// A: Non-existent vault
	vaultA := filepath.Join(tmpDir, "DryRunNonExistent")
	changedA, msgA := EnsureAgentVaultSetup(vaultA, "", true)
	if !changedA || !strings.Contains(msgA, "Would clone") {
		t.Errorf("vaultA failed: changed=%v msg=%s", changedA, msgA)
	}

	// B: Existing vault without .git
	vaultB := filepath.Join(tmpDir, "DryRunUntracked")
	_ = os.MkdirAll(vaultB, 0755)
	_ = os.WriteFile(filepath.Join(vaultB, "note.md"), []byte("hello"), 0644)
	changedB, msgB := EnsureAgentVaultSetup(vaultB, "", true)
	if !changedB || !strings.Contains(msgB, "Would initialize git repository") {
		t.Errorf("vaultB failed: changed=%v msg=%s", changedB, msgB)
	}

	// C: Existing vault with .git
	vaultC := filepath.Join(tmpDir, "DryRunTracked")
	_ = os.MkdirAll(filepath.Join(vaultC, ".git"), 0755)
	changedC, msgC := EnsureAgentVaultSetup(vaultC, "", true)
	if changedC || !strings.Contains(msgC, "Would sync git-backed") {
		t.Errorf("vaultC failed: changed=%v msg=%s", changedC, msgC)
	}
}

func TestEnsureAgentVaultSetupGuardedFallbackOffline(t *testing.T) {
	defer resetGlobals()

	tmpDir := t.TempDir()
	vaultDir := filepath.Join(tmpDir, "OfflineVault")

	// Mock RunCommand to fail on clone and succeed on git init
	RunCommand = func(name string, args ...string) (string, string, int, error) {
		cmdStr := strings.Join(args, " ")
		if strings.Contains(cmdStr, "clone") {
			return "", "Network unreachable", 1, fmt.Errorf("exit 1")
		}
		if name == "git" && strings.Contains(cmdStr, "init") {
			_ = os.MkdirAll(filepath.Join(vaultDir, ".git"), 0755)
			return "Initialized empty Git repository", "", 0, nil
		}
		return "", "", 0, nil
	}

	changed, msg := EnsureAgentVaultSetup(vaultDir, "", false)
	if !changed || !strings.Contains(msg, "Initialized local git-backed AgentVault") || !strings.Contains(msg, "(offline/fallback mode)") {
		t.Errorf("offline guarded fallback failed: changed=%v msg=%s", changed, msg)
	}

	if _, err := os.Stat(filepath.Join(vaultDir, ".git")); os.IsNotExist(err) {
		t.Errorf(".git directory was not created")
	}

	gitignorePath := filepath.Join(vaultDir, ".gitignore")
	if _, err := os.Stat(gitignorePath); os.IsNotExist(err) {
		t.Errorf(".gitignore was not created")
	}
}

func TestEnsureAgentVaultSetupUntrackedExisting(t *testing.T) {
	defer resetGlobals()

	tmpDir := t.TempDir()
	vaultDir := filepath.Join(tmpDir, "ExistingUntracked")
	_ = os.MkdirAll(vaultDir, 0755)
	_ = os.WriteFile(filepath.Join(vaultDir, "note.md"), []byte("# Note"), 0644)

	RunCommand = func(name string, args ...string) (string, string, int, error) {
		cmdStr := strings.Join(args, " ")
		if strings.Contains(cmdStr, "init") {
			_ = os.MkdirAll(filepath.Join(vaultDir, ".git"), 0755)
			return "Initialized", "", 0, nil
		}
		return "", "", 0, nil
	}

	changed, msg := EnsureAgentVaultSetup(vaultDir, "", false)
	if !changed || !strings.Contains(msg, "Initialized git tracking for existing AgentVault") {
		t.Errorf("untracked existing failed: changed=%v msg=%s", changed, msg)
	}

	// Run again now that .git exists
	RunCommand = func(name string, args ...string) (string, string, int, error) {
		cmdStr := strings.Join(args, " ")
		if strings.Contains(cmdStr, "remote get-url origin") {
			return "https://github.com/Cooper-Wallace/AgentVault.git", "", 0, nil
		}
		return "", "", 0, nil
	}
	changed2, msg2 := EnsureAgentVaultSetup(vaultDir, "", false)
	if changed2 || !strings.Contains(msg2, "Verified git-backed AgentVault") {
		t.Errorf("verified existing failed: changed=%v msg=%s", changed2, msg2)
	}
}

func TestEnsureAgentVaultSetupCloneSuccess(t *testing.T) {
	defer resetGlobals()

	tmpDir := t.TempDir()
	vaultDir := filepath.Join(tmpDir, "ClonedVault")

	RunCommand = func(name string, args ...string) (string, string, int, error) {
		cmdStr := strings.Join(args, " ")
		if strings.Contains(cmdStr, "clone") {
			_ = os.MkdirAll(filepath.Join(vaultDir, ".git"), 0755)
			return "Cloning into...", "", 0, nil
		}
		return "", "", 0, nil
	}

	changed, msg := EnsureAgentVaultSetup(vaultDir, "", false)
	if !changed || !strings.Contains(msg, "Cloned git-backed AgentVault") {
		t.Errorf("clone success failed: changed=%v msg=%s", changed, msg)
	}
}

func TestGetAgentkitBin(t *testing.T) {
	defer resetGlobals()

	tmpDir := t.TempDir()

	// 1. LookPath returns agentkit
	LookPath = func(file string) (string, error) {
		if file == "agentkit" {
			return "/usr/bin/agentkit", nil
		}
		return "", exec.ErrNotFound
	}
	bin := GetAgentkitBin()
	if bin != "/usr/bin/agentkit" {
		t.Errorf("expected /usr/bin/agentkit, got %q", bin)
	}

	// 2. ~/.local/bin/agentkit
	LookPath = func(file string) (string, error) {
		return "", exec.ErrNotFound
	}
	UserHomeDir = func() (string, error) {
		return tmpDir, nil
	}
	localBin := filepath.Join(tmpDir, ".local", "bin")
	_ = os.MkdirAll(localBin, 0755)
	agentkitLocal := filepath.Join(localBin, "agentkit")
	_ = os.WriteFile(agentkitLocal, []byte("#!/bin/sh\n"), 0755)

	bin2 := GetAgentkitBin()
	if bin2 != agentkitLocal {
		t.Errorf("expected %s, got %q", agentkitLocal, bin2)
	}

	// 3. Missing
	_ = os.Remove(agentkitLocal)
	bin3 := GetAgentkitBin()
	if bin3 != "" {
		t.Errorf("expected empty string, got %q", bin3)
	}
}

func TestCmdAuditScenarios(t *testing.T) {
	defer resetGlobals()

	tmpDir := t.TempDir()
	bashrc := filepath.Join(tmpDir, ".bashrc")
	_ = os.WriteFile(bashrc, []byte(GetStandardEnvBlock()+"\n"), 0644)
	dbPath := filepath.Join(tmpDir, "dev.db")
	_ = os.WriteFile(dbPath, []byte{}, 0644)

	UserHomeDir = func() (string, error) {
		return tmpDir, nil
	}

	// All tools found
	LookPath = func(file string) (string, error) {
		return "/mock/bin/" + file, nil
	}

	var buf bytes.Buffer
	code := CmdAudit(&buf, AuditOptions{
		OS:         "linux",
		Profile:    "minimal",
		BashrcPath: bashrc,
		Verbose:    true,
	})

	out := buf.String()
	if code != 0 {
		t.Errorf("expected code 0, got %d. Output:\n%s", code, out)
	}
	if !strings.Contains(out, "--- Layer 0: Platform OS & Desktop ---") {
		t.Errorf("missing Layer 0 header")
	}
	if !strings.Contains(out, "--- Layer 1: Developer Toolchains") {
		t.Errorf("missing Layer 1 header")
	}
	if !strings.Contains(out, "--- Layer 2: Shell Environment Bridge") {
		t.Errorf("missing Layer 2 header")
	}
	if !strings.Contains(out, "--- Layer 3: Remote DevOps Governance") {
		t.Errorf("missing Layer 3 header")
	}

	// Missing tool scenario
	LookPath = func(file string) (string, error) {
		if file == "git" {
			return "", exec.ErrNotFound
		}
		return "/mock/bin/" + file, nil
	}

	var bufFail bytes.Buffer
	codeFail := CmdAudit(&bufFail, AuditOptions{
		OS:         "linux",
		Profile:    "minimal",
		BashrcPath: bashrc,
	})
	if codeFail != 1 {
		t.Errorf("expected code 1 on failure, got %d", codeFail)
	}
	if !strings.Contains(bufFail.String(), "Missing components detected") {
		t.Errorf("expected missing components detected message")
	}
}

func TestCmdApplyStages(t *testing.T) {
	defer resetGlobals()

	tmpDir := t.TempDir()
	bashrc := filepath.Join(tmpDir, ".bashrc")
	profile := filepath.Join(tmpDir, ".profile")
	_ = os.WriteFile(bashrc, []byte("# bashrc\n"), 0644)
	_ = os.WriteFile(profile, []byte("# profile\n"), 0644)

	UserHomeDir = func() (string, error) {
		return tmpDir, nil
	}

	LookPath = func(file string) (string, error) {
		if file == "agentkit" {
			return "/usr/bin/agentkit", nil
		}
		return "/mock/bin/" + file, nil
	}

	executedCommands := []string{}
	RunCmdInteractive = func(out io.Writer, cmdStr string, dryRun bool) int {
		executedCommands = append(executedCommands, cmdStr)
		return 0
	}

	// Apply stage 2 (env)
	var bufStage2 bytes.Buffer
	code2 := CmdApply(&bufStage2, ApplyOptions{
		OS:          "linux",
		Profile:     "minimal",
		Stage:       "2",
		BashrcPath:  bashrc,
		ProfilePath: profile,
		DryRun:      false,
	})
	if code2 != 0 {
		t.Errorf("apply stage 2 failed with code %d", code2)
	}
	if !strings.Contains(bufStage2.String(), "Stage 2: Shell Environment Bridge") {
		t.Errorf("missing Stage 2 output")
	}

	// Apply stage 3 (AI governance)
	var bufStage3 bytes.Buffer
	code3 := CmdApply(&bufStage3, ApplyOptions{
		OS:          "linux",
		Profile:     "minimal",
		Stage:       "3",
		BashrcPath:  bashrc,
		ProfilePath: profile,
		DryRun:      true,
	})
	if code3 != 0 {
		t.Errorf("apply stage 3 dry-run failed with code %d", code3)
	}
	if !strings.Contains(bufStage3.String(), "Stage 3: AI Agent Governance") {
		t.Errorf("missing Stage 3 output")
	}

	// Apply stage 0 with desktop bootstrap on Linux full
	desktopDir := filepath.Join(tmpDir, "Projects", "riced-kubuntu-os")
	_ = os.MkdirAll(desktopDir, 0755)
	bootstrapPath := filepath.Join(desktopDir, "bootstrap.sh")
	_ = os.WriteFile(bootstrapPath, []byte("#!/bin/sh\n"), 0755)

	var bufStage0 bytes.Buffer
	code0 := CmdApply(&bufStage0, ApplyOptions{
		OS:      "linux",
		Profile: "full",
		Stage:   "0",
		DryRun:  true,
	})
	if code0 != 0 {
		t.Errorf("apply stage 0 failed with code %d", code0)
	}
	if !strings.Contains(bufStage0.String(), "desktop bootstrap") {
		t.Errorf("missing desktop bootstrap message in stage 0")
	}

	// Apply stage 1 with missing tools and --install-missing
	LookPath = func(file string) (string, error) {
		return "", exec.ErrNotFound
	}
	var bufStage1 bytes.Buffer
	code1 := CmdApply(&bufStage1, ApplyOptions{
		OS:             "linux",
		Profile:        "minimal",
		Stage:          "1",
		InstallMissing: true,
		DryRun:         true,
	})
	if code1 != 0 {
		t.Errorf("apply stage 1 failed with code %d", code1)
	}
	if !strings.Contains(bufStage1.String(), "Would install") {
		t.Errorf("missing install command in stage 1 dry-run")
	}

	// Apply stage 4
	var bufStage4 bytes.Buffer
	code4 := CmdApply(&bufStage4, ApplyOptions{
		OS:    "linux",
		Stage: "4",
	})
	if code4 != 0 {
		t.Errorf("apply stage 4 failed with code %d", code4)
	}
	if !strings.Contains(bufStage4.String(), "Stage 4: Remote DevOps Governance") {
		t.Errorf("missing Stage 4 output")
	}
}

func TestCmdEnv(t *testing.T) {
	tmpDir := t.TempDir()
	bashrc := filepath.Join(tmpDir, ".bashrc")
	profile := filepath.Join(tmpDir, ".profile")
	dbPath := filepath.Join(tmpDir, "dev.db")

	var buf bytes.Buffer
	code := CmdEnv(&buf, EnvOptions{
		BashrcPath:  bashrc,
		ProfilePath: profile,
		DBPath:      dbPath,
		DryRun:      false,
	})
	if code != 0 {
		t.Errorf("cmd_env failed with code %d", code)
	}
	if !strings.Contains(buf.String(), "[MODIFIED]") {
		t.Errorf("expected [MODIFIED], got: %s", buf.String())
	}

	var buf2 bytes.Buffer
	code2 := CmdEnv(&buf2, EnvOptions{
		BashrcPath:  bashrc,
		ProfilePath: profile,
		DBPath:      dbPath,
		DryRun:      false,
	})
	if code2 != 0 {
		t.Errorf("cmd_env 2 failed with code %d", code2)
	}
	if !strings.Contains(buf2.String(), "[UP-TO-DATE]") {
		t.Errorf("expected [UP-TO-DATE], got: %s", buf2.String())
	}
}

func TestCmdToolchains(t *testing.T) {
	defer resetGlobals()

	// All installed
	LookPath = func(file string) (string, error) {
		return "/mock/bin/" + file, nil
	}

	var buf bytes.Buffer
	code := CmdToolchains(&buf, ToolchainsOptions{
		OS:      "linux",
		Profile: "minimal",
		Install: false,
		DryRun:  false,
	})
	if code != 0 {
		t.Errorf("expected code 0 when all installed, got %d", code)
	}
	if !strings.Contains(buf.String(), "[installed]") {
		t.Errorf("missing [installed] in output")
	}

	// Missing tools without install
	LookPath = func(file string) (string, error) {
		return "", exec.ErrNotFound
	}
	var bufMissing bytes.Buffer
	codeMissing := CmdToolchains(&bufMissing, ToolchainsOptions{
		OS:      "linux",
		Profile: "minimal",
		Install: false,
		DryRun:  false,
	})
	if codeMissing != 1 {
		t.Errorf("expected code 1 when missing tools and install=false, got %d", codeMissing)
	}
	if !strings.Contains(bufMissing.String(), "missing. Run with --install to install.") {
		t.Errorf("expected missing message in output")
	}

	// Missing tools with install and dry-run
	var bufInstall bytes.Buffer
	codeInstall := CmdToolchains(&bufInstall, ToolchainsOptions{
		OS:      "linux",
		Profile: "minimal",
		Install: true,
		DryRun:  true,
	})
	if codeInstall != 0 {
		t.Errorf("expected code 0 with install=true, got %d", codeInstall)
	}
	if !strings.Contains(bufInstall.String(), "Installing missing toolchains:") {
		t.Errorf("expected Installing missing toolchains: in output")
	}
}

func TestColorizeAndTerminal(t *testing.T) {
	defer resetGlobals()

	var buf bytes.Buffer
	plain := Colorize(&buf, "hello", ColorGreen)
	if plain != "hello" {
		t.Errorf("expected plain text when not terminal, got %q", plain)
	}

	ForceColor = true
	colored := Colorize(&buf, "hello", ColorGreen)
	expected := ColorGreen + "hello" + ColorReset
	if colored != expected {
		t.Errorf("expected %q, got %q", expected, colored)
	}
}

func TestDefaultPaths(t *testing.T) {
	bashrc := DefaultBashrcPath()
	if !strings.HasSuffix(bashrc, ".bashrc") {
		t.Errorf("DefaultBashrcPath() = %s, expected suffix .bashrc", bashrc)
	}

	profile := DefaultProfilePath()
	if !strings.HasSuffix(profile, ".profile") {
		t.Errorf("DefaultProfilePath() = %s, expected suffix .profile", profile)
	}

	devDB := DefaultDBPath()
	if !strings.HasSuffix(devDB, "dev.db") {
		t.Errorf("DefaultDBPath() = %s, expected suffix dev.db", devDB)
	}

	vault := DefaultAgentVaultPath()
	if !strings.HasSuffix(vault, filepath.Join("Documents", "AgentVault")) {
		t.Errorf("DefaultAgentVaultPath() = %s, expected Documents/AgentVault", vault)
	}
}

func TestDefaultRunCommandAndInteractive(t *testing.T) {
	defer resetGlobals()

	stdout, stderr, exitCode, err := defaultRunCommand("echo", "hello")
	if exitCode != 0 || err != nil || strings.TrimSpace(stdout) != "hello" {
		t.Errorf("defaultRunCommand echo failed: exit=%d out=%s err=%v", exitCode, stdout, err)
	}
	_ = stderr

	_, _, exitCodeFail, errFail := defaultRunCommand("sh", "-c", "exit 42")
	if exitCodeFail != 42 || errFail == nil {
		t.Errorf("defaultRunCommand fail expected exit 42, got %d, err=%v", exitCodeFail, errFail)
	}

	var buf bytes.Buffer
	code := defaultRunCmdInteractive(&buf, "echo interactive", false)
	if code != 0 {
		t.Errorf("defaultRunCmdInteractive echo failed: %d", code)
	}
	if !strings.Contains(buf.String(), "Executing: echo interactive") {
		t.Errorf("missing executing line: %s", buf.String())
	}

	codeFail := defaultRunCmdInteractive(&buf, "exit 2", false)
	if codeFail != 2 {
		t.Errorf("defaultRunCmdInteractive exit 2 expected 2, got %d", codeFail)
	}
}

func TestExpandPath(t *testing.T) {
	defer resetGlobals()

	if expandPath("") != "" {
		t.Errorf("expandPath(\"\") should be empty")
	}

	home, _ := os.UserHomeDir()
	expandedTilde := expandPath("~/foo")
	expected := filepath.Join(home, "foo")
	if expandedTilde != expected {
		t.Errorf("expandPath(\"~/foo\") = %s, expected %s", expandedTilde, expected)
	}

	expandedOnlyTilde := expandPath("~")
	if expandedOnlyTilde != home {
		t.Errorf("expandPath(\"~\") = %s, expected %s", expandedOnlyTilde, home)
	}
}

func TestAuditToolchainsCompanions(t *testing.T) {
	defer resetGlobals()

	cat := &Catalog{
		Tools: map[string]ToolSpec{
			"custom": {
				Name:          "Custom",
				Binary:        "custom",
				Profile:       "dev",
				CompanionBins: []string{"comp1", "comp2"},
				Install:       map[string]string{"linux": "apt install custom"},
			},
		},
	}

	// Case 1: Primary exists, companions missing
	LookPath = func(file string) (string, error) {
		if file == "custom" {
			return "/bin/custom", nil
		}
		return "", exec.ErrNotFound
	}

	res := AuditToolchains(cat, "dev", "linux")
	if len(res) != 1 {
		t.Fatalf("expected 1 result, got %d", len(res))
	}
	if res[0].Installed {
		t.Errorf("tool should not be installed when companions are missing")
	}
	if len(res[0].MissingBins) != 2 {
		t.Errorf("expected 2 missing companions, got %v", res[0].MissingBins)
	}

	// Case 2: Primary and companions all exist
	LookPath = func(file string) (string, error) {
		return "/bin/" + file, nil
	}
	res2 := AuditToolchains(cat, "dev", "linux")
	if !res2[0].Installed {
		t.Errorf("tool should be installed when all exist")
	}
}

func TestCmdAuditAdditionalBranches(t *testing.T) {
	defer resetGlobals()

	tmpDir := t.TempDir()
	bashrc := filepath.Join(tmpDir, ".bashrc")
	_ = os.WriteFile(bashrc, []byte(GetStandardEnvBlock()+"\n"), 0644)

	// Target OS macos
	var bufMac bytes.Buffer
	codeMac := CmdAudit(&bufMac, AuditOptions{
		OS:         "macos",
		Profile:    "minimal",
		BashrcPath: bashrc,
	})
	_ = codeMac
	if !strings.Contains(bufMac.String(), "Layer 0 desktop profile skipped on macos") {
		t.Errorf("missing macos layer 0 skip message")
	}

	// Target OS linux with riced-kubuntu-os present
	desktopDir := filepath.Join(tmpDir, "Projects", "riced-kubuntu-os")
	_ = os.MkdirAll(desktopDir, 0755)
	UserHomeDir = func() (string, error) {
		return tmpDir, nil
	}

	// Repokit in ~/.local/bin and gh authenticated
	localBin := filepath.Join(tmpDir, ".local", "bin")
	_ = os.MkdirAll(localBin, 0755)
	_ = os.WriteFile(filepath.Join(localBin, "repokit"), []byte("#!/bin/sh\n"), 0755)

	LookPath = func(file string) (string, error) {
		if file == "gh" {
			return "/bin/gh", nil
		}
		return "/bin/" + file, nil
	}
	RunCommand = func(name string, args ...string) (string, string, int, error) {
		if name == "gh" && len(args) > 1 && args[1] == "status" {
			return "Logged in to github.com", "", 0, nil
		}
		return "", "", 0, nil
	}

	var bufLinux bytes.Buffer
	codeLinux := CmdAudit(&bufLinux, AuditOptions{
		OS:         "linux",
		Profile:    "minimal",
		BashrcPath: bashrc,
	})
	_ = codeLinux
	if !strings.Contains(bufLinux.String(), "Found riced-kubuntu-os repository") {
		t.Errorf("missing found riced-kubuntu-os message")
	}
	if !strings.Contains(bufLinux.String(), "repokit available:") {
		t.Errorf("missing repokit available message")
	}
	if !strings.Contains(bufLinux.String(), "GitHub CLI authenticated") {
		t.Errorf("missing GitHub CLI authenticated message")
	}
}

func TestCmdApplyAdditionalBranches(t *testing.T) {
	defer resetGlobals()

	tmpDir := t.TempDir()
	bashrc := filepath.Join(tmpDir, ".bashrc")
	profile := filepath.Join(tmpDir, ".profile")
	_ = os.WriteFile(bashrc, []byte("# bashrc\n"), 0644)
	_ = os.WriteFile(profile, []byte("# profile\n"), 0644)

	UserHomeDir = func() (string, error) {
		return tmpDir, nil
	}

	// Non-Linux Stage 0
	var bufStage0Mac bytes.Buffer
	code0Mac := CmdApply(&bufStage0Mac, ApplyOptions{
		OS:      "macos",
		Profile: "dev",
		Stage:   "0",
	})
	if code0Mac != 0 || !strings.Contains(bufStage0Mac.String(), "Stage 0 skipped") {
		t.Errorf("expected Stage 0 skipped on macos")
	}

	// Linux Stage 0 with missing bootstrap.sh
	var bufStage0NoBoot bytes.Buffer
	code0NoBoot := CmdApply(&bufStage0NoBoot, ApplyOptions{
		OS:      "linux",
		Profile: "full",
		Stage:   "0",
	})
	if code0NoBoot != 0 || !strings.Contains(bufStage0NoBoot.String(), "bootstrap script not found") {
		t.Errorf("expected bootstrap script not found message")
	}

	// Stage 1 with all installed
	LookPath = func(file string) (string, error) {
		return "/bin/" + file, nil
	}
	var bufStage1All bytes.Buffer
	code1All := CmdApply(&bufStage1All, ApplyOptions{
		OS:      "linux",
		Profile: "minimal",
		Stage:   "1",
	})
	if code1All != 0 || !strings.Contains(bufStage1All.String(), "All required toolchains") {
		t.Errorf("expected all required toolchains message")
	}

	// Stage 1 with missing toolchains, no install flag
	LookPath = func(file string) (string, error) {
		return "", exec.ErrNotFound
	}
	var bufStage1Missing bytes.Buffer
	code1Missing := CmdApply(&bufStage1Missing, ApplyOptions{
		OS:             "linux",
		Profile:        "minimal",
		Stage:          "1",
		InstallMissing: false,
	})
	if code1Missing != 0 || !strings.Contains(bufStage1Missing.String(), "Use --install-missing") {
		t.Errorf("expected Use --install-missing note")
	}

	// Stage all with dryRun = false
	LookPath = func(file string) (string, error) {
		if file == "agentkit" {
			return "/bin/agentkit", nil
		}
		if file == "repokit" {
			return "/bin/repokit", nil
		}
		return "/bin/" + file, nil
	}
	RunCmdInteractive = func(out io.Writer, cmdStr string, dryRun bool) int {
		return 0
	}
	var bufStageAll bytes.Buffer
	codeAll := CmdApply(&bufStageAll, ApplyOptions{
		OS:          "linux",
		Profile:     "minimal",
		Stage:       "all",
		BashrcPath:  bashrc,
		ProfilePath: profile,
		DryRun:      false,
	})
	if codeAll != 0 {
		t.Errorf("apply stage all failed: %d", codeAll)
	}
	if !strings.Contains(bufStageAll.String(), "Workstation orchestration completed successfully.") {
		t.Errorf("expected orchestration completed successfully message")
	}
}
