#!/usr/bin/env python3
import argparse
import enum
import json
import os
import re
import subprocess
import sys
import time
from dataclasses import dataclass
from typing import Any


RELEASE_TITLE_RE = re.compile(r"^chore\(main\): release \d+[.]\d+[.]\d+(?:[-+][0-9A-Za-z.-]+)?$")
RELEASE_BRANCH_PREFIX = "release-please--branches--main--"


class CheckState(enum.Enum):
    READY = "ready"
    WAITING = "waiting"
    BLOCKED = "blocked"


@dataclass(frozen=True)
class CheckSummary:
    state: CheckState
    missing: list[str]
    pending: list[str]
    failing: list[str]


def label_names(pr: dict[str, Any]) -> set[str]:
    names: set[str] = set()
    for label in pr.get("labels") or []:
        if isinstance(label, str):
            names.add(label)
        elif isinstance(label, dict):
            name = label.get("name")
            if isinstance(name, str):
                names.add(name)
    return names


def is_release_please_pr(pr: dict[str, Any]) -> bool:
    if pr.get("state") != "OPEN":
        return False
    if pr.get("isDraft"):
        return False
    if pr.get("baseRefName") != "main":
        return False
    if not str(pr.get("headRefName") or "").startswith(RELEASE_BRANCH_PREFIX):
        return False
    if not RELEASE_TITLE_RE.match(str(pr.get("title") or "")):
        return False
    return any(name.startswith("autorelease:") for name in label_names(pr))


def rollup_name(check: dict[str, Any]) -> str:
    return str(check.get("name") or check.get("context") or "")


def check_status(check: dict[str, Any]) -> CheckState:
    typename = check.get("__typename")
    if typename == "StatusContext" or "state" in check:
        state = str(check.get("state") or "").upper()
        if state == "SUCCESS":
            return CheckState.READY
        if state in {"ERROR", "FAILURE"}:
            return CheckState.BLOCKED
        return CheckState.WAITING

    status = str(check.get("status") or "").upper()
    conclusion = str(check.get("conclusion") or "").upper()
    if status != "COMPLETED":
        return CheckState.WAITING
    if conclusion in {"SUCCESS", "NEUTRAL", "SKIPPED"}:
        return CheckState.READY
    return CheckState.BLOCKED


def assess_checks(
    status_check_rollup: list[dict[str, Any]],
    *,
    expected_checks: tuple[str, ...],
    ignored_checks: tuple[str, ...],
) -> CheckSummary:
    ignored = set(ignored_checks)
    by_name: dict[str, list[dict[str, Any]]] = {}
    for check in status_check_rollup:
        name = rollup_name(check)
        if not name or name in ignored:
            continue
        by_name.setdefault(name, []).append(check)

    missing: list[str] = []
    pending: list[str] = []
    failing: list[str] = []
    for expected in expected_checks:
        checks = by_name.get(expected)
        if not checks:
            missing.append(expected)
            continue

        states = [check_status(check) for check in checks]
        if CheckState.BLOCKED in states:
            details = []
            for check in checks:
                if check_status(check) == CheckState.BLOCKED:
                    details.append(str(check.get("conclusion") or check.get("state") or "blocked").upper())
            failing.append(f"{expected}: {', '.join(details)}")
        elif CheckState.WAITING in states:
            pending.append(expected)

    if failing:
        state = CheckState.BLOCKED
    elif missing or pending:
        state = CheckState.WAITING
    else:
        state = CheckState.READY
    return CheckSummary(state=state, missing=missing, pending=pending, failing=failing)


def gh_json(args: list[str], *, token: str | None = None) -> Any:
    command = ["gh", *args]
    env = os.environ.copy()
    if token:
        env["GH_TOKEN"] = token
    result = subprocess.run(command, check=True, capture_output=True, text=True, env=env)
    return json.loads(result.stdout)


def run_gh(args: list[str], *, token: str | None = None, check: bool = True) -> subprocess.CompletedProcess[str]:
    command = ["gh", *args]
    env = os.environ.copy()
    if token:
        env["GH_TOKEN"] = token
    return subprocess.run(command, check=check, text=True, env=env)


def fetch_pr(repo: str, pr_number: int) -> dict[str, Any]:
    fields = ",".join(
        [
            "number",
            "title",
            "url",
            "state",
            "isDraft",
            "baseRefName",
            "headRefName",
            "labels",
            "statusCheckRollup",
        ],
    )
    return gh_json(["pr", "view", str(pr_number), "--repo", repo, "--json", fields])


def approve_pr(repo: str, pr_number: int, *, dry_run: bool) -> None:
    if dry_run:
        print(f"would approve release PR #{pr_number}")
        return

    token = os.environ.get("GH_REVIEW_TOKEN") or os.environ.get("GH_TOKEN")
    result = run_gh(
        [
            "pr",
            "review",
            str(pr_number),
            "--repo",
            repo,
            "--approve",
            "--body",
            "Release Please PR approved automatically after all expected CI checks passed.",
        ],
        token=token,
        check=False,
    )
    if result.returncode != 0:
        print("::notice::Could not submit an approval review; continuing to merge if branch rules allow it.")


def merge_pr(repo: str, pr: dict[str, Any], *, dry_run: bool) -> None:
    pr_number = int(pr["number"])
    title = str(pr["title"])
    if dry_run:
        print(f"would squash-merge release PR #{pr_number}: {title}")
        return

    token = os.environ.get("GH_MERGE_TOKEN") or os.environ.get("GH_TOKEN")
    if not token:
        raise RuntimeError("GH_MERGE_TOKEN or GH_TOKEN is required to merge the Release Please PR")

    run_gh(
        [
            "pr",
            "merge",
            str(pr_number),
            "--repo",
            repo,
            "--squash",
            "--delete-branch",
            "--subject",
            title,
            "--body",
            "Automatically merged after Release Please CI passed.",
        ],
        token=token,
    )


def main() -> int:
    parser = argparse.ArgumentParser(
        description="Approve and merge Release Please PRs once expected CI checks pass.",
    )
    parser.add_argument("--repo", required=True)
    parser.add_argument("--pr", type=int, required=True)
    parser.add_argument(
        "--expected-check",
        action="append",
        default=[],
        help="Check run or status context that must pass before merging.",
    )
    parser.add_argument(
        "--ignore-check",
        action="append",
        default=[],
        help="Check run or status context to ignore when assessing readiness.",
    )
    parser.add_argument("--dry-run", action="store_true")
    parser.add_argument(
        "--wait-attempts",
        type=int,
        default=1,
        help="Number of times to re-check waiting PR checks before exiting.",
    )
    parser.add_argument(
        "--wait-seconds",
        type=float,
        default=0,
        help="Seconds to sleep between waiting check re-reads.",
    )
    args = parser.parse_args()

    expected_checks = tuple(args.expected_check)
    if not expected_checks:
        raise SystemExit("--expected-check is required at least once")
    if args.wait_attempts < 1:
        raise SystemExit("--wait-attempts must be at least 1")

    pr: dict[str, Any] = {}
    summary: CheckSummary | None = None
    for attempt in range(args.wait_attempts):
        pr = fetch_pr(args.repo, args.pr)
        if not is_release_please_pr(pr):
            print(f"PR #{args.pr} is not an open Release Please PR; skipping.")
            return 0

        summary = assess_checks(
            pr.get("statusCheckRollup") or [],
            expected_checks=expected_checks,
            ignored_checks=tuple(args.ignore_check),
        )
        if summary.state != CheckState.WAITING or attempt == args.wait_attempts - 1:
            break
        time.sleep(args.wait_seconds)

    if summary is None:
        raise RuntimeError("check summary was not evaluated")
    if summary.state == CheckState.BLOCKED:
        print(f"Release Please PR #{args.pr} is blocked: {', '.join(summary.failing)}")
        return 1
    if summary.state == CheckState.WAITING:
        waits = []
        if summary.missing:
            waits.append(f"missing: {', '.join(summary.missing)}")
        if summary.pending:
            waits.append(f"pending: {', '.join(summary.pending)}")
        print(f"Release Please PR #{args.pr} is waiting for checks ({'; '.join(waits)}).")
        return 0

    print(f"Release Please PR #{args.pr} has passed expected CI checks.")
    approve_pr(args.repo, args.pr, dry_run=args.dry_run)
    merge_pr(args.repo, pr, dry_run=args.dry_run)
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except subprocess.CalledProcessError as exc:
        print(f"command failed: {' '.join(exc.cmd)}", file=sys.stderr)
        if exc.stdout:
            print(exc.stdout, file=sys.stderr)
        if exc.stderr:
            print(exc.stderr, file=sys.stderr)
        raise SystemExit(exc.returncode)
