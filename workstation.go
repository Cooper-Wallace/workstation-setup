package workstation

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"
)

// Version of workstation-setup.
const Version = "1.0.0"

// Canonical shell environment block markers.
const (
	EnvBlockStart = "# >>> Antigravity Standards Environment >>>"
	EnvBlockEnd   = "# <<< Antigravity Standards Environment <<<"
)

// Default repository slug for Obsidian AgentVault.
const DefaultAgentVaultSlug = "Cooper-Wallace/AgentVault"

// ObsidianGitignoreEntries defines files ignored inside Obsidian AgentVault.
var ObsidianGitignoreEntries = []string{
	".obsidian/workspace.json",
	".obsidian/workspace-mobile.json",
	".obsidian/cache",
	".trash/",
	".DS_Store",
}

// OrderedToolKeys defines canonical deterministic ordering for tools.
var OrderedToolKeys = []string{
	"git",
	"gh",
	"zram",
	"uv",
	"node",
	"starship",
	"zoxide",
	"fzf",
	"eza",
	"go",
	"docker",
	"kubectl",
	"k3s",
	"helm",
	"k9s",
}

// ANSI styling constants.
const (
	ColorReset  = "\033[0m"
	ColorBold   = "\033[1m"
	ColorDim    = "\033[2m"
	ColorRed    = "\033[91m"
	ColorGreen  = "\033[92m"
	ColorYellow = "\033[93m"
	ColorBlue   = "\033[94m"
	ColorCyan   = "\033[96m"
)

// Injectable variables for hermetic testing.
var (
	LookPath      = exec.LookPath
	ProcSwapsPath = "/proc/swaps"
	UserHomeDir   = os.UserHomeDir
	ForceColor    = false
)

// RunCommandFunc is the signature for command runners.
type RunCommandFunc func(name string, args ...string) (stdout string, stderr string, exitCode int, err error)

// RunCommand is injectable for hermetic unit testing.
var RunCommand RunCommandFunc = defaultRunCommand

func defaultRunCommand(name string, args ...string) (string, string, int, error) {
	cmd := exec.Command(name, args...)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}
	return outBuf.String(), errBuf.String(), exitCode, err
}

// RunCmdInteractiveFunc is the signature for interactive command runners.
type RunCmdInteractiveFunc func(out io.Writer, cmdStr string, dryRun bool) int

// RunCmdInteractive is injectable for testing commands executed in apply/toolchains.
var RunCmdInteractive RunCmdInteractiveFunc = defaultRunCmdInteractive

func defaultRunCmdInteractive(out io.Writer, cmdStr string, dryRun bool) int {
	if dryRun {
		fmt.Fprintf(out, "  %s Would execute: %s\n", Colorize(out, "[DRY-RUN]", ColorYellow), cmdStr)
		return 0
	}
	fmt.Fprintf(out, "  %s Executing: %s\n", Colorize(out, "[RUN]", ColorCyan), cmdStr)
	cmd := exec.Command("sh", "-c", cmdStr)
	cmd.Stdout = out
	cmd.Stderr = out
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode()
		}
		return 1
	}
	return 0
}

// ToolSpec represents declarative metadata for a toolchain.
type ToolSpec struct {
	Name          string            `json:"name"`
	Binary        string            `json:"binary"`
	Profile       string            `json:"profile"`
	Category      string            `json:"category"`
	Description   string            `json:"description"`
	CompanionBins []string          `json:"companion_bins"`
	Install       map[string]string `json:"install"`
}

// Catalog represents the full toolchains catalog structure.
type Catalog struct {
	Schema      string              `json:"$schema,omitempty"`
	Version     string              `json:"version"`
	Description string              `json:"description"`
	Tools       map[string]ToolSpec `json:"tools"`
}

// ToolAuditResult represents audit status of a single tool.
type ToolAuditResult struct {
	Key           string   `json:"key"`
	Name          string   `json:"name"`
	Binary        string   `json:"binary"`
	BinaryPath    string   `json:"binary_path,omitempty"`
	CompanionBins []string `json:"companion_bins"`
	MissingBins   []string `json:"missing_bins"`
	Installed     bool     `json:"installed"`
	InstallCmd    string   `json:"install_cmd"`
	Profile       string   `json:"profile"`
	Category      string   `json:"category"`
	Description   string   `json:"description"`
}

// IsTerminal checks if the writer is an interactive terminal.
func IsTerminal(w io.Writer) bool {
	if ForceColor {
		return true
	}
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// Colorize returns ANSI styled string if writer is a terminal or ForceColor is set.
func Colorize(w io.Writer, text, color string) string {
	if !IsTerminal(w) {
		return text
	}
	return fmt.Sprintf("%s%s%s", color, text, ColorReset)
}

// DetectOS detects running operating system normalized to 'linux', 'macos', or 'windows'.
func DetectOS(override string) string {
	if override != "" && !strings.EqualFold(override, "auto") {
		val := strings.TrimSpace(strings.ToLower(override))
		switch val {
		case "darwin", "osx", "macos":
			return "macos"
		case "win32", "windows":
			return "windows"
		case "linux", "ubuntu", "debian":
			return "linux"
		default:
			return val
		}
	}

	goos := strings.ToLower(runtime.GOOS)
	switch goos {
	case "darwin":
		return "macos"
	case "windows":
		return "windows"
	default:
		return "linux"
	}
}

// DefaultBashrcPath returns default ~/.bashrc path.
func DefaultBashrcPath() string {
	home, err := UserHomeDir()
	if err != nil {
		return ".bashrc"
	}
	return filepath.Join(home, ".bashrc")
}

// DefaultProfilePath returns default ~/.profile path.
func DefaultProfilePath() string {
	home, err := UserHomeDir()
	if err != nil {
		return ".profile"
	}
	return filepath.Join(home, ".profile")
}

// DefaultDBPath returns default ~/dev.db path.
func DefaultDBPath() string {
	home, err := UserHomeDir()
	if err != nil {
		return "dev.db"
	}
	return filepath.Join(home, "dev.db")
}

// DefaultAgentVaultPath returns default ~/Documents/AgentVault path.
func DefaultAgentVaultPath() string {
	home, err := UserHomeDir()
	if err != nil {
		return filepath.Join("Documents", "AgentVault")
	}
	return filepath.Join(home, "Documents", "AgentVault")
}

// LoadToolchainsCatalog loads the toolchains catalog from configPath or embedded JSON.
func LoadToolchainsCatalog(configPath string) (*Catalog, error) {
	var data []byte
	var err error

	if configPath != "" {
		data, err = os.ReadFile(configPath)
		if err != nil {
			return nil, fmt.Errorf("toolchains catalog not found at: %s: %w", configPath, err)
		}
	} else {
		if len(EmbeddedToolchainsJSON) > 0 {
			data = EmbeddedToolchainsJSON
		} else {
			data, err = os.ReadFile("config/toolchains.json")
			if err != nil {
				return nil, fmt.Errorf("toolchains catalog not found: %w", err)
			}
		}
	}

	var cat Catalog
	if err := json.Unmarshal(data, &cat); err != nil {
		return nil, fmt.Errorf("failed to parse toolchains catalog JSON: %w", err)
	}
	return &cat, nil
}

// FilterToolsByProfile filters tools catalog according to cumulative profile tier.
func FilterToolsByProfile(catalog *Catalog, profile string) map[string]ToolSpec {
	profileLower := strings.ToLower(strings.TrimSpace(profile))
	var allowed map[string]bool
	switch profileLower {
	case "minimal":
		allowed = map[string]bool{"minimal": true}
	case "dev":
		allowed = map[string]bool{"minimal": true, "dev": true}
	case "full":
		allowed = map[string]bool{"minimal": true, "dev": true, "full": true}
	default:
		allowed = map[string]bool{"minimal": true, "dev": true}
	}

	res := make(map[string]ToolSpec)
	if catalog == nil {
		return res
	}
	for k, tool := range catalog.Tools {
		toolProfile := tool.Profile
		if toolProfile == "" {
			toolProfile = "minimal"
		}
		if allowed[toolProfile] {
			res[k] = tool
		}
	}
	return res
}

// GetOrderedToolKeys returns tools keys in canonical order, followed by extra keys sorted alphabetically.
func GetOrderedToolKeys(tools map[string]ToolSpec) []string {
	var ordered []string
	seen := make(map[string]bool)

	for _, k := range OrderedToolKeys {
		if _, ok := tools[k]; ok {
			ordered = append(ordered, k)
			seen[k] = true
		}
	}

	var extras []string
	for k := range tools {
		if !seen[k] {
			extras = append(extras, k)
		}
	}
	sort.Strings(extras)
	ordered = append(ordered, extras...)
	return ordered
}

// AuditToolchains audits system for presence of tools in the specified profile tier.
func AuditToolchains(catalog *Catalog, profile, targetOS string) []ToolAuditResult {
	filtered := FilterToolsByProfile(catalog, profile)
	keys := GetOrderedToolKeys(filtered)
	results := make([]ToolAuditResult, 0, len(keys))

	for _, key := range keys {
		tool := filtered[key]
		binary := tool.Binary
		if binary == "" {
			binary = key
		}

		primaryPath, err := LookPath(binary)
		var missingCompanions []string
		for _, comp := range tool.CompanionBins {
			if _, compErr := LookPath(comp); compErr != nil {
				missingCompanions = append(missingCompanions, comp)
			}
		}

		installed := (err == nil && primaryPath != "") && len(missingCompanions) == 0

		installCmd := ""
		if tool.Install != nil {
			installCmd = tool.Install[targetOS]
		}

		name := tool.Name
		if name == "" {
			name = key
		}
		prof := tool.Profile
		if prof == "" {
			prof = "minimal"
		}
		cat := tool.Category
		if cat == "" {
			cat = "general"
		}

		results = append(results, ToolAuditResult{
			Key:           key,
			Name:          name,
			Binary:        binary,
			BinaryPath:    primaryPath,
			CompanionBins: tool.CompanionBins,
			MissingBins:   missingCompanions,
			Installed:     installed,
			InstallCmd:    installCmd,
			Profile:       prof,
			Category:      cat,
			Description:   tool.Description,
		})
	}

	return results
}

// CheckSwapStatus checks active swap and zram buffer configuration.
func CheckSwapStatus(targetOS string) (bool, string) {
	var currentOS string
	if targetOS != "" {
		currentOS = DetectOS(targetOS)
	} else {
		currentOS = DetectOS("auto")
	}

	if currentOS != "linux" {
		return true, "Native compressed memory paging active"
	}

	// Check ProcSwapsPath
	if data, err := os.ReadFile(ProcSwapsPath); err == nil {
		lines := splitNonEmptyLines(string(data))
		if len(lines) > 1 {
			return true, "Active swap/zram detected"
		}
	}

	// Fallback to swapon --show
	if stdout, _, exitCode, _ := RunCommand("swapon", "--show"); exitCode == 0 && strings.TrimSpace(stdout) != "" {
		lines := splitNonEmptyLines(stdout)
		if len(lines) > 1 || (len(lines) == 1 && !strings.HasPrefix(lines[0], "NAME")) {
			return true, "Active swap/zram detected"
		}
	}

	return false, "No active swap or zram detected (high risk of instantaneous OOM freeze)"
}

func splitNonEmptyLines(s string) []string {
	scanner := bufio.NewScanner(strings.NewReader(s))
	var lines []string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func expandPath(p string) string {
	if p == "" {
		return ""
	}
	if p == "~" || strings.HasPrefix(p, "~/") {
		home, err := UserHomeDir()
		if err == nil {
			if p == "~" {
				p = home
			} else {
				p = filepath.Join(home, p[2:])
			}
		}
	}
	abs, err := filepath.Abs(p)
	if err == nil {
		return abs
	}
	return filepath.Clean(p)
}

// ResolveAgentVaultPath resolves Obsidian AgentVault path following priority rules.
func ResolveAgentVaultPath(explicitPath string) string {
	if explicitPath != "" {
		return expandPath(explicitPath)
	}
	if envVal := os.Getenv("AGENTKIT_AGENTVAULT_PATH"); envVal != "" {
		return expandPath(envVal)
	}
	if envVal := os.Getenv("AGENTKIT_VAULT_PATH"); envVal != "" {
		return expandPath(envVal)
	}
	return DefaultAgentVaultPath()
}

// GetAgentkitBin resolves agentkit binary from PATH or ~/.local/bin.
func GetAgentkitBin() string {
	if binInPath, err := LookPath("agentkit"); err == nil && binInPath != "" {
		return binInPath
	}
	home, err := UserHomeDir()
	if err == nil {
		localBin := filepath.Join(home, ".local", "bin", "agentkit")
		if fi, err := os.Stat(localBin); err == nil && !fi.IsDir() {
			return localBin
		}
	}
	return ""
}

// EnsureObsidianGitignore ensures .gitignore inside vault has necessary Obsidian workspace ignores.
func EnsureObsidianGitignore(vaultPath string) error {
	gitignoreFile := filepath.Join(vaultPath, ".gitignore")
	data, err := os.ReadFile(gitignoreFile)
	if os.IsNotExist(err) {
		content := strings.Join(ObsidianGitignoreEntries, "\n") + "\n"
		return os.WriteFile(gitignoreFile, []byte(content), 0644)
	}
	content := string(data)
	existingLines := make(map[string]bool)
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			existingLines[trimmed] = true
		}
	}

	var missing []string
	for _, entry := range ObsidianGitignoreEntries {
		if !existingLines[entry] {
			missing = append(missing, entry)
		}
	}

	if len(missing) > 0 {
		prefix := ""
		if len(content) > 0 && !strings.HasSuffix(content, "\n") {
			prefix = "\n"
		}
		newContent := content + prefix + strings.Join(missing, "\n") + "\n"
		return os.WriteFile(gitignoreFile, []byte(newContent), 0644)
	}
	return nil
}

// EnsureAgentVaultSetup ensures Obsidian AgentVault is provisioned, git-tracked, and synchronized.
func EnsureAgentVaultSetup(vaultPath, remoteSlug string, dryRun bool) (bool, string) {
	targetVault := vaultPath
	if targetVault == "" {
		targetVault = ResolveAgentVaultPath("")
	}
	if remoteSlug == "" {
		remoteSlug = DefaultAgentVaultSlug
	}

	if dryRun {
		if _, err := os.Stat(targetVault); os.IsNotExist(err) {
			return true, fmt.Sprintf("Would clone %s or initialize AgentVault at %s", remoteSlug, targetVault)
		}
		if _, err := os.Stat(filepath.Join(targetVault, ".git")); os.IsNotExist(err) {
			return true, fmt.Sprintf("Would initialize git repository in AgentVault at %s", targetVault)
		}
		return false, fmt.Sprintf("Would sync git-backed AgentVault at %s", targetVault)
	}

	// Case A: targetVault does not exist or directory is empty
	entries, readErr := os.ReadDir(targetVault)
	isCaseA := os.IsNotExist(readErr) || (readErr == nil && len(entries) == 0)

	if isCaseA {
		cloned := false
		if _, err := LookPath("gh"); err == nil {
			_, _, code, _ := RunCommand("gh", "repo", "clone", remoteSlug, targetVault)
			if code == 0 {
				if fi, e := os.Stat(filepath.Join(targetVault, ".git")); e == nil && fi.IsDir() {
					cloned = true
				}
			}
		}

		if !cloned {
			if _, err := LookPath("git"); err == nil {
				cloneURL := fmt.Sprintf("https://github.com/%s.git", remoteSlug)
				_, _, code, _ := RunCommand("git", "clone", cloneURL, targetVault)
				if code == 0 {
					if fi, e := os.Stat(filepath.Join(targetVault, ".git")); e == nil && fi.IsDir() {
						cloned = true
					}
				}
			}
		}

		if cloned {
			_ = EnsureObsidianGitignore(targetVault)
			return true, fmt.Sprintf("Cloned git-backed AgentVault (%s) to %s", remoteSlug, targetVault)
		}

		// Guarded Degrade Path: initialize locally
		_ = os.MkdirAll(targetVault, 0755)
		_, _, initCode, _ := RunCommand("git", "-C", targetVault, "init", "-b", "main")
		if initCode != 0 {
			_, _, _, _ = RunCommand("git", "-C", targetVault, "init")
			_, _, _, _ = RunCommand("git", "-C", targetVault, "branch", "-M", "main")
		}

		_ = EnsureObsidianGitignore(targetVault)

		agentkitBin := GetAgentkitBin()
		if agentkitBin != "" {
			_, _, _, _ = RunCommand(agentkitBin, "vault", "init", "--path", targetVault)
		}

		return true, fmt.Sprintf("Initialized local git-backed AgentVault at %s (offline/fallback mode)", targetVault)
	}

	// Case B: targetVault exists and is not empty
	_ = EnsureObsidianGitignore(targetVault)
	if _, err := os.Stat(filepath.Join(targetVault, ".git")); os.IsNotExist(err) {
		_, _, initCode, _ := RunCommand("git", "-C", targetVault, "init", "-b", "main")
		if initCode != 0 {
			_, _, _, _ = RunCommand("git", "-C", targetVault, "init")
			_, _, _, _ = RunCommand("git", "-C", targetVault, "branch", "-M", "main")
		}
		_, _, _, _ = RunCommand("git", "-C", targetVault, "add", "-A")
		_, _, commitCode, _ := RunCommand("git", "-C", targetVault, "commit", "-m", "chore(vault): initialize git-backed Obsidian AgentVault")
		if commitCode != 0 {
			_, _, _, _ = RunCommand("git", "-C", targetVault, "-c", "user.name=Cooper Wallace", "-c", "user.email=cooper@wallace.dev", "commit", "-m", "chore(vault): initialize git-backed Obsidian AgentVault")
		}
		return true, fmt.Sprintf("Initialized git tracking for existing AgentVault at %s", targetVault)
	}

	// .git exists: pull origin main
	out, _, remoteCode, _ := RunCommand("git", "-C", targetVault, "remote", "get-url", "origin")
	if remoteCode == 0 && strings.TrimSpace(out) != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, "git", "-C", targetVault, "pull", "--rebase", "--autostash", "origin", "main")
		_ = cmd.Run()
	}
	return false, fmt.Sprintf("Verified git-backed AgentVault at %s", targetVault)
}

// GetStandardEnvBlock returns the canonical Antigravity Standards shell environment block.
func GetStandardEnvBlock() string {
	lines := []string{
		EnvBlockStart,
		"# Dynamically resolve GitHub token from gh keyring if not already exported",
		"if command -v gh >/dev/null 2>&1; then",
		"    export GITHUB_PERSONAL_ACCESS_TOKEN=\"${GITHUB_PERSONAL_ACCESS_TOKEN:-$(gh auth token 2>/dev/null)}\"",
		"fi",
		"",
		"# Default SQLite development database path for Antigravity SQLite MCP server",
		"export SQLITE_DEV_DB=\"${SQLITE_DEV_DB:-$HOME/dev.db}\"",
		"",
		"# Default Obsidian AgentVault path for cross-session agent memory and repokit exemptions",
		"export AGENTKIT_AGENTVAULT_PATH=\"${AGENTKIT_AGENTVAULT_PATH:-$HOME/Documents/AgentVault}\"",
		EnvBlockEnd,
	}
	return strings.Join(lines, "\n")
}

// ConfigureShellEnvironment idempotently injects or updates standard environment block.
func ConfigureShellEnvironment(bashrcPath, profilePath, dbPath string, dryRun bool) (bool, string) {
	block := GetStandardEnvBlock()
	changed := false
	var messages []string

	targets := []string{bashrcPath}
	if profilePath != "" {
		if fi, err := os.Stat(profilePath); err == nil && !fi.IsDir() {
			targets = append(targets, profilePath)
		}
	}

	for _, target := range targets {
		if _, err := os.Stat(target); os.IsNotExist(err) {
			if dryRun {
				changed = true
				messages = append(messages, fmt.Sprintf("Would create and inject standard env block into %s", target))
			} else {
				_ = os.MkdirAll(filepath.Dir(target), 0755)
				_ = os.WriteFile(target, []byte(block+"\n"), 0644)
				changed = true
				messages = append(messages, fmt.Sprintf("Created and injected standard env block into %s", target))
			}
			continue
		}

		data, err := os.ReadFile(target)
		if err != nil {
			continue
		}
		content := string(data)

		if strings.Contains(content, EnvBlockStart) && strings.Contains(content, EnvBlockEnd) {
			pattern := regexp.MustCompile("(?s)" + regexp.QuoteMeta(EnvBlockStart) + ".*?" + regexp.QuoteMeta(EnvBlockEnd))
			match := pattern.FindString(content)
			if strings.TrimSpace(match) == strings.TrimSpace(block) {
				messages = append(messages, fmt.Sprintf("Standard env block in %s is up-to-date", target))
			} else {
				if dryRun {
					changed = true
					messages = append(messages, fmt.Sprintf("Would update standard env block in %s", target))
				} else {
					newContent := pattern.ReplaceAllLiteralString(content, block)
					_ = os.WriteFile(target, []byte(newContent), 0644)
					changed = true
					messages = append(messages, fmt.Sprintf("Updated standard env block in %s", target))
				}
			}
		} else {
			if dryRun {
				changed = true
				messages = append(messages, fmt.Sprintf("Would append standard env block to %s", target))
			} else {
				prefix := ""
				if len(content) > 0 && !strings.HasSuffix(content, "\n") {
					prefix = "\n"
				}
				newContent := content + prefix + block + "\n"
				_ = os.WriteFile(target, []byte(newContent), 0644)
				changed = true
				messages = append(messages, fmt.Sprintf("Appended standard env block to %s", target))
			}
		}
	}

	sqliteDB := dbPath
	if sqliteDB == "" {
		sqliteDB = DefaultDBPath()
	}

	if dryRun {
		if _, err := os.Stat(sqliteDB); os.IsNotExist(err) {
			messages = append(messages, fmt.Sprintf("Would touch SQLite dev database at %s", sqliteDB))
		} else {
			messages = append(messages, fmt.Sprintf("SQLite dev database present at %s", sqliteDB))
		}
	} else {
		_ = os.MkdirAll(filepath.Dir(sqliteDB), 0755)
		if _, err := os.Stat(sqliteDB); os.IsNotExist(err) {
			_ = os.WriteFile(sqliteDB, []byte{}, 0644)
			messages = append(messages, fmt.Sprintf("Created SQLite dev database at %s", sqliteDB))
		} else {
			now := time.Now()
			_ = os.Chtimes(sqliteDB, now, now)
			messages = append(messages, fmt.Sprintf("Verified SQLite dev database at %s", sqliteDB))
		}
	}

	return changed, strings.Join(messages, "; ")
}

// AuditOptions holds arguments for CmdAudit.
type AuditOptions struct {
	OS         string
	Profile    string
	BashrcPath string
	Verbose    bool
	ConfigPath string
}

// CmdAudit executes complete 4-layer workstation audit.
func CmdAudit(out io.Writer, args AuditOptions) int {
	targetOS := DetectOS(args.OS)
	profile := strings.ToLower(args.Profile)
	if profile == "" {
		profile = "dev"
	}
	bashrcPath := expandPath(args.BashrcPath)
	if bashrcPath == "" {
		bashrcPath = DefaultBashrcPath()
	}

	fmt.Fprintln(out, Colorize(out, "╔═════════════════════════════════════════════════════════════════╗", ColorBold))
	fmt.Fprintf(out, "%s\n", Colorize(out, fmt.Sprintf("║ Workstation Setup Audit Engine v%-29s║", Version), ColorBold))
	fmt.Fprintln(out, Colorize(out, "╚═════════════════════════════════════════════════════════════════╝", ColorBold))
	fmt.Fprintf(out, "Target OS: %s | Architecture: %s | Profile: %s\n",
		Colorize(out, targetOS, ColorCyan),
		runtime.GOARCH,
		Colorize(out, profile, ColorCyan))
	fmt.Fprintf(out, "Shell RC:  %s\n\n", bashrcPath)

	hasFailures := false

	// Layer 0: Platform OS & Desktop
	fmt.Fprintln(out, Colorize(out, "--- Layer 0: Platform OS & Desktop ---", ColorBold))
	home, _ := UserHomeDir()
	l0Repo := filepath.Join(home, "Projects", "riced-kubuntu-os")
	if targetOS == "linux" {
		if fi, err := os.Stat(l0Repo); err == nil && fi.IsDir() {
			fmt.Fprintf(out, "  %s Found riced-kubuntu-os repository: %s\n", Colorize(out, "✓", ColorGreen), l0Repo)
		} else {
			fmt.Fprintf(out, "  %s riced-kubuntu-os not detected at %s (optional for headless/non-KDE)\n", Colorize(out, "ℹ", ColorBlue), l0Repo)
		}
	} else {
		fmt.Fprintf(out, "  %s Layer 0 desktop profile skipped on %s (Linux specific)\n", Colorize(out, "ℹ", ColorBlue), targetOS)
	}

	// System / Environment checks: Swap & memory buffer status
	swapOK, swapMsg := CheckSwapStatus(targetOS)
	if swapOK {
		fmt.Fprintf(out, "  %s Swap/Memory buffer: %s\n", Colorize(out, "✓", ColorGreen), swapMsg)
	} else {
		fmt.Fprintf(out, "  %s Swap/Memory buffer: %s\n", Colorize(out, "ℹ", ColorYellow), swapMsg)
		if targetOS == "linux" {
			fmt.Fprintf(out, "      To enable zram swap: %s\n", Colorize(out, "sudo apt install -y zram-tools && sudo systemctl enable --now zramswap.service", ColorDim))
		}
	}

	// Layer 1: Developer Toolchains
	fmt.Fprintln(out)
	fmt.Fprintf(out, "%s\n", Colorize(out, fmt.Sprintf("--- Layer 1: Developer Toolchains (%s tier) ---", profile), ColorBold))
	catalog, err := LoadToolchainsCatalog(args.ConfigPath)
	if err != nil {
		fmt.Fprintf(out, "  %s Error loading toolchains: %v\n", Colorize(out, "✗", ColorRed), err)
		return 1
	}

	toolAudits := AuditToolchains(catalog, profile, targetOS)
	installedCount := 0
	totalCount := len(toolAudits)

	for _, item := range toolAudits {
		if item.Installed {
			installedCount++
			pathStr := ""
			if item.BinaryPath != "" {
				pathStr = fmt.Sprintf(" (%s)", item.BinaryPath)
			}
			fmt.Fprintf(out, "  %s %-22s [installed]%s\n", Colorize(out, "✓", ColorGreen), item.Name, pathStr)
		} else {
			hasFailures = true
			missingReason := "missing"
			if len(item.MissingBins) > 0 {
				missingReason = fmt.Sprintf("missing companion bins: %s", strings.Join(item.MissingBins, ", "))
			}
			fmt.Fprintf(out, "  %s %-22s [%s]\n", Colorize(out, "✗", ColorRed), item.Name, missingReason)
			if args.Verbose || !item.Installed {
				fmt.Fprintf(out, "      Install command: %s\n", Colorize(out, item.InstallCmd, ColorDim))
			}
		}
	}
	fmt.Fprintf(out, "  Status: %d/%d toolchains installed for '%s' profile.\n", installedCount, totalCount, profile)

	// Layer 2: Shell Environment Bridge & Agent Governance
	fmt.Fprintln(out)
	fmt.Fprintln(out, Colorize(out, "--- Layer 2: Shell Environment Bridge & Agent Governance ---", ColorBold))
	if data, err := os.ReadFile(bashrcPath); err == nil {
		content := string(data)
		if strings.Contains(content, EnvBlockStart) && strings.Contains(content, EnvBlockEnd) {
			fmt.Fprintf(out, "  %s Managed environment block present in %s\n", Colorize(out, "✓", ColorGreen), bashrcPath)
		} else {
			fmt.Fprintf(out, "  %s Managed environment block MISSING in %s\n", Colorize(out, "✗", ColorRed), bashrcPath)
			hasFailures = true
		}
	} else {
		fmt.Fprintf(out, "  %s Shell config file %s not found\n", Colorize(out, "✗", ColorRed), bashrcPath)
		hasFailures = true
	}

	// Dev DB status
	devDB := DefaultDBPath()
	if _, err := os.Stat(devDB); err == nil {
		fmt.Fprintf(out, "  %s SQLite dev database present: %s\n", Colorize(out, "✓", ColorGreen), devDB)
	} else {
		fmt.Fprintf(out, "  %s SQLite dev database missing: %s (will be created by apply)\n", Colorize(out, "ℹ", ColorYellow), devDB)
	}

	// Obsidian AgentVault status
	vaultDir := ResolveAgentVaultPath("")
	if fi, err := os.Stat(filepath.Join(vaultDir, ".git")); err == nil && fi.IsDir() {
		fmt.Fprintf(out, "  %s Obsidian AgentVault present (git-tracked): %s\n", Colorize(out, "✓", ColorGreen), vaultDir)
	} else if fi, err := os.Stat(vaultDir); err == nil && fi.IsDir() {
		fmt.Fprintf(out, "  %s Obsidian AgentVault present (untracked; run apply to init git): %s\n", Colorize(out, "ℹ", ColorYellow), vaultDir)
	} else {
		fmt.Fprintf(out, "  %s Obsidian AgentVault missing: %s (will be provisioned by apply)\n", Colorize(out, "ℹ", ColorBlue), vaultDir)
	}

	// agentkit status
	agentkitBin := GetAgentkitBin()
	if agentkitBin != "" {
		if inPath, _ := LookPath("agentkit"); inPath != "" && inPath == agentkitBin {
			fmt.Fprintf(out, "  %s agentkit available: %s\n", Colorize(out, "✓", ColorGreen), agentkitBin)
		} else {
			fmt.Fprintf(out, "  %s agentkit found in ~/.local/bin: %s\n", Colorize(out, "✓", ColorGreen), agentkitBin)
		}
	} else {
		fmt.Fprintf(out, "  %s agentkit not installed (run agentkit install.sh)\n", Colorize(out, "ℹ", ColorBlue))
	}

	// Layer 3: Remote DevOps Governance
	fmt.Fprintln(out)
	fmt.Fprintln(out, Colorize(out, "--- Layer 3: Remote DevOps Governance ---", ColorBold))
	repokitBin := ""
	if p, err := LookPath("repokit"); err == nil && p != "" {
		repokitBin = p
	} else {
		localRepokit := filepath.Join(home, ".local", "bin", "repokit")
		if fi, err := os.Stat(localRepokit); err == nil && !fi.IsDir() {
			repokitBin = localRepokit
		}
	}

	if repokitBin != "" {
		fmt.Fprintf(out, "  %s repokit available: %s\n", Colorize(out, "✓", ColorGreen), repokitBin)
	} else {
		fmt.Fprintf(out, "  %s repokit not installed (run repokit install.sh)\n", Colorize(out, "ℹ", ColorBlue))
	}

	if ghBin, err := LookPath("gh"); err == nil && ghBin != "" {
		_, _, authCode, _ := RunCommand("gh", "auth", "status")
		if authCode == 0 {
			fmt.Fprintf(out, "  %s GitHub CLI authenticated\n", Colorize(out, "✓", ColorGreen))
		} else {
			fmt.Fprintf(out, "  %s GitHub CLI installed but not authenticated (`gh auth login`)\n", Colorize(out, "ℹ", ColorYellow))
		}
	}

	fmt.Fprintln(out)
	if hasFailures {
		fmt.Fprintln(out, Colorize(out, "Audit completed: Missing components detected. Run 'workstation-setup apply' to resolve.", ColorYellow))
		return 1
	}
	fmt.Fprintln(out, Colorize(out, "Audit completed: All required workstation specifications verified!", ColorGreen))
	return 0
}

// ApplyOptions holds arguments for CmdApply.
type ApplyOptions struct {
	OS             string
	Profile        string
	Stage          string
	BashrcPath     string
	ProfilePath    string
	InstallMissing bool
	DryRun         bool
	ConfigPath     string
}

// CmdApply executes workstation configuration stages.
func CmdApply(out io.Writer, args ApplyOptions) int {
	targetOS := DetectOS(args.OS)
	profile := strings.ToLower(args.Profile)
	if profile == "" {
		profile = "dev"
	}
	stage := strings.ToLower(args.Stage)
	if stage == "" {
		stage = "all"
	}
	dryRun := args.DryRun
	installMissing := args.InstallMissing
	bashrcPath := expandPath(args.BashrcPath)
	if bashrcPath == "" {
		bashrcPath = DefaultBashrcPath()
	}
	profilePath := expandPath(args.ProfilePath)
	if profilePath == "" {
		profilePath = DefaultProfilePath()
	}

	fmt.Fprintln(out, Colorize(out, "╔═════════════════════════════════════════════════════════════════╗", ColorBold))
	fmt.Fprintf(out, "%s\n", Colorize(out, fmt.Sprintf("║ Workstation Setup Orchestrator v%-28s║", Version), ColorBold))
	fmt.Fprintln(out, Colorize(out, "╚═════════════════════════════════════════════════════════════════╝", ColorBold))
	if dryRun {
		fmt.Fprintln(out, Colorize(out, "Mode: DRY-RUN (No system mutations will be applied)", ColorYellow))
	}
	fmt.Fprintf(out, "Target OS: %s | Profile: %s | Stages: %s\n\n", targetOS, profile, stage)

	home, _ := UserHomeDir()

	// Stage 0: Platform OS & Desktop
	if stage == "all" || stage == "0" {
		fmt.Fprintln(out, Colorize(out, "[Stage 0: Platform OS & Desktop]", ColorBold))
		if targetOS == "linux" && profile == "full" {
			bootstrapScript := filepath.Join(home, "Projects", "riced-kubuntu-os", "bootstrap.sh")
			if fi, err := os.Stat(bootstrapScript); err == nil && !fi.IsDir() {
				if dryRun {
					fmt.Fprintf(out, "  %s Would execute desktop bootstrap: %s\n", Colorize(out, "[DRY-RUN]", ColorYellow), bootstrapScript)
				} else {
					fmt.Fprintf(out, "  Found Kubuntu desktop bootstrap at: %s\n", bootstrapScript)
					fmt.Fprintf(out, "  Run manually when desktop reconfiguration is desired: %s\n", bootstrapScript)
				}
			} else {
				fmt.Fprintf(out, "  %s riced-kubuntu-os bootstrap script not found at %s. Skipping.\n", Colorize(out, "ℹ", ColorBlue), bootstrapScript)
			}
		} else {
			fmt.Fprintf(out, "  %s Stage 0 skipped (active on Linux with 'full' profile only).\n", Colorize(out, "ℹ", ColorBlue))
		}
		fmt.Fprintln(out)
	}

	// Stage 1: Developer Toolchains
	if stage == "all" || stage == "1" {
		fmt.Fprintf(out, "%s\n", Colorize(out, fmt.Sprintf("[Stage 1: Developer Toolchains (%s)]", profile), ColorBold))
		catalog, err := LoadToolchainsCatalog(args.ConfigPath)
		if err != nil {
			fmt.Fprintf(out, "  %s Error loading toolchains: %v\n", Colorize(out, "✗", ColorRed), err)
			return 1
		}
		audits := AuditToolchains(catalog, profile, targetOS)
		var missing []ToolAuditResult
		for _, t := range audits {
			if !t.Installed {
				missing = append(missing, t)
			}
		}

		if len(missing) == 0 {
			fmt.Fprintf(out, "  %s All required toolchains for profile '%s' are already installed.\n", Colorize(out, "✓", ColorGreen), profile)
		} else {
			var missingNames []string
			for _, t := range missing {
				missingNames = append(missingNames, t.Name)
			}
			fmt.Fprintf(out, "  Found %d missing toolchain(s): %s\n", len(missing), strings.Join(missingNames, ", "))
			for _, item := range missing {
				if installMissing {
					if dryRun {
						fmt.Fprintf(out, "  %s Would install %s: %s\n", Colorize(out, "[DRY-RUN]", ColorYellow), item.Name, item.InstallCmd)
					} else {
						fmt.Fprintf(out, "  Installing %s...\n", item.Name)
						RunCmdInteractive(out, item.InstallCmd, false)
					}
				} else {
					fmt.Fprintf(out, "  • %s: %s\n", item.Name, Colorize(out, item.InstallCmd, ColorCyan))
				}
			}
			if !installMissing {
				fmt.Fprintf(out, "  %s Use --install-missing to automatically run toolchain install commands.\n", Colorize(out, "Note:", ColorYellow))
			}
		}
		fmt.Fprintln(out)
	}

	// Stage 2: Shell Environment Bridge
	if stage == "all" || stage == "2" {
		fmt.Fprintln(out, Colorize(out, "[Stage 2: Shell Environment Bridge]", ColorBold))
		changed, msg := ConfigureShellEnvironment(bashrcPath, profilePath, "", dryRun)
		statusLabel := Colorize(out, "VERIFIED", ColorBlue)
		if changed {
			statusLabel = Colorize(out, "MODIFIED", ColorGreen)
		}
		fmt.Fprintf(out, "  [%s] %s\n\n", statusLabel, msg)
	}

	// Stage 3: AI Agent Governance
	if stage == "all" || stage == "3" {
		fmt.Fprintln(out, Colorize(out, "[Stage 3: AI Agent Governance]", ColorBold))
		vaultChanged, vaultMsg := EnsureAgentVaultSetup("", DefaultAgentVaultSlug, dryRun)
		statusLabel := Colorize(out, "VERIFIED", ColorBlue)
		if vaultChanged {
			statusLabel = Colorize(out, "MODIFIED", ColorGreen)
		}
		fmt.Fprintf(out, "  [%s] %s\n", statusLabel, vaultMsg)

		agentkitBin := GetAgentkitBin()
		if agentkitBin != "" {
			cmd := agentkitBin + " apply"
			if dryRun {
				cmd += " --dry-run"
			}
			fmt.Fprintf(out, "  Running AI agent governance and wake-lock configuration via: %s\n", cmd)
			RunCmdInteractive(out, cmd, dryRun)

			syncCmd := agentkitBin + " vault sync"
			if dryRun {
				syncCmd += " --dry-run"
			}
			fmt.Fprintf(out, "  Synchronizing Obsidian AgentVault via: %s\n", syncCmd)
			RunCmdInteractive(out, syncCmd, dryRun)
		} else {
			fmt.Fprintf(out, "  %s agentkit not found in PATH or ~/.local/bin.\n", Colorize(out, "ℹ", ColorBlue))
			fmt.Fprintln(out, "    To synchronize AI agent permissions: cd ~/Projects/agentkit && ./install.sh && agentkit apply")
		}
		fmt.Fprintln(out)
	}

	// Stage 4: Remote DevOps Governance
	if stage == "all" || stage == "4" {
		fmt.Fprintln(out, Colorize(out, "[Stage 4: Remote DevOps Governance]", ColorBold))
		repokitBin := ""
		if p, err := LookPath("repokit"); err == nil && p != "" {
			repokitBin = p
		} else {
			localRepokit := filepath.Join(home, ".local", "bin", "repokit")
			if fi, err := os.Stat(localRepokit); err == nil && !fi.IsDir() {
				repokitBin = localRepokit
			}
		}

		if repokitBin != "" {
			fmt.Fprintf(out, "  %s repokit is ready.\n", Colorize(out, "✓", ColorGreen))
		} else {
			fmt.Fprintf(out, "  %s repokit not found.\n", Colorize(out, "ℹ", ColorBlue))
			fmt.Fprintln(out, "    To install GitHub repo governance: cd ~/Projects/gh-standardise && ./install.sh")
		}
		fmt.Fprintln(out)
	}

	prefix := "Workstation orchestration"
	if dryRun {
		prefix = "Dry-run simulation"
	}
	fmt.Fprintf(out, "%s\n", Colorize(out, fmt.Sprintf("✓ %s completed successfully.", prefix), ColorGreen))
	return 0
}

// EnvOptions holds arguments for CmdEnv.
type EnvOptions struct {
	BashrcPath  string
	ProfilePath string
	DBPath      string
	DryRun      bool
}

// CmdEnv is the standalone runner for Stage 2: Shell Environment Bridge.
func CmdEnv(out io.Writer, args EnvOptions) int {
	bashrcPath := expandPath(args.BashrcPath)
	if bashrcPath == "" {
		bashrcPath = DefaultBashrcPath()
	}
	profilePath := expandPath(args.ProfilePath)
	if profilePath == "" {
		profilePath = DefaultProfilePath()
	}
	dbPath := expandPath(args.DBPath)
	if dbPath == "" {
		dbPath = DefaultDBPath()
	}

	changed, msg := ConfigureShellEnvironment(bashrcPath, profilePath, dbPath, args.DryRun)
	statusLabel := Colorize(out, "UP-TO-DATE", ColorBlue)
	if changed {
		statusLabel = Colorize(out, "MODIFIED", ColorGreen)
	}
	fmt.Fprintf(out, "[%s] %s\n", statusLabel, msg)
	return 0
}

// ToolchainsOptions holds arguments for CmdToolchains.
type ToolchainsOptions struct {
	OS         string
	Profile    string
	Install    bool
	DryRun     bool
	ConfigPath string
}

// CmdToolchains is the standalone runner for Stage 1: Developer Toolchains.
func CmdToolchains(out io.Writer, args ToolchainsOptions) int {
	targetOS := DetectOS(args.OS)
	profile := strings.ToLower(args.Profile)
	if profile == "" {
		profile = "dev"
	}

	catalog, err := LoadToolchainsCatalog(args.ConfigPath)
	if err != nil {
		fmt.Fprintf(out, "Error loading toolchains catalog: %v\n", err)
		return 1
	}

	audits := AuditToolchains(catalog, profile, targetOS)
	var missing []ToolAuditResult
	for _, t := range audits {
		if !t.Installed {
			missing = append(missing, t)
		}
	}

	fmt.Fprintf(out, "Auditing toolchains (OS: %s, Profile: %s):\n", targetOS, profile)
	for _, t := range audits {
		if t.Installed {
			fmt.Fprintf(out, "  %s %-22s [installed]\n", Colorize(out, "✓", ColorGreen), t.Name)
		} else {
			fmt.Fprintf(out, "  %s %-22s [missing]\n", Colorize(out, "✗", ColorRed), t.Name)
			fmt.Fprintf(out, "      Command: %s\n", Colorize(out, t.InstallCmd, ColorCyan))
		}
	}

	if len(missing) > 0 && args.Install {
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Installing missing toolchains:")
		for _, t := range missing {
			RunCmdInteractive(out, t.InstallCmd, args.DryRun)
		}
	} else if len(missing) > 0 {
		fmt.Fprintln(out)
		fmt.Fprintf(out, "%d tool(s) missing. Run with --install to install.\n", len(missing))
	}

	if len(missing) == 0 || args.Install {
		return 0
	}
	return 1
}
