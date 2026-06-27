# Security Policy

## Supported Versions

Security fixes are handled on the default branch and the active development branch used by this repository.

| Branch | Supported |
| --- | --- |
| `main` | Yes |
| `dev-pxhao` | Yes |

## Reporting a Vulnerability

Do not open a public issue for suspected vulnerabilities, leaked secrets, authentication bypasses, dependency supply-chain findings, or data exposure reports.

Report security issues through GitHub private vulnerability reporting or a private GitHub security advisory for this repository. Include:

- Affected branch or commit.
- Steps to reproduce.
- Impact and affected data or capability.
- Logs, requests, screenshots, or proof of concept with secrets removed.
- Whether the issue affects backend, web, mobile, Docker/deployment config, OpenAPI/generated clients, or dependencies.

## Triage Expectations

Security reports should receive an initial triage response before ordinary feature work. Confirmed vulnerabilities should stay private until a fix is available, tested, and released to the affected branches.

## Scope

In scope:

- Authentication, authorization, token handling, session refresh, and logout behavior.
- API request validation, import/export handling, and generated client contracts.
- Docker, CI, dependency maintenance, and deployment configuration that could expose secrets or weaken isolation.
- Dependency vulnerabilities that affect reachable runtime code.

Out of scope:

- Reports without a reproducible impact.
- Social engineering, spam, or denial-of-service testing against infrastructure not owned by this repository.
- Public disclosure before maintainers have had a reasonable chance to investigate and fix the issue.
