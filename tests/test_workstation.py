#!/usr/bin/env python3
"""
Unit and integration tests for workstation-setup CLI and library modules.
"""

import importlib.machinery
import importlib.util
import json
import os
import shutil
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

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

        expected_tools = ["git", "gh", "uv", "node", "starship", "zoxide", "fzf", "eza"]
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
        self.assertNotIn("uv", minimal_tools)
        self.assertNotIn("zoxide", minimal_tools)

        dev_tools = ws.filter_tools_by_profile(catalog, "dev")
        self.assertIn("git", dev_tools)
        self.assertIn("gh", dev_tools)
        self.assertIn("uv", dev_tools)
        self.assertIn("node", dev_tools)
        self.assertIn("starship", dev_tools)
        self.assertNotIn("zoxide", dev_tools)

        full_tools = ws.filter_tools_by_profile(catalog, "full")
        self.assertIn("git", full_tools)
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
        ws.configure_shell_environment(test_bashrc, test_profile, dry_run=False)

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


if __name__ == "__main__":
    unittest.main()
