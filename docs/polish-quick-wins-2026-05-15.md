# Polishing Quick Wins - May 15, 2026

Status: initial pass completed in this run.

Done now:
- Confirmed `.env` is ignored and not tracked in current refs (see Security Notice).
- Added CI secret scanning for new push and PR commit ranges via Gitleaks
  (.github/workflows/secret-scan.yml).
- Added Security sections to README.md, CLAUDE.md, and .github/copilot-instructions.md.

Recommended follow-ups:
- Rotate historical credentials flagged by the local full-history Gitleaks trial,
  then decide whether to baseline old findings or rewrite history.
- CONTRIBUTING.md: local dev bootstrap, Go PATH fallback (/home/caboose/.local/go/bin), mise workflow, test/build targets, docs rules.
- Add go vet/staticcheck step to CI (separate from build) to tighten code hygiene.
- Devcontainer.json for VS Code with Go + Docker CLI preinstalled.
- Expand docs/homelab-agent-control-plan.md cross-links from service READMEs.
- Optional: pre-commit hook for gitleaks + whitespace checks.

Notes:
- Go and mise are present in the operator environment; use the repository
  validation commands before promoting this branch.
- A full-history Gitleaks scan currently reports pre-existing redacted findings
  in older commits, so the new CI gate intentionally scans only new commit
  ranges until those historical findings are rotated and handled.
