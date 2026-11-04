#!/usr/bin/env python3
"""
Unit and integration tests for workstation-setup CLI and library modules.
"""

import argparse
import importlib.machinery
import importlib.util
import io
import json
import os
import shutil
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path
from unittest.mock import MagicMock, call, mock_open, patch

REPO_ROOT = Path(__file__).resolve().parent.parent
BIN_PATH = REPO_ROOT / "bin" / "workstation-setup"
CONFIG_PATH = REPO_ROOT / "config" / "toolchains.json"

# Dynamically import workstation-setup binary module
loader = importlib.machinery.SourceFileLoader("workstation_setup", str(BIN_PATH))
spec = importlib.util.spec_from_loader("workstation_setup", loader)
ws = importlib.util.module_from_spec(spec)
loader.exec_module(ws)


class TestWorkstationSetup(unittest.TestCase):
    """Test suite for workstation-setup functionality."""

    def setUp(self):
        self.temp_dir = tempfile.TemporaryDirectory()
        self.test_root = Path(self.temp_dir.name)

    def tearDown(self):
        self.temp_dir.cleanup()

    def test_os_detection_and_overrides(self):
        """Test OS detection with overrides and auto detection."""
        self.assertEqual(ws.detect_os("linux"), "linux")
        self.assertEqual(ws.detect_os("ubuntu"), "linux")
        self.assertEqual(ws.detect_os("macos"), "macos")
        self.assertEqual(ws.detect_os("darwin"), "macos")
        self.assertEqual(ws.detect_os("windows"), "windows")
        self.assertEqual(ws.detect_os("win32"), "windows")

        # Auto detection should return one of linux, macos, windows
        detected = ws.detect_os("auto")
        self.assertIn(detected, ("linux", "macos", "windows"))

        detected_none = ws.detect_os(None)
        self.assertIn(detected_none, ("linux", "macos", "windows"))

    def test_toolchains_catalog_validity(self):
        """Verify config/toolchains.json exists and conforms to schema expectations."""
        self.assertTrue(CONFIG_PATH.exists(), f"Missing toolchains catalog at: {CONFIG_PATH}")
        catalog = ws.load_toolchains_catalog(CONFIG_PATH)

        self.assertIn("version", catalog)
        self.assertIn("tools", catalog)
        tools = catalog["tools"]

        expected_tools = ["git", "gh", "zram", "uv", "node", "starship", "zoxide", "fzf", "eza"]
        for tool_name in expected_tools:
            self.assertIn(tool_name, tools, f"Catalog missing required tool: {tool_name}")
            tool_data = tools[tool_name]
            self.assertIn("binary", tool_data)
            self.assertIn("profile", tool_data)
            self.assertIn(tool_data["profile"], ("minimal", "dev", "full"))
            self.assertIn("install", tool_data)
            install = tool_data["install"]
            self.assertIn("linux", install)
            self.assertIn("macos", install)
            self.assertIn("windows", install)

    def test_profile_filtering(self):
        """Verify toolchain filtering per profile tier."""
        catalog = ws.load_toolchains_catalog(CONFIG_PATH)

        minimal_tools = ws.filter_tools_by_profile(catalog, "minimal")
        self.assertIn("git", minimal_tools)
        self.assertIn("gh", minimal_tools)
        self.assertIn("zram", minimal_tools)
        self.assertNotIn("uv", minimal_tools)
        self.assertNotIn("zoxide", minimal_tools)

        dev_tools = ws.filter_tools_by_profile(catalog, "dev")
        self.assertIn("git", dev_tools)
        self.assertIn("gh", dev_tools)
        self.assertIn("zram", dev_tools)
        self.assertIn("uv", dev_tools)
        self.assertIn("node", dev_tools)
        self.assertIn("starship", dev_tools)
        self.assertNotIn("zoxide", dev_tools)

        full_tools = ws.filter_tools_by_profile(catalog, "full")
        self.assertIn("git", full_tools)
        self.assertIn("zram", full_tools)
        self.assertIn("uv", full_tools)
        self.assertIn("zoxide", full_tools)
        self.assertIn("fzf", full_tools)
        self.assertIn("eza", full_tools)

    def test_configure_shell_environment_injection_and_idempotence(self):
        """Test injecting shell environment block into bashrc and profile, verifying idempotence."""
        test_bashrc = self.test_root / ".bashrc"
        test_bashrc.write_text("# Existing bashrc content\nexport TEST_VAR=1\n", encoding="utf-8")

        test_profile = self.test_root / ".profile"
        test_profile.write_text("# Existing profile content\n", encoding="utf-8")

        test_db = self.test_root / "dev.db"

        # First application: should inject block and touch dev.db
        changed, msg = ws.configure_shell_environment(
            bashrc_path=test_bashrc,
            profile_path=test_profile,
            db_path=test_db,
            dry_run=False,
        )
        self.assertTrue(changed)
        self.assertTrue(test_db.exists(), "dev.db should be created/touched")

        bashrc_content = test_bashrc.read_text(encoding="utf-8")
        self.assertIn(ws.ENV_BLOCK_START, bashrc_content)
        self.assertIn(ws.ENV_BLOCK_END, bashrc_content)
        self.assertIn("GITHUB_PERSONAL_ACCESS_TOKEN", bashrc_content)
        self.assertIn("SQLITE_DEV_DB", bashrc_content)
        self.assertIn("AGENTKIT_AGENTVAULT_PATH", bashrc_content)

        profile_content = test_profile.read_text(encoding="utf-8")
        self.assertIn(ws.ENV_BLOCK_START, profile_content)
        self.assertIn(ws.ENV_BLOCK_END, profile_content)

        # Count occurrences in bashrc (must be exactly 1)
        self.assertEqual(bashrc_content.count(ws.ENV_BLOCK_START), 1)
        self.assertEqual(bashrc_content.count(ws.ENV_BLOCK_END), 1)

        # Second application: should be completely idempotent (changed == False)
        changed_second, msg_second = ws.configure_shell_environment(
            bashrc_path=test_bashrc,
            profile_path=test_profile,
            db_path=test_db,
            dry_run=False,
        )
        self.assertFalse(changed_second, "Second run should report no changes (idempotent)")
        bashrc_content_after = test_bashrc.read_text(encoding="utf-8")
        self.assertEqual(bashrc_content, bashrc_content_after)
        self.assertEqual(bashrc_content_after.count(ws.ENV_BLOCK_START), 1)

    def test_configure_shell_environment_dry_run(self):
        """Verify dry-run does not write to files or create databases."""
        test_bashrc = self.test_root / ".bashrc"
        test_bashrc.write_text("# Clean bashrc\n", encoding="utf-8")

        test_db = self.test_root / "dev.db"

        changed, msg = ws.configure_shell_environment(
            bashrc_path=test_bashrc,
            profile_path=None,
            db_path=test_db,
            dry_run=True,
        )
        self.assertTrue(changed, "Dry-run should indicate changes would occur")
        self.assertFalse(test_db.exists(), "Dry-run must NOT create dev.db")

        bashrc_content = test_bashrc.read_text(encoding="utf-8")
        self.assertNotIn(ws.ENV_BLOCK_START, bashrc_content)
        self.assertEqual(bashrc_content, "# Clean bashrc\n")

    def test_cli_audit_and_apply_dry_run(self):
        """Test invoking bin/workstation-setup via CLI subprocess."""
        test_bashrc = self.test_root / ".bashrc"
        test_bashrc.write_text("# Fresh bashrc\n", encoding="utf-8")
        test_profile = self.test_root / ".profile"
        test_profile.write_text("# Fresh profile\n", encoding="utf-8")

        # Test apply --dry-run
        cmd_apply = [
            sys.executable,
            str(BIN_PATH),
            "apply",
            "--profile", "minimal",
            "--bashrc-path", str(test_bashrc),
            "--profile-path", str(test_profile),
            "--dry-run",
        ]
        res = subprocess.run(cmd_apply, capture_output=True, text=True)
        self.assertEqual(res.returncode, 0, f"Apply dry-run failed with stderr:\n{res.stderr}")
        self.assertIn("DRY-RUN", res.stdout)
        self.assertIn("Stage 1", res.stdout)
        self.assertIn("Stage 2", res.stdout)

        # Test env subcommand --dry-run
        cmd_env = [
            sys.executable,
            str(BIN_PATH),
            "env",
            "--bashrc-path", str(test_bashrc),
            "--profile-path", str(test_profile),
            "--dry-run",
        ]
        res_env = subprocess.run(cmd_env, capture_output=True, text=True)
        self.assertEqual(res_env.returncode, 0)
        self.assertIn("MODIFIED", res_env.stdout)

        # Now actually apply env to test_bashrc so audit has a valid env block
        ws.configure_shell_environment(test_bashrc, test_profile, db_path=self.test_root / "dev.db", dry_run=False)

        # Test audit subcommand
        cmd_audit = [
            sys.executable,
            str(BIN_PATH),
            "audit",
            "--profile", "minimal",
            "--bashrc-path", str(test_bashrc),
        ]
        res_audit = subprocess.run(cmd_audit, capture_output=True, text=True)
        self.assertIn("Layer 1", res_audit.stdout)
        self.assertIn("Layer 2", res_audit.stdout)
        self.assertIn("Swap/Memory buffer", res_audit.stdout)

        # Test toolchains subcommand --dry-run
        cmd_toolchains = [
            sys.executable,
            str(BIN_PATH),
            "toolchains",
            "--profile", "minimal",
            "--dry-run",
        ]
        res_tools = subprocess.run(cmd_toolchains, capture_output=True, text=True)
        self.assertIn("Auditing toolchains", res_tools.stdout)

    def test_check_swap_status_non_linux(self):
        """Verify check_swap_status reports native compressed memory on non-Linux platforms."""
        with patch("platform.system", return_value="Darwin"):
            active, msg = ws.check_swap_status()
            self.assertTrue(active)
            self.assertEqual(msg, "Native compressed memory paging active")

        with patch("platform.system", return_value="Windows"):
            active, msg = ws.check_swap_status()
            self.assertTrue(active)
            self.assertEqual(msg, "Native compressed memory paging active")

        # Explicit target_os overrides
        self.assertTrue(ws.check_swap_status(target_os="macos")[0])
        self.assertTrue(ws.check_swap_status(target_os="windows")[0])

    def test_check_swap_status_linux_active_proc_swaps(self):
        """Verify check_swap_status detects active swap from /proc/swaps."""
        mock_proc_content = (
            "Filename\tType\tSize\tUsed\tPriority\n"
            "/dev/zram0\tpartition\t8388604\t0\t100\n"
        )
        with patch("platform.system", return_value="Linux"):
            with patch("builtins.open", mock_open(read_data=mock_proc_content)):
                active, msg = ws.check_swap_status()
                self.assertTrue(active)
                self.assertEqual(msg, "Active swap/zram detected")

    def test_check_swap_status_linux_no_swap(self):
        """Verify check_swap_status flags missing swap when /proc/swaps has no entries."""
        mock_proc_content = "Filename\tType\tSize\tUsed\tPriority\n"
        mock_res = MagicMock()
        mock_res.returncode = 0
        mock_res.stdout = ""
        with patch("platform.system", return_value="Linux"):
            with patch("builtins.open", mock_open(read_data=mock_proc_content)):
                with patch("subprocess.run", return_value=mock_res):
                    active, msg = ws.check_swap_status()
                    self.assertFalse(active)
                    self.assertIn("No active swap or zram detected", msg)

    def test_check_swap_status_linux_swapon_fallback(self):
        """Verify check_swap_status falls back to swapon --show when /proc/swaps is unavailable."""
        mock_res = MagicMock()
        mock_res.returncode = 0
        mock_res.stdout = "NAME      TYPE      SIZE USED PRIO\n/dev/zram0 partition 7.7G   0B  100\n"
        with patch("platform.system", return_value="Linux"):
            with patch("builtins.open", side_effect=FileNotFoundError):
                with patch("subprocess.run", return_value=mock_res):
                    active, msg = ws.check_swap_status()
                    self.assertTrue(active)
                    self.assertEqual(msg, "Active swap/zram detected")

    def test_stage3_apply_with_agentkit_in_path(self):
        """Verify Stage 3 applies agentkit from PATH with and without dry-run."""
        test_bashrc = self.test_root / ".bashrc"
        test_bashrc.write_text("# bashrc\n", encoding="utf-8")
        test_profile = self.test_root / ".profile"
        test_profile.write_text("# profile\n", encoding="utf-8")

        args = argparse.Namespace(
            os="auto",
            profile="minimal",
            stage="3",
            dry_run=False,
            install_missing=False,
            bashrc_path=str(test_bashrc),
            profile_path=str(test_profile),
        )

        with patch("shutil.which", return_value="/usr/local/bin/agentkit"):
            with patch.object(ws, "run_cmd_interactive", return_value=0) as mock_run:
                with patch("sys.stdout", new_callable=io.StringIO) as mock_stdout:
                    ret = ws.cmd_apply(args)
                    self.assertEqual(ret, 0)
                    mock_run.assert_has_calls([
                        call("/usr/local/bin/agentkit apply", dry_run=False),
                        call("/usr/local/bin/agentkit vault sync", dry_run=False),
                    ])
                    out = mock_stdout.getvalue()
                    self.assertIn("Running AI agent governance and wake-lock configuration via: /usr/local/bin/agentkit apply", out)
                    self.assertIn("Synchronizing Obsidian AgentVault via: /usr/local/bin/agentkit vault sync", out)

        # Test with dry_run=True
        args.dry_run = True
        with patch("shutil.which", return_value="/usr/local/bin/agentkit"):
            with patch.object(ws, "run_cmd_interactive", return_value=0) as mock_run:
                with patch("sys.stdout", new_callable=io.StringIO) as mock_stdout:
                    ret = ws.cmd_apply(args)
                    self.assertEqual(ret, 0)
                    mock_run.assert_has_calls([
                        call("/usr/local/bin/agentkit apply --dry-run", dry_run=True),
                        call("/usr/local/bin/agentkit vault sync --dry-run", dry_run=True),
                    ])
                    out = mock_stdout.getvalue()
                    self.assertIn("Running AI agent governance and wake-lock configuration via: /usr/local/bin/agentkit apply --dry-run", out)
                    self.assertIn("Synchronizing Obsidian AgentVault via: /usr/local/bin/agentkit vault sync --dry-run", out)

    def test_stage3_apply_with_local_agentkit(self):
        """Verify Stage 3 applies agentkit found in ~/.local/bin when not in PATH."""
        test_bashrc = self.test_root / ".bashrc"
        test_bashrc.write_text("# bashrc\n", encoding="utf-8")
        test_profile = self.test_root / ".profile"
        test_profile.write_text("# profile\n", encoding="utf-8")

        local_bin = self.test_root / ".local" / "bin"
        local_bin.mkdir(parents=True, exist_ok=True)
        local_agentkit = local_bin / "agentkit"
        local_agentkit.touch()

        args = argparse.Namespace(
            os="auto",
            profile="minimal",
            stage="3",
            dry_run=False,
            install_missing=False,
            bashrc_path=str(test_bashrc),
            profile_path=str(test_profile),
        )

        with patch("shutil.which", return_value=None):
            with patch.object(Path, "home", return_value=self.test_root):
                with patch.object(ws, "run_cmd_interactive", return_value=0) as mock_run:
                    with patch("sys.stdout", new_callable=io.StringIO) as mock_stdout:
                        ret = ws.cmd_apply(args)
                        self.assertEqual(ret, 0)
                        mock_run.assert_has_calls([
                            call(f"{local_agentkit} apply", dry_run=False),
                            call(f"{local_agentkit} vault sync", dry_run=False),
                        ])
                        out = mock_stdout.getvalue()
                        self.assertIn(f"Running AI agent governance and wake-lock configuration via: {local_agentkit} apply", out)
                        self.assertIn(f"Synchronizing Obsidian AgentVault via: {local_agentkit} vault sync", out)

    def test_stage3_apply_missing_agentkit(self):
        """Verify Stage 3 outputs advisory instructions when agentkit is not installed."""
        test_bashrc = self.test_root / ".bashrc"
        test_bashrc.write_text("# bashrc\n", encoding="utf-8")
        test_profile = self.test_root / ".profile"
        test_profile.write_text("# profile\n", encoding="utf-8")

        args = argparse.Namespace(
            os="auto",
            profile="minimal",
            stage="3",
            dry_run=False,
            install_missing=False,
            bashrc_path=str(test_bashrc),
            profile_path=str(test_profile),
        )

        with patch("shutil.which", return_value=None):
            with patch.object(Path, "home", return_value=self.test_root):
                with patch.object(ws, "run_cmd_interactive") as mock_run:
                    with patch("sys.stdout", new_callable=io.StringIO) as mock_stdout:
                        ret = ws.cmd_apply(args)
                        self.assertEqual(ret, 0)
                        mock_run.assert_not_called()
                        out = mock_stdout.getvalue()
                        self.assertIn("agentkit not found in PATH or ~/.local/bin.", out)
                        self.assertIn("To synchronize AI agent permissions: cd ~/Projects/agentkit && ./install.sh && agentkit apply", out)

    def test_audit_agentkit_status(self):
        """Verify cmd_audit reports agentkit presence from PATH, ~/.local/bin, or missing."""
        test_bashrc = self.test_root / ".bashrc"
        test_bashrc.write_text("# bashrc\n", encoding="utf-8")
        ws.configure_shell_environment(test_bashrc, None, db_path=self.test_root / "dev.db", dry_run=False)

        args = argparse.Namespace(
            os="auto",
            profile="minimal",
            bashrc_path=str(test_bashrc),
            verbose=False,
        )

        # 1. agentkit in PATH
        original_which = shutil.which
        def which_path(cmd):
            if cmd == "agentkit":
                return "/usr/bin/agentkit"
            return original_which(cmd)

        with patch("shutil.which", side_effect=which_path):
            with patch("sys.stdout", new_callable=io.StringIO) as mock_stdout:
                ws.cmd_audit(args)
                out = mock_stdout.getvalue()
                self.assertIn("agentkit available: /usr/bin/agentkit", out)

        # 2. agentkit in ~/.local/bin
        local_bin = self.test_root / ".local" / "bin"
        local_bin.mkdir(parents=True, exist_ok=True)
        local_agentkit = local_bin / "agentkit"
        local_agentkit.touch()

        def which_none(cmd):
            if cmd == "agentkit":
                return None
            return original_which(cmd)

        with patch("shutil.which", side_effect=which_none):
            with patch.object(Path, "home", return_value=self.test_root):
                with patch("sys.stdout", new_callable=io.StringIO) as mock_stdout:
                    ws.cmd_audit(args)
                    out = mock_stdout.getvalue()
                    self.assertIn(f"agentkit found in ~/.local/bin: {local_agentkit}", out)

        # 3. agentkit missing entirely
        local_agentkit.unlink()

        with patch("shutil.which", side_effect=which_none):
            with patch.object(Path, "home", return_value=self.test_root):
                with patch("sys.stdout", new_callable=io.StringIO) as mock_stdout:
                    ws.cmd_audit(args)
                    out = mock_stdout.getvalue()
                    self.assertIn("agentkit not installed (run agentkit install.sh)", out)

    def test_standard_env_block_contains_agentvault_path(self):
        """Verify get_standard_env_block exports AGENTKIT_AGENTVAULT_PATH."""
        block = ws.get_standard_env_block()
        self.assertIn('export AGENTKIT_AGENTVAULT_PATH="${AGENTKIT_AGENTVAULT_PATH:-$HOME/Documents/AgentVault}"', block)

    def test_resolve_agentvault_path_hierarchy(self):
        """Verify resolve_agentvault_path follows explicit -> AGENTKIT_AGENTVAULT_PATH -> AGENTKIT_VAULT_PATH -> default."""
        # 1. Explicit path
        custom_explicit = self.test_root / "explicit-vault"
        self.assertEqual(ws.resolve_agentvault_path(str(custom_explicit)), custom_explicit.resolve())

        # 2. AGENTKIT_AGENTVAULT_PATH environment variable
        env_primary = str(self.test_root / "primary-vault")
        with patch.dict(os.environ, {"AGENTKIT_AGENTVAULT_PATH": env_primary, "AGENTKIT_VAULT_PATH": "/ignored"}):
            self.assertEqual(ws.resolve_agentvault_path(), Path(env_primary).resolve())

        # 3. AGENTKIT_VAULT_PATH fallback environment variable
        env_secondary = str(self.test_root / "secondary-vault")
        with patch.dict(os.environ, {"AGENTKIT_AGENTVAULT_PATH": "", "AGENTKIT_VAULT_PATH": env_secondary}):
            self.assertEqual(ws.resolve_agentvault_path(), Path(env_secondary).resolve())

        # 4. DEFAULT_AGENTVAULT_PATH fallback
        with patch.dict(os.environ, {"AGENTKIT_AGENTVAULT_PATH": "", "AGENTKIT_VAULT_PATH": ""}):
            self.assertEqual(ws.resolve_agentvault_path(), ws.DEFAULT_AGENTVAULT_PATH)

    def test_ensure_agentvault_setup_dry_run(self):
        """Verify ensure_agentvault_setup in dry-run mode is completely non-mutating."""
        # A: Non-existent vault
        vault_a = self.test_root / "DryRunNonExistent"
        changed_a, msg_a = ws.ensure_agentvault_setup(vault_path=vault_a, dry_run=True)
        self.assertTrue(changed_a)
        self.assertIn("Would clone", msg_a)
        self.assertFalse(vault_a.exists())

        # B: Existing vault without .git
        vault_b = self.test_root / "DryRunUntracked"
        vault_b.mkdir(parents=True, exist_ok=True)
        (vault_b / "test.md").write_text("content", encoding="utf-8")
        changed_b, msg_b = ws.ensure_agentvault_setup(vault_path=vault_b, dry_run=True)
        self.assertTrue(changed_b)
        self.assertIn("Would initialize git repository in AgentVault", msg_b)
        self.assertFalse((vault_b / ".git").exists())

        # C: Existing vault with .git
        vault_c = self.test_root / "DryRunTracked"
        vault_c.mkdir(parents=True, exist_ok=True)
        (vault_c / ".git").mkdir(parents=True, exist_ok=True)
        changed_c, msg_c = ws.ensure_agentvault_setup(vault_path=vault_c, dry_run=True)
        self.assertFalse(changed_c)
        self.assertIn("Would sync git-backed AgentVault", msg_c)

    def test_ensure_agentvault_setup_guarded_fallback_offline(self):
        """Verify guarded degrade path when clone fails, ensuring .git and .gitignore are created."""
        vault_dir = self.test_root / "OfflineVault"

        # Simulate git clone and gh repo clone failure
        original_run = subprocess.run
        def mock_subprocess_run(cmd, *args, **kwargs):
            cmd_str = " ".join(cmd) if isinstance(cmd, list) else str(cmd)
            if "clone" in cmd_str:
                mock_res = MagicMock()
                mock_res.returncode = 1
                mock_res.stdout = "Fatal: network unreachable"
                mock_res.stderr = "Could not resolve host: github.com"
                return mock_res
            return original_run(cmd, *args, **kwargs)

        with patch("subprocess.run", side_effect=mock_subprocess_run):
            changed, msg = ws.ensure_agentvault_setup(vault_path=vault_dir, dry_run=False)

        self.assertTrue(changed)
        self.assertIn("Initialized local git-backed AgentVault", msg)
        self.assertIn("(offline/fallback mode)", msg)

        # Verify .git exists
        self.assertTrue((vault_dir / ".git").exists())
        self.assertTrue((vault_dir / ".git").is_dir())

        # Verify .gitignore exists and contains Obsidian ignore patterns
        gitignore_path = vault_dir / ".gitignore"
        self.assertTrue(gitignore_path.exists())
        gitignore_content = gitignore_path.read_text(encoding="utf-8")
        for entry in ws.OBSIDIAN_GITIGNORE_ENTRIES:
            self.assertIn(entry, gitignore_content)

    def test_ensure_agentvault_setup_untracked_existing(self):
        """Verify initializing git tracking for an existing untracked AgentVault."""
        vault_dir = self.test_root / "ExistingUntrackedVault"
        vault_dir.mkdir(parents=True, exist_ok=True)
        (vault_dir / "Dashboard.md").write_text("# My Vault\n", encoding="utf-8")
        sub_folder = vault_dir / "Projects"
        sub_folder.mkdir()
        (sub_folder / "agentkit.md").write_text("# Agentkit\n", encoding="utf-8")

        changed, msg = ws.ensure_agentvault_setup(vault_path=vault_dir, dry_run=False)
        self.assertTrue(changed)
        self.assertIn("Initialized git tracking for existing AgentVault at", msg)

        # Verify .git and .gitignore
        self.assertTrue((vault_dir / ".git").exists())
        self.assertTrue((vault_dir / ".gitignore").exists())

        # Verify initial commit
        res_log = subprocess.run(
            ["git", "-C", str(vault_dir), "log", "-1", "--oneline"],
            capture_output=True,
            text=True,
        )
        self.assertEqual(res_log.returncode, 0)
        self.assertIn("chore(vault): initialize git-backed Obsidian AgentVault", res_log.stdout)

        # Re-running on the now git-tracked vault should report verified/up-to-date
        changed_second, msg_second = ws.ensure_agentvault_setup(vault_path=vault_dir, dry_run=False)
        self.assertFalse(changed_second)
        self.assertIn("Verified git-backed AgentVault at", msg_second)

    def test_audit_agentvault_reporting(self):
        """Verify cmd_audit reports git-tracked, untracked, and missing AgentVault states."""
        test_bashrc = self.test_root / ".bashrc"
        test_bashrc.write_text("# bashrc\n", encoding="utf-8")
        ws.configure_shell_environment(test_bashrc, None, db_path=self.test_root / "dev.db", dry_run=False)

        args = argparse.Namespace(
            os="auto",
            profile="minimal",
            bashrc_path=str(test_bashrc),
            verbose=False,
        )

        vault_dir = self.test_root / "AuditVault"

        # 1. Missing vault
        with patch.dict(os.environ, {"AGENTKIT_AGENTVAULT_PATH": str(vault_dir)}):
            with patch("sys.stdout", new_callable=io.StringIO) as mock_stdout:
                ws.cmd_audit(args)
                out = mock_stdout.getvalue()
                self.assertIn(f"Obsidian AgentVault missing: {vault_dir} (will be provisioned by apply)", out)

        # 2. Untracked vault
        vault_dir.mkdir(parents=True, exist_ok=True)
        (vault_dir / "note.md").write_text("hello", encoding="utf-8")
        with patch.dict(os.environ, {"AGENTKIT_AGENTVAULT_PATH": str(vault_dir)}):
            with patch("sys.stdout", new_callable=io.StringIO) as mock_stdout:
                ws.cmd_audit(args)
                out = mock_stdout.getvalue()
                self.assertIn(f"Obsidian AgentVault present (untracked; run apply to init git): {vault_dir}", out)

        # 3. Git-tracked vault
        subprocess.run(["git", "-C", str(vault_dir), "init", "-b", "main"], capture_output=True)
        with patch.dict(os.environ, {"AGENTKIT_AGENTVAULT_PATH": str(vault_dir)}):
            with patch("sys.stdout", new_callable=io.StringIO) as mock_stdout:
                ws.cmd_audit(args)
                out = mock_stdout.getvalue()
                self.assertIn(f"Obsidian AgentVault present (git-tracked): {vault_dir}", out)


if __name__ == "__main__":
    unittest.main()
