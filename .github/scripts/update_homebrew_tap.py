#!/usr/bin/env python3
import argparse
import re
from pathlib import Path


FORMULAE = (
    "Formula/caboose-homelab.rb",
    "Formula/caboose-homelab-mcp.rb",
)

TAG_RE = re.compile(r"^v\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.-]+)?$")
SHA256_RE = re.compile(r"^[0-9a-f]{64}$")


def validate_tag(tag: str) -> None:
    if not TAG_RE.match(tag):
        raise ValueError("tag must look like vX.Y.Z")


def validate_sha256(sha256: str) -> None:
    if not SHA256_RE.match(sha256):
        raise ValueError("sha256 must be 64 lowercase hex characters")


def update_formula(path: Path, tag: str, sha256: str, source_repo: str) -> None:
    if not path.exists():
        raise FileNotFoundError(path)

    formula = path.read_text()
    url = f'https://github.com/{source_repo}/archive/refs/tags/{tag}.tar.gz'
    formula, url_count = re.subn(
        r'^\s*url "https://github\.com/[^"]+/archive/refs/tags/[^"]+\.tar\.gz"',
        f'  url "{url}"',
        formula,
        count=1,
        flags=re.MULTILINE,
    )
    formula, sha_count = re.subn(
        r'^\s*sha256 "[0-9a-f]+"',
        f'  sha256 "{sha256}"',
        formula,
        count=1,
        flags=re.MULTILINE,
    )

    if url_count != 1:
        raise ValueError(f"{path} does not contain exactly one source archive url")
    if sha_count != 1:
        raise ValueError(f"{path} does not contain exactly one sha256")

    path.write_text(formula)


def update_formulae(tap_dir: Path, tag: str, sha256: str, source_repo: str) -> list[Path]:
    validate_tag(tag)
    validate_sha256(sha256)

    updated: list[Path] = []
    for formula in FORMULAE:
        path = tap_dir / formula
        update_formula(path, tag, sha256, source_repo)
        updated.append(path)
    return updated


def main() -> int:
    parser = argparse.ArgumentParser(
        description="Update caboose-ai Homebrew tap formulae for a tagged release.",
    )
    parser.add_argument("--tap-dir", type=Path, required=True)
    parser.add_argument("--tag", required=True)
    parser.add_argument("--sha256", required=True)
    parser.add_argument("--source-repo", default="caboose-ai/caboose-ai.io")
    args = parser.parse_args()

    updated = update_formulae(
        tap_dir=args.tap_dir,
        tag=args.tag,
        sha256=args.sha256,
        source_repo=args.source_repo,
    )
    for path in updated:
        print(path)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
