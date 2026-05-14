#!/usr/bin/env python3
import argparse
import shlex
import subprocess
from pathlib import Path
from typing import Callable, Sequence


FORMULAE = (
    "caboose-homelab",
    "caboose-homelab-mcp",
)

Runner = Callable[[Sequence[str]], None]


def validate_formulae(tap_dir: Path) -> None:
    missing = [
        tap_dir / "Formula" / f"{formula}.rb"
        for formula in FORMULAE
        if not (tap_dir / "Formula" / f"{formula}.rb").exists()
    ]
    if missing:
        raise FileNotFoundError(
            "missing Homebrew formulae: " + ", ".join(str(path) for path in missing),
        )


def brew_smoke_commands(tap_dir: Path, tap_name: str = "caboose-ai/tap") -> list[list[str]]:
    validate_formulae(tap_dir)

    commands: list[list[str]] = [
        ["brew", "tap", "--custom-remote", tap_name, tap_dir.resolve().as_uri()],
    ]
    for formula in FORMULAE:
        name = f"{tap_name}/{formula}"
        commands.append(["brew", "install", "--build-from-source", name])
        commands.append(["brew", "test", name])
    return commands


def run_commands(commands: Sequence[Sequence[str]], runner: Runner | None = None) -> None:
    if runner is None:
        runner = lambda command: subprocess.run(command, check=True)

    for command in commands:
        runner(command)


def main() -> int:
    parser = argparse.ArgumentParser(
        description="Install and brew-test both caboose-ai Homebrew formulae.",
    )
    parser.add_argument("--tap-dir", type=Path, required=True)
    parser.add_argument("--tap-name", default="caboose-ai/tap")
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="Print the brew commands without running them.",
    )
    args = parser.parse_args()

    commands = brew_smoke_commands(tap_dir=args.tap_dir, tap_name=args.tap_name)
    if args.dry_run:
        for command in commands:
            print(" ".join(shlex.quote(part) for part in command))
        return 0

    run_commands(commands)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
