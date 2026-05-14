import tempfile
import unittest
from pathlib import Path

import brew_release_smoke


class BrewReleaseSmokeTest(unittest.TestCase):
    def test_builds_install_and_test_commands_for_both_formulae(self):
        with tempfile.TemporaryDirectory() as tmp:
            tap_dir = Path(tmp)
            formula_dir = tap_dir / "Formula"
            formula_dir.mkdir()
            (formula_dir / "caboose-homelab.rb").write_text("class CabooseHomelab < Formula\nend\n")
            (formula_dir / "caboose-homelab-mcp.rb").write_text(
                "class CabooseHomelabMcp < Formula\nend\n",
            )

            commands = brew_release_smoke.brew_smoke_commands(
                tap_dir=tap_dir,
                tap_name="caboose-ai/test",
            )

            self.assertEqual(
                [
                    [
                        "brew",
                        "tap",
                        "--custom-remote",
                        "caboose-ai/test",
                        tap_dir.resolve().as_uri(),
                    ],
                    [
                        "brew",
                        "install",
                        "--build-from-source",
                        "caboose-ai/test/caboose-homelab",
                    ],
                    ["brew", "test", "caboose-ai/test/caboose-homelab"],
                    [
                        "brew",
                        "install",
                        "--build-from-source",
                        "caboose-ai/test/caboose-homelab-mcp",
                    ],
                    ["brew", "test", "caboose-ai/test/caboose-homelab-mcp"],
                ],
                commands,
            )

    def test_missing_formula_fails(self):
        with tempfile.TemporaryDirectory() as tmp:
            tap_dir = Path(tmp)
            formula_dir = tap_dir / "Formula"
            formula_dir.mkdir()
            (formula_dir / "caboose-homelab.rb").write_text("class CabooseHomelab < Formula\nend\n")

            with self.assertRaisesRegex(FileNotFoundError, "caboose-homelab-mcp.rb"):
                brew_release_smoke.brew_smoke_commands(tap_dir=tap_dir)


if __name__ == "__main__":
    unittest.main()
