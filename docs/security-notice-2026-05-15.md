# Security Notice - Local .env Exposure Review (May 15, 2026)

During CAB-1 polish work, a local root `.env` file was present in the workspace
that Paperclip agents can read. Current git refs do not show `.env` or
`dev/homelab/.env` as tracked files, but any live values exposed to local agent
runs or logs should still be treated as sensitive.

Immediate remediation steps:
- Rotate any values from local `.env` files that were live credentials and may
  have appeared in agent transcripts, logs, screenshots, or pasted output.
- Rotate historical credentials flagged by the local full-history Gitleaks trial
  before enabling a full-history CI gate.
- Audit access logs for unusual activity for any rotated service credentials.
- Keep local operators on 1Password-backed secrets or untracked `.env` files.
- Keep placeholder examples in `dev/homelab/.env.example`.
- Use CI secret scanning to prevent future commits with secrets.

Repository hygiene changes in this run:
- Added a Gitleaks workflow and repo config.
- Reinforced `.env` and live-secret policy in README, CLAUDE, and Copilot
  instructions.
- Verified `.env` and `dev/homelab/.env` are ignored and not tracked in current
  refs.
- Confirmed a full-history Gitleaks run finds pre-existing redacted findings in
  older commits, so CI scans only new commit ranges for now.

Notes:
- Git history for current refs did not show tracked `.env` files. If another
  branch, fork, backup, or external log contains live secrets, rotate those
  values and then decide whether history scrubbing is worth the operational
  risk.
