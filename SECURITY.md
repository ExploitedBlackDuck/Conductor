# Security Policy

This policy covers vulnerabilities **in Conductor itself**. Vulnerabilities in
rclone should be reported to the [rclone project](https://rclone.org).

## Reporting a vulnerability

Please report security issues privately. Open a
[GitHub security advisory](https://docs.github.com/en/code-security/security-advisories)
on this repository, or email the maintainers at the address listed on the
project page. Do not open a public issue for an unfixed vulnerability.

Include, where possible:

- a description of the issue and its impact;
- the version / commit affected;
- reproduction steps or a proof of concept;
- any suggested remediation.

## Scope

In scope: the Conductor application — the Go core, the Wails binding layer, the
frontend, the daemon supervisor, the rc client, the datastore, secret handling,
and the audit log. Particular areas of interest:

- the webview ↔ Go bridge and the loopback rc channel;
- subprocess construction (must remain argv-only, never a shell);
- the destructive-operation confirmation path (must have no bypass);
- secret handling (rc credentials, the per-install data key) and at-rest sealing;
- audit-log integrity (the hash chain).

Out of scope: vulnerabilities in rclone itself, in the operator's remotes, or in
third-party dependencies (please report those upstream; we will update pins).

## Response expectations

We aim to acknowledge a report within a few business days, agree on a disclosure
timeline, and credit reporters who wish to be credited. Fixes ship as a new
release with published checksums and a `CHANGELOG.md` entry.
