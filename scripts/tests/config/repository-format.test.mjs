import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { spawnSync } from 'node:child_process';
import path from 'node:path';
import { test } from 'node:test';

const repositoryRoot = path.resolve(new URL('../../..', import.meta.url).pathname);
const editorconfig = readRepositoryFile('.editorconfig');
const gitattributes = readRepositoryFile('.gitattributes');

test('editorconfig keeps repository text formatting stable across editors', () => {
	assert.match(editorconfig, /^root = true$/m);
	const defaults = editorconfigSection('*');
	for (const line of [
		'charset = utf-8',
		'end_of_line = lf',
		'insert_final_newline = true',
		'trim_trailing_whitespace = true',
		'indent_style = space',
		'indent_size = 2',
	]) {
		assert.ok(defaults.has(line), `.editorconfig [*] is missing ${line}`);
	}

	for (const pattern of ['*.go', 'go.mod', 'Makefile']) {
		const section = editorconfigSection(pattern);
		assert.ok(section.has('indent_style = tab'), `.editorconfig [${pattern}] must use tabs`);
	}

	assert.ok(editorconfigSection('*.md').has('trim_trailing_whitespace = false'));
});

test('gitattributes pins text file line endings and binary assets', () => {
	assert.match(gitattributes, /^\* text=auto eol=lf$/m);
	const textPatterns = [
		'*.go',
		'*.js',
		'*.mjs',
		'*.ts',
		'*.tsx',
		'*.json',
		'*.yaml',
		'*.yml',
		'*.md',
		'*.css',
		'*.html',
		'*.conf',
		'*.sql',
		'*.env.example',
		'*.mod',
		'*.sum',
		'Dockerfile*',
	];
	for (const pattern of textPatterns) {
		assert.match(gitattributes, new RegExp(`^${escapeRegExp(pattern)} text eol=lf$`, 'm'), `${pattern} must be pinned to LF`);
	}

	for (const pattern of ['*.png', '*.jpg', '*.jpeg', '*.gif', '*.webp', '*.ico', '*.pdf', '*.zip', '*.xlsx', '*.woff', '*.woff2', '*.ttf', '*.otf']) {
		assert.match(gitattributes, new RegExp(`^${escapeRegExp(pattern)} binary$`, 'm'), `${pattern} must stay binary`);
	}
});

test('gitattributes covers current repository text extensions', () => {
	// 只检查仓库里已经出现的文本类型，新增类型时先明确 LF 规则再提交。
	for (const extension of repositoryTextExtensions()) {
		assert.match(gitattributes, new RegExp(`^\\*\\.${escapeRegExp(extension)} text eol=lf$`, 'm'), `*.${extension} must be pinned to LF`);
	}
});

function readRepositoryFile(path) {
	return readFileSync(new URL(`../../../${path}`, import.meta.url), 'utf8');
}

function repositoryTextExtensions() {
	const binaryExtensions = new Set(['gif', 'ico', 'jpeg', 'jpg', 'otf', 'pdf', 'png', 'ttf', 'webp', 'woff', 'woff2', 'xlsx', 'zip']);
	const extensions = new Set();
	for (const file of trackedRepositoryFiles()) {
		const extension = path.extname(file).slice(1);
		if (file.endsWith('.env.example') || path.basename(file).startsWith('Dockerfile')) {
			continue;
		}
		if (extension && !binaryExtensions.has(extension)) {
			extensions.add(extension);
		}
	}
	return [...extensions].sort();
}

function trackedRepositoryFiles() {
	const result = spawnSync('git', ['ls-files', '-z'], { cwd: repositoryRoot, encoding: 'utf8' });
	assert.equal(result.status, 0, result.stderr);
	return result.stdout.split('\0').filter(Boolean);
}

function editorconfigSection(pattern) {
	const header = `[${pattern}]`;
	const start = editorconfig.indexOf(`${header}\n`);
	assert.notEqual(start, -1, `.editorconfig is missing [${pattern}]`);
	const bodyStart = start + header.length + 1;
	const nextSection = editorconfig.indexOf('\n[', bodyStart);
	const body = editorconfig.slice(bodyStart, nextSection === -1 ? undefined : nextSection);
	return new Set(
		body
			.split(/\r?\n/)
			.map((line) => line.trim())
			.filter(Boolean),
	);
}

function escapeRegExp(value) {
	return value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}
