import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { test } from 'node:test';

// 这些测试把 GitHub 协作模板当成仓库契约，防止后续改动丢掉复现、验收和测试信息。
const bugReport = readRepositoryFile('.github/ISSUE_TEMPLATE/bug_report.yml');
const documentationRequest = readRepositoryFile('.github/ISSUE_TEMPLATE/documentation.yml');
const featureRequest = readRepositoryFile('.github/ISSUE_TEMPLATE/feature_request.yml');
const issueTemplateConfig = readRepositoryFile('.github/ISSUE_TEMPLATE/config.yml');
const pullRequestTemplate = readRepositoryFile('.github/PULL_REQUEST_TEMPLATE.md');
const securityPolicy = readRepositoryFile('SECURITY.md');
const contributingGuide = readRepositoryFile('CONTRIBUTING.md');
const codeowners = readRepositoryFile('.github/CODEOWNERS');
const ciWorkflow = readRepositoryFile('.github/workflows/ci.yaml');
const codeqlWorkflow = readRepositoryFile('.github/workflows/codeql.yaml');
const rootReadme = readRepositoryFile('readme.md');
const workflowSources = [
	['ci.yaml', ciWorkflow],
	['codeql.yaml', codeqlWorkflow],
];

const issueAreaOptions = [
	'Backend API',
	'Web app',
	'Mobile app',
	'Docker / deployment config',
	'OpenAPI / generated client',
	'Other',
];

test('pull request template asks reviewers for scope, tests, review focus, risk, and rollback details', () => {
	for (const heading of ['## Summary', '## Changes', '## Reviewer Notes', '## Tests', '## Risk And Rollback', '## Checklist']) {
		assert.match(pullRequestTemplate, new RegExp(`^${escapeRegExp(heading)}$`, 'm'));
	}

	for (const command of [
		'npm run verify:backend',
		'npm run verify:api-contract',
		'npm run verify:web',
		'npm run verify:web:e2e',
		'npm run verify:mobile',
	]) {
		assert.match(pullRequestTemplate, new RegExp(escapeRegExp(command)));
	}

	for (const requiredPrompt of [
		'Related issue:',
		'Review focus:',
		'Screenshots or recording:',
		'Security or dependency notes:',
		'Risk level: Low / Medium / High',
		'User-facing or API compatibility impact:',
		'Config, migration, or deployment notes:',
		'Rollback plan:',
		'Tests or contract checks cover the changed behavior.',
		'OpenAPI, generated clients, docs, or `.env.example` are updated when relevant.',
		'No secrets, local-only files, or unrelated formatting changes are included.',
	]) {
		assert.match(pullRequestTemplate, new RegExp(escapeRegExp(requiredPrompt)));
	}
});

test('issue forms keep the GitHub-required top-level shape and repository defaults', () => {
	for (const [name, source] of [
		['bug report', bugReport],
		['documentation update', documentationRequest],
		['feature request', featureRequest],
	]) {
		assert.match(source, /^name:\s+\S/m, `${name} issue form must define a display name`);
		assert.match(source, /^description:\s+\S/m, `${name} issue form must define a description`);
		assert.match(source, /^title:\s+"/m, `${name} issue form must define a default title`);
		assert.match(source, /^labels:\s+\[/m, `${name} issue form must attach triage labels`);
		assert.match(source, /^body:$/m, `${name} issue form must define a body`);
		assert.match(source, /^  - type: (?!markdown\b)[a-z]+$/m, `${name} issue form must collect user input`);
		assertUniqueIds(name, source);
	}
});

test('documentation form requires affected area, current docs, problem, expected update, and acceptance criteria', () => {
	for (const option of [
		'Backend API',
		'Web app',
		'Mobile app',
		'Docker / deployment config',
		'OpenAPI / generated client',
		'Middleware / runtime behavior',
		'Repository configuration',
		'Other',
	]) {
		assert.match(issueBlock(documentationRequest, 'area'), new RegExp(`- ${escapeRegExp(option)}`));
	}

	for (const requiredField of ['area', 'current', 'problem', 'expected', 'acceptance']) {
		assertRequired(documentationRequest, requiredField);
	}

	assert.match(documentationRequest, /^labels:\s+\["documentation"\]$/m);
	assert.match(issueBlock(documentationRequest, 'current'), /File or URL:/);
	assert.match(issueBlock(documentationRequest, 'expected'), /命令、接口、配置项或示例/);
	assert.match(issueBlock(documentationRequest, 'acceptance'), /placeholder:\s+\|[\s\S]+- \[ \] \.\.\./);
});

test('bug report form requires enough information to reproduce and diagnose defects', () => {
	for (const option of issueAreaOptions) {
		assert.match(issueBlock(bugReport, 'area'), new RegExp(`- ${escapeRegExp(option)}`));
	}

	for (const requiredField of ['area', 'steps', 'expected', 'actual', 'environment']) {
		assertRequired(bugReport, requiredField);
	}

	assert.match(issueBlock(bugReport, 'config'), /render: yaml/);
	assert.match(issueBlock(bugReport, 'logs'), /render: text/);
});

test('feature request form requires problem, proposal, ownership area, and acceptance criteria', () => {
	for (const option of issueAreaOptions) {
		assert.match(issueBlock(featureRequest, 'area'), new RegExp(`- ${escapeRegExp(option)}`));
	}

	for (const requiredField of ['problem', 'proposal', 'area', 'acceptance']) {
		assertRequired(featureRequest, requiredField);
	}

	assert.match(issueBlock(featureRequest, 'acceptance'), /placeholder:\s+\|[\s\S]+- \[ \] \.\.\./);
});

test('blank issue creation stays disabled so contributors choose a structured form', () => {
	assert.match(issueTemplateConfig, /^blank_issues_enabled: false$/m);
});

test('issue chooser routes security reports and questions away from public issues', () => {
	for (const requiredText of [
		'contact_links:',
		'Security vulnerability report',
		'/security/advisories/new',
		'suspected vulnerabilities privately',
		'Questions and discussions',
		'/discussions',
		'API contract documentation',
		'/tree/main/backend/api',
		'OpenAPI contracts',
	]) {
		assert.match(issueTemplateConfig, new RegExp(escapeRegExp(requiredText)));
	}
});

test('security policy documents private vulnerability reporting and supported branches', () => {
	for (const heading of ['## Supported Versions', '## Reporting a Vulnerability', '## Triage Expectations', '## Scope']) {
		assert.match(securityPolicy, new RegExp(`^${escapeRegExp(heading)}$`, 'm'));
	}

	for (const requiredText of [
		'`main`',
		'`dev-pxhao`',
		'Do not open a public issue',
		'GitHub private vulnerability reporting',
		'Affected branch or commit.',
		'Steps to reproduce.',
		'Impact and affected data or capability.',
		'Confirmed vulnerabilities should stay private',
		'Authentication, authorization, token handling',
		'Docker, CI, dependency maintenance, and deployment configuration',
	]) {
		assert.match(securityPolicy, new RegExp(escapeRegExp(requiredText)));
	}
});

test('contributing guide documents branch flow, issue triage, verification, contracts, and security reporting', () => {
	for (const heading of ['## Branch And Pull Request Flow', '## Issue Triage', '## Local Verification', '## API And Runtime Contracts', '## Security']) {
		assert.match(contributingGuide, new RegExp(`^${escapeRegExp(heading)}$`, 'm'));
	}

	for (const requiredText of [
		'`dev-pxhao`',
		'pull request template',
		'npm run verify:compose',
		'npm run verify:api-contract',
		'npm run verify:backend',
		'npm run verify:web',
		'npm run verify:mobile',
		'npm run verify:web:e2e',
		'npm run generate:api-types',
		'backend/api/openapi.yaml',
		'backend/api/health-openapi.yaml',
		'.env.example',
		'SECURITY.md',
		'GitHub issue chooser',
		'private vulnerability reporting',
		'OpenAPI contract directory',
		'bug report form',
		'feature request form',
		'documentation form',
		'reproducible defects',
		'README, OpenAPI, middleware, deployment, or repository configuration guidance',
		'Dependency updates are maintainer-initiated only',
		'Do not enable repository automation that creates bot branches or default dependency pull requests',
		'prepare dependency changes on `dev-pxhao`',
	]) {
		assert.match(contributingGuide, new RegExp(escapeRegExp(requiredText)));
	}
});

test('codeowners routes critical areas to the repository owner', () => {
	assert.match(codeowners, /^\* @QX-hao$/m);

	for (const path of [
		'/.github/',
		'/backend/api/',
		'/scripts/generate-api-types.mjs',
		'/backend/cmd/server/',
		'/backend/internal/middleware/',
		'/backend/internal/config/',
		'/docker-compose.yaml',
		'/Dockerfile.*',
	]) {
		assert.match(codeowners, new RegExp(`^${escapeRegExp(path)}\\s+@QX-hao$`, 'm'), `${path} must be owned`);
	}

	for (const line of codeowners.split(/\r?\n/)) {
		if (!line.trim() || line.trimStart().startsWith('#')) continue;
		assert.match(line, /^\S+\s+@\S+$/, `CODEOWNERS line must include path and owner: ${line}`);
	}
});

test('root readme links collaboration, security, ownership, and API contract entrypoints', () => {
	for (const requiredText of [
		'## 协作与安全',
		'CONTRIBUTING.md',
		'SECURITY.md',
		'.github/CODEOWNERS',
		'backend/api/openapi.yaml',
		'backend/api/health-openapi.yaml',
		'npm run verify:api-contract',
		'CodeQL',
		'Go 和 JavaScript/TypeScript',
		'Web 的 Next 配置和 Mobile Web 的 Nginx 配置',
		'Cross-Origin-Opener-Policy: same-origin',
		'Origin-Agent-Cluster: ?1',
		'静态前端默认不写 `Strict-Transport-Security`、`Content-Security-Policy` 或 `Cross-Origin-Embedder-Policy`',
	]) {
		assert.match(rootReadme, new RegExp(escapeRegExp(requiredText)));
	}
});

test('ci runs on main and active development branch pushes', () => {
	assert.match(ciWorkflow, /^  push:\n    branches: \[main, dev-pxhao\]$/m);
	assert.match(ciWorkflow, /^  pull_request:\n    branches: \[main, dev-pxhao\]$/m);
});

test('ci cancels superseded runs on the same workflow ref', () => {
	assert.match(ciWorkflow, /^concurrency:\n  group: \$\{\{ github\.workflow \}\}-\$\{\{ github\.ref \}\}\n  cancel-in-progress: true$/m);
});

test('codeql workflow scans Go and TypeScript with least required permissions', () => {
	assert.match(codeqlWorkflow, /^name: CodeQL$/m);
	assert.match(codeqlWorkflow, /^  push:\n    branches: \[main, dev-pxhao\]$/m);
	assert.match(codeqlWorkflow, /^  pull_request:\n    branches: \[main, dev-pxhao\]$/m);
	assert.match(codeqlWorkflow, /^  schedule:\n    - cron: '[^']+'$/m);
	assert.match(codeqlWorkflow, /^  workflow_dispatch:$/m);
	assert.match(codeqlWorkflow, /^permissions:\n  actions: read\n  contents: read\n  security-events: write$/m);
	assert.match(codeqlWorkflow, /^concurrency:\n  group: \$\{\{ github\.workflow \}\}-\$\{\{ github\.ref \}\}\n  cancel-in-progress: true$/m);
	assert.match(codeqlWorkflow, /runs-on: ubuntu-24\.04/);
	assert.match(codeqlWorkflow, /timeout-minutes: 20/);
	assert.match(codeqlWorkflow, /strategy:\n      fail-fast: false\n      matrix:/);
	assert.match(codeqlWorkflow, /language: go[\s\S]+build-mode: autobuild/);
	assert.match(codeqlWorkflow, /language: javascript-typescript[\s\S]+build-mode: none/);
	assert.match(codeqlWorkflow, /actions\/checkout@[a-f0-9]{40}\s+# v7[\s\S]+persist-credentials: false/);
	assert.doesNotMatch(codeqlWorkflow, /actions\/checkout@v4/);
	assert.match(codeqlWorkflow, /github\/codeql-action\/init@[a-f0-9]{40}\s+# v4/);
	assert.match(codeqlWorkflow, /queries: security-extended/);
	assert.match(codeqlWorkflow, /if: matrix\.build-mode == 'autobuild'\n        uses: github\/codeql-action\/autobuild@[a-f0-9]{40}\s+# v4/);
	assert.match(codeqlWorkflow, /github\/codeql-action\/analyze@[a-f0-9]{40}\s+# v4/);
	assert.doesNotMatch(codeqlWorkflow, /github\/codeql-action\/(?:init|autobuild|analyze)@v3/);
	assert.match(codeqlWorkflow, /category: '\/language:\$\{\{ matrix\.language \}\}'/);
});

test('remote workflow actions are pinned to immutable commits with version comments', () => {
	for (const [name, workflow] of workflowSources) {
		const usages = workflowActionUsages(workflow);
		assert.ok(usages.length > 0, `${name} must use at least one remote action`);
		for (const usage of usages) {
			assert.match(usage.ref, /^[a-f0-9]{40}$/, `${name} must pin ${usage.action} to a full commit SHA`);
			assert.match(usage.versionComment, /^v\d+$/, `${name} must document the reviewed major for ${usage.action}`);
		}
	}
});

function readRepositoryFile(path) {
	return readFileSync(new URL(`../../../${path}`, import.meta.url), 'utf8');
}

function workflowActionUsages(workflow) {
	return [...workflow.matchAll(/^\s+-?\s*uses:\s+([^@\s#]+\/[^@\s#]+)@([^\s#]+)(?:\s+#\s+(\S+))?$/gm)].map((match) => ({
		action: match[1],
		ref: match[2],
		versionComment: match[3] ?? '',
	}));
}

// Issue Form 没有官方本地校验器，这里用轻量文本断言覆盖本仓库必须保留的字段。
function issueBlock(source, id) {
	const marker = `id: ${id}`;
	const start = source.indexOf(marker);
	assert.notEqual(start, -1, `issue form must include field id "${id}"`);

	const rest = source.slice(start);
	const nextField = rest.indexOf('\n  - type:', 1);
	return nextField === -1 ? rest : rest.slice(0, nextField);
}

function assertRequired(source, id) {
	assert.match(issueBlock(source, id), /validations:\n\s+required: true/, `field "${id}" must be required`);
}

function assertUniqueIds(name, source) {
	const ids = [...source.matchAll(/^\s+id:\s+([a-zA-Z0-9_-]+)$/gm)].map((match) => match[1]);
	assert.ok(ids.length > 0, `${name} issue form must define stable field ids`);
	assert.deepEqual(ids, [...new Set(ids)], `${name} issue form field ids must be unique`);
}

function escapeRegExp(value) {
	return value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}
