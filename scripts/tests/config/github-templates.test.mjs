import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { test } from 'node:test';

// 这些测试把 GitHub 协作模板当成仓库契约，防止后续改动丢掉复现、验收和测试信息。
const bugReport = readRepositoryFile('.github/ISSUE_TEMPLATE/bug_report.yml');
const featureRequest = readRepositoryFile('.github/ISSUE_TEMPLATE/feature_request.yml');
const issueTemplateConfig = readRepositoryFile('.github/ISSUE_TEMPLATE/config.yml');
const pullRequestTemplate = readRepositoryFile('.github/PULL_REQUEST_TEMPLATE.md');

const issueAreaOptions = [
	'Backend API',
	'Web app',
	'Mobile app',
	'Docker / deployment config',
	'OpenAPI / generated client',
	'Other',
];

test('pull request template asks reviewers for scope, tests, risk, and rollback details', () => {
	for (const heading of ['## Summary', '## Changes', '## Tests', '## Risk And Rollback', '## Checklist']) {
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

function readRepositoryFile(path) {
	return readFileSync(new URL(`../../../${path}`, import.meta.url), 'utf8');
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
