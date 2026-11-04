# workstation-setup - Agent Directives & Architecture

Welcome to the **workstation-setup** repository. This repository acts as **Layer 1 (Generic Developer Toolchains, Shell Environment Bridge, and Cross-Platform Meta-Orchestrator)** across Cooper Wallace's development workstations (Linux, macOS, and Windows).

---

## 1. Repository Layout

```text
.
├── AGENTS.md                 # Primary agent instructions and repository map
├── README.md                 # Human-oriented quickstart and usage documentation
├── LICENSE                   # MIT License
├── install.sh                # Static Go binary compiler & installer into ~/.local/bin
├── .gitignore                # Git ignore patterns
├── .goreleaser.yaml          # GoReleaser v2 cross-compilation & release packaging
├── go.mod                    # Go module definition (Go 1.23+)
│
├── cmd/
│   └── workstation-setup/
│       └── main.go           # CLI entrypoint and subcommand routing
│
├── config/
│   └── toolchains.json       # Declarative JSON catalog of 15 cross-platform developer toolchains
│
├── embed.go                  # Embedded assets via //go:embed config/toolchains.json
├── workstation.go            # Core domain types, runner logic, stages, and toolchain evaluation
└── workstation_test.go       # Hermetic unit and integration test suite
```

---

## 2. Multi-Layer Workstation Architecture

This tool operates as the central coordinator in a 4-layer workstation stack:

- **Layer 0 (`riced-kubuntu-os`)**: Kubuntu OS, KDE Plasma 6 desktop configuration, and hardware pipelines.
- **Layer 1 (`workstation-setup`)**: Cross-platform developer toolchains (`git`, `gh`, `zram`, `uv`, `node`, `starship`, `go`, `docker`, `zoxide`, `fzf`, `eza`, `kubectl`, `k3s`, `helm`, `k9s`) and Shell Environment Bridge across Linux, macOS, and Windows.
- **Layer 2 (`agentkit`)**: AI pair programmer harness permissions, Git-backed Obsidian `AgentVault` (`$HOME/Documents/AgentVault`) provisioning and synchronization via `agentkit vault sync`, global agent directives (`AGENTS.md`), MCP servers, and autonomous workflows.
- **Layer 3 (`repokit`)**: Remote GitHub repository rulesets, squash-and-merge enforcement, and semantic PR CI quality gates.

---

## 3. Engineering Guidelines for AI Agents

1. **Clean Architecture & Minimal Dependencies**:
   - Use Go 1.23+ standard library exclusively (`os`, `os/exec`, `path/filepath`, `encoding/json`, `flag`, `fmt`, `strings`, `runtime`, `embed`).
   - Do NOT introduce external third-party dependencies into `go.mod`.
   - Embed declarative configurations directly into the compiled binary using `//go:embed config/toolchains.json`.

2. **Idempotence & Safety**:
   - All state mutations (shell block injection, database initialization, toolchain installations) must be safe to run multiple times without duplicating entries.
   - All mutating commands must support `--dry-run`.
   - The shell block markers `# >>> Antigravity Standards Environment >>>` and `# <<< Antigravity Standards Environment <<<` must be strictly preserved.

3. **Dynamic Path Resolution**:
   - Never hardcode `/home/cooper` or absolute user paths in code or configuration.
   - Always resolve dynamically using `os.UserHomeDir()`, `$HOME`, or relative project roots (`REPO_ROOT`).

4. **Testing & Verification**:
   - Any modifications to the Go source, toolchain catalog, OS detection, profile filters, or shell injection must be validated with:
     ```bash
     go test -v -race ./...
     ```
   - Keep tests comprehensive, fast, and hermetic (using `t.TempDir()`).

5. **Conventional Commits 1.0.0**:
   - Commit messages must strictly follow: `<type>(<scope>): <short summary>`
   - Allowed types: `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`, `build`, `ci`, `chore`, `revert`.

6. **AgentVault Memory & Shell Environment Bridge**:
   - Stage 2 exports `AGENTKIT_AGENTVAULT_PATH` (`$HOME/Documents/AgentVault` by default) so `agentkit` and `repokit` share a consistent vault path across workstations.
   - Stage 3 provisions and synchronizes the Git-backed Obsidian `AgentVault`, tracking markdown knowledge bases, project notes, and session handoffs with standard `.obsidian` ignores and graceful offline degradation.
