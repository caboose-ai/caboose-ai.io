import tempfile
import unittest
from pathlib import Path

import update_homebrew_tap


HOMELAB_FORMULA = '''class CabooseHomelab < Formula
  desc "Homelab SSO stack installer and service operator CLI"
  homepage "https://github.com/caboose-ai/caboose-ai.io"
  url "https://github.com/caboose-ai/caboose-ai.io/archive/refs/tags/v0.1.0.tar.gz"
  sha256 "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
  license "Apache-2.0"
end
'''


MCP_FORMULA = '''class CabooseHomelabMcp < Formula
  desc "MCP server for homelab service operations"
  homepage "https://github.com/caboose-ai/caboose-ai.io"
  url "https://github.com/caboose-ai/caboose-ai.io/archive/refs/tags/v0.1.0.tar.gz"
  sha256 "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
  license "Apache-2.0"
end
'''


class UpdateHomebrewTapTest(unittest.TestCase):
    def test_updates_both_formulae_to_release_tag_and_sha(self):
        with tempfile.TemporaryDirectory() as tmp:
            tap_dir = Path(tmp)
            formula_dir = tap_dir / "Formula"
            formula_dir.mkdir()
            homelab = formula_dir / "caboose-homelab.rb"
            mcp = formula_dir / "caboose-homelab-mcp.rb"
            homelab.write_text(HOMELAB_FORMULA)
            mcp.write_text(MCP_FORMULA)

            updated = update_homebrew_tap.update_formulae(
                tap_dir=tap_dir,
                tag="v0.2.0",
                sha256="f" * 64,
                source_repo="caboose-ai/caboose-ai.io",
            )

            expected_url = (
                'url "https://github.com/caboose-ai/caboose-ai.io/'
                'archive/refs/tags/v0.2.0.tar.gz"'
            )
            expected_sha = f'sha256 "{"f" * 64}"'

            self.assertEqual([homelab, mcp], updated)
            self.assertIn(expected_url, homelab.read_text())
            self.assertIn(expected_sha, homelab.read_text())
            self.assertIn(expected_url, mcp.read_text())
            self.assertIn(expected_sha, mcp.read_text())
            self.assertNotIn("v0.1.0", homelab.read_text())
            self.assertNotIn("aaaaaaaa", mcp.read_text())

    def test_missing_formula_fails(self):
        with tempfile.TemporaryDirectory() as tmp:
            tap_dir = Path(tmp)
            (tap_dir / "Formula").mkdir()
            (tap_dir / "Formula" / "caboose-homelab.rb").write_text(HOMELAB_FORMULA)

            with self.assertRaisesRegex(FileNotFoundError, "caboose-homelab-mcp.rb"):
                update_homebrew_tap.update_formulae(
                    tap_dir=tap_dir,
                    tag="v0.2.0",
                    sha256="f" * 64,
                    source_repo="caboose-ai/caboose-ai.io",
                )

    def test_invalid_sha_fails(self):
        with self.assertRaisesRegex(ValueError, "64 lowercase hex"):
            update_homebrew_tap.validate_sha256("not-a-sha")


if __name__ == "__main__":
    unittest.main()
