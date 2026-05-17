import unittest

import release_please_automerge

REPO = "caboose-ai/caboose-ai.io"
ALLOWED_AUTHORS = ("cxm6467",)


EXPECTED_CHECKS = (
    "Conventional PR title",
    "Test and build",
    "docs-check",
    "Gitleaks Secret Scan",
    "lint",
)


def release_pr(**overrides):
    pr = {
        "number": 12,
        "title": "chore(main): release 0.9.0",
        "state": "OPEN",
        "isDraft": False,
        "baseRefName": "main",
        "headRefName": "release-please--branches--main--components--caboose-ai.io",
        "labels": [{"name": "autorelease: pending"}],
        "headRepositoryOwner": {"login": "caboose-ai"},
        "headRepository": {
            "name": "caboose-ai.io",
            "nameWithOwner": "caboose-ai/caboose-ai.io",
        },
        "isCrossRepository": False,
        "author": {"login": "cxm6467"},
    }
    pr.update(overrides)
    return pr


def successful_rollup():
    return [
        {
            "__typename": "CheckRun",
            "name": name,
            "status": "COMPLETED",
            "conclusion": "SUCCESS",
        }
        for name in EXPECTED_CHECKS
    ]


class ReleasePleaseAutomergeTest(unittest.TestCase):
    def test_identifies_release_please_prs_only(self):
        self.assertTrue(
            release_please_automerge.is_release_please_pr(
                release_pr(), repo=REPO, allowed_authors=ALLOWED_AUTHORS
            )
        )
        self.assertFalse(
            release_please_automerge.is_release_please_pr(
                release_pr(headRefName="fix/release-pr-auto-approval"),
                repo=REPO,
                allowed_authors=ALLOWED_AUTHORS,
            ),
        )
        self.assertFalse(
            release_please_automerge.is_release_please_pr(
                release_pr(title="feat(homelab): add installer check"),
                repo=REPO,
                allowed_authors=ALLOWED_AUTHORS,
            ),
        )
        self.assertFalse(
            release_please_automerge.is_release_please_pr(
                release_pr(labels=[{"name": "needs-review"}]),
                repo=REPO,
                allowed_authors=ALLOWED_AUTHORS,
            ),
        )

    def test_rejects_untrusted_release_please_sources(self):
        self.assertFalse(
            release_please_automerge.is_release_please_pr(
                release_pr(
                    isCrossRepository=True,
                    headRepositoryOwner={"login": "attacker"},
                    headRepository={
                        "name": "caboose-ai.io",
                        "nameWithOwner": "attacker/caboose-ai.io",
                    },
                ),
                repo=REPO,
                allowed_authors=ALLOWED_AUTHORS,
            ),
        )
        self.assertFalse(
            release_please_automerge.is_release_please_pr(
                release_pr(author={"login": "manual-user"}),
                repo=REPO,
                allowed_authors=ALLOWED_AUTHORS,
            ),
        )
        self.assertFalse(
            release_please_automerge.is_release_please_pr(
                release_pr(
                    headRepository={
                        "name": "other",
                        "nameWithOwner": "caboose-ai/other",
                    }
                ),
                repo=REPO,
                allowed_authors=ALLOWED_AUTHORS,
            ),
        )

    def test_ready_when_expected_checks_succeed(self):
        summary = release_please_automerge.assess_checks(
            successful_rollup()
            + [
                {
                    "__typename": "CheckRun",
                    "name": "Auto-merge Release Please PR",
                    "status": "IN_PROGRESS",
                    "conclusion": None,
                },
            ],
            expected_checks=EXPECTED_CHECKS,
            ignored_checks=("Auto-merge Release Please PR",),
        )

        self.assertEqual(release_please_automerge.CheckState.READY, summary.state)
        self.assertEqual([], summary.missing)
        self.assertEqual([], summary.pending)
        self.assertEqual([], summary.failing)

    def test_waits_for_missing_or_pending_checks(self):
        rollup = successful_rollup()[:-1]
        rollup.append(
            {
                "__typename": "CheckRun",
                "name": "lint",
                "status": "IN_PROGRESS",
                "conclusion": None,
            },
        )

        summary = release_please_automerge.assess_checks(
            rollup,
            expected_checks=EXPECTED_CHECKS,
            ignored_checks=(),
        )

        self.assertEqual(release_please_automerge.CheckState.WAITING, summary.state)
        self.assertEqual([], summary.missing)
        self.assertEqual(["lint"], summary.pending)

        missing = release_please_automerge.assess_checks(
            successful_rollup()[:-1],
            expected_checks=EXPECTED_CHECKS,
            ignored_checks=(),
        )
        self.assertEqual(release_please_automerge.CheckState.WAITING, missing.state)
        self.assertEqual(["lint"], missing.missing)

    def test_blocks_on_failed_check(self):
        rollup = successful_rollup()
        rollup[-1] = {
            "__typename": "CheckRun",
            "name": "lint",
            "status": "COMPLETED",
            "conclusion": "FAILURE",
        }

        summary = release_please_automerge.assess_checks(
            rollup,
            expected_checks=EXPECTED_CHECKS,
            ignored_checks=(),
        )

        self.assertEqual(release_please_automerge.CheckState.BLOCKED, summary.state)
        self.assertEqual(["lint: FAILURE"], summary.failing)

    def test_accepts_legacy_status_context_success(self):
        rollup = successful_rollup()[:-1]
        rollup.append(
            {"__typename": "StatusContext", "context": "lint", "state": "SUCCESS"}
        )

        summary = release_please_automerge.assess_checks(
            rollup,
            expected_checks=EXPECTED_CHECKS,
            ignored_checks=(),
        )

        self.assertEqual(release_please_automerge.CheckState.READY, summary.state)


if __name__ == "__main__":
    unittest.main()
