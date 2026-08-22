# Actions/RCC private security review packet — 2026-08

PRIVATE SECURITY REVIEW REQUIRED. Do not publish this document, its commits,
or reproductions before coordinated triage.

## Findings

1. The standalone published gateway previously promoted anonymous API traffic
   across an internal trust boundary. The local fix is commit `e683f934`.
2. Codex kick delivery used Ctrl-C as an input-clear operation, exited the CLI,
   and allowed the following Markdown prompt to be interpreted by Bash. The
   local uncommitted fix removes Ctrl-C from Codex delivery and adds unit and
   real-tmux regression coverage.

## Preserved evidence

- Cold backup: `/var/home/kdlocpanda/services/hive-local/backups/20260822T050659Z`
- Durable runtime evidence: `/data/logs/kicks/{supervisor,scanner}` in
  `hive-local_hive-data`
- Published-gateway regression results and audit records remain on the durable
  volume.
- No real credential values are copied into this packet.

## Disclosure gate

Before any upstream disclosure, re-review the minimal reproductions, remove
deployment-specific identities and paths, confirm supported affected versions,
and choose a private maintainer channel. Public GitHub issues are prohibited
until that review explicitly clears them.
