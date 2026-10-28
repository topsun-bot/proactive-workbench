# Security Policy

## Supported versions

Only the latest release and the `main` branch receive security fixes.

## Reporting a vulnerability

Please report vulnerabilities privately through GitHub's
[private vulnerability reporting](https://github.com/topsun-bot/proactive-workbench/security/advisories/new)
instead of opening a public issue.

Include the affected version or commit, reproduction steps, and the impact you expect. We aim to acknowledge reports within 3 working days and to ship a fix or mitigation as soon as possible, crediting you in the advisory unless you prefer otherwise.

## Scope notes

- `workbench serve` binds to `127.0.0.1` by default. Exposing it on other interfaces is not a supported configuration.
- Long-term memory is stored in a local JSON file; treat it as personal data.
