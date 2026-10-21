# workstation-setup - Agent Directives & Architecture

Welcome to the **workstation-setup** repository. This repository acts as **Layer 1 (Generic Developer Toolchains, Shell Environment Bridge, and Cross-Platform Meta-Orchestrator)** across Cooper Wallace's development workstations (Linux, macOS, and Windows).

---

## 1. Repository Layout

```text
.
├── AGENTS.md                 # Primary agent instructions and repository map
├── README.md                 # Human-oriented quickstart and usage documentation
├── LICENSE                   # MIT License
├── install.sh                # Symlinks workstation-setup to ~/.local/bin
├── .gitignore                # Git ignore patterns
│
├── bin/
│   └── workstation-setup     # Python 3 CLI tool (audit, apply/setup, env, toolchains)
│
├── config/
│   └── toolchains.json       # Declarative JSON catalog of cross-platform developer toolchains
│
└── tests/
    └── test_workstation.py   # Automated unit and integration test suite
```

---

## 2. Multi-Layer Workstation Architecture

This tool operates as the central coordinator in a 4-layer workstation stack:

- **Layer 0 (`riced-kubuntu-os`)**: Kubuntu OS, KDE Plasma 6 desktop configuration, and hardware pipelines.
- **Layer 1 (`workstation-setup`)**: Cross-platform developer toolchains (`git`, `gh`, `uv`, `node`, `starship`, `zoxide`, `fzf`, `eza`) and Shell Environment Bridge across Linux, macOS, and Windows.
- **Layer 2 (`antigravity-standardise`)**: AI pair programmer harness permissions, global agent directives (`AGENTS.md`), MCP servers, and autonomous workflows.
- **Layer 3 (`gh-standardise`)**: Remote GitHub repository rulesets, squash-and-merge enforcement, and semantic PR CI quality gates.

---

## 3. Engineering Guidelines for AI Agents

1. **Clean Architecture & Minimal Dependencies**:
   - Use Python 3.10+ standard library exclusively (`argparse`, `json`, `os`, `platform`, `re`, `shutil`, `subprocess`, `sys`, `pathlib`, `typing`).
   - Do NOT introduce external dependencies (e.g. `requests`, `pyyaml`) into `bin/workstation-setup`.

2. **Idempotence & Safety**:
   - All state mutations (shell block injection, database initialization, toolchain installations) must be safe to run multiple times without duplicating entries.
   - All mutating commands must support `--dry-run`.
   - The shell block markers `# >>> Antigravity Standards Environment >>>` and `# <<< Antigravity Standards Environment <<<` must be strictly preserved.

3. **Dynamic Path Resolution**:
   - Never hardcode `/home/cooper` or absolute user paths in code or configuration.
   - Always resolve dynamically using `Path.home()`, `$HOME`, or relative project roots (`REPO_ROOT`).

4. **Testing & Verification**:
   - Any modifications to the toolchain catalog, OS detection, profile filters, or shell injection must be validated with:
     ```bash
     python3 -m unittest discover tests -v
     ```
   - Keep tests comprehensive, fast, and hermetic (using `tempfile.TemporaryDirectory`).

5. **Conventional Commits 1.0.0**:
   - Commit messages must strictly follow: `<type>(<scope>): <short summary>`
   - Allowed types: `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`, `build`, `ci`, `chore`, `revert`.
