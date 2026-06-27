# Contributing

## Branch And Pull Request Flow

- Use `dev-pxhao` for active development work.
- Keep changes focused by area: backend, web, mobile, Docker/deployment config, OpenAPI/generated client, or repository configuration.
- Fill out the pull request template with summary, related issue, review focus, tests, risk, and rollback notes.
- Do not include secrets, local `.env` files, generated build output, dependency caches, or unrelated formatting-only changes.
- Dependency updates are maintainer-initiated only. Do not enable repository automation that creates bot branches or default dependency pull requests; prepare dependency changes on `dev-pxhao` with the same focused scope, verification, and review notes as any other change.

## Issue Triage

- Use the bug report form only for reproducible defects with steps, expected behavior, actual behavior, and environment details.
- Use the feature request form for new capabilities or workflow improvements with acceptance criteria.
- Use the documentation form for missing, unclear, or outdated README, OpenAPI, middleware, deployment, or repository configuration guidance.
- Keep security reports out of public issues and use the private path documented in `SECURITY.md`.

## Local Verification

Run the checks that match the changed area before opening a pull request:

```bash
npm run verify:compose
npm run verify:api-contract
npm run verify:backend
npm run verify:web
npm run verify:mobile
```

Run browser end-to-end tests when the change affects web user flows:

```bash
npm run verify:web:e2e
```

If a check is intentionally skipped, explain the reason in the pull request `Tests` section.

## API And Runtime Contracts

- Update `backend/api/openapi.yaml` when business API request or response shapes change.
- Run `npm run generate:api-types` after OpenAPI changes so web and mobile generated clients stay aligned.
- Keep `backend/api/health-openapi.yaml` aligned with `/livez`, `/readyz`, and `/health` behavior.
- Update `.env.example`, Docker Compose files, README files, or package docs when configuration behavior changes.
- Add or update focused tests for middleware, handlers, generated clients, and repository configuration contracts.

## Security

Do not report suspected vulnerabilities in public issues or pull request comments. Follow `SECURITY.md` for private reporting, supported branches, and disclosure expectations.
The GitHub issue chooser also links directly to private vulnerability reporting, discussions, and the OpenAPI contract directory so public issues stay focused on reproducible bugs and accepted feature requests.
