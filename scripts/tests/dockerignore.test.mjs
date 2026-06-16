import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { test } from 'node:test';

const dockerignores = [
	['root', '../../.dockerignore'],
	['backend', '../../backend/.dockerignore'],
	['web', '../../web/.dockerignore'],
	['mobile', '../../mobile/.dockerignore'],
].map(([name, path]) => [name, dockerignorePatterns(path)]);

test('docker build contexts exclude local secrets and dependency caches', () => {
	for (const [name, patterns] of dockerignores) {
		assertHasAnyPattern(name, patterns, ['.env', '**/.env']);
		assertHasAnyPattern(name, patterns, ['.env.local', '.env.*', '**/.env.local']);
		assertHasAnyPattern(name, patterns, ['node_modules', '**/node_modules']);
	}
});

test('docker build contexts exclude generated build and test output', () => {
	for (const [name, patterns] of dockerignores) {
		assertHasAnyPattern(name, patterns, ['dist', '**/dist']);
		assertHasAnyPattern(name, patterns, ['build', '**/build']);
		assertHasAnyPattern(name, patterns, ['coverage', '**/coverage']);
		assertHasAnyPattern(name, patterns, ['*.test', '**/*.test']);
	}
});

test('docker build contexts exclude editor and local OS files', () => {
	for (const [name, patterns] of dockerignores) {
		assertHasAnyPattern(name, patterns, ['.DS_Store']);
		assertHasAnyPattern(name, patterns, ['Thumbs.db']);
		assertHasAnyPattern(name, patterns, ['.idea', '.idea/']);
		assertHasAnyPattern(name, patterns, ['.vscode', '.vscode/']);
	}
});

test('application runtime images declare a non-root user', () => {
	for (const [name, path] of [
		['backend', '../../backend/Dockerfile'],
		['web', '../../web/Dockerfile'],
		['mobile', '../../mobile/Dockerfile'],
	]) {
		const dockerfile = readFileSync(new URL(path, import.meta.url), 'utf8');
		assert.ok(runtimeStageHasNonRootUser(dockerfile), `${name} Dockerfile runtime stage must declare a non-root USER`);
	}
});

function dockerignorePatterns(path) {
	return new Set(
		readFileSync(new URL(path, import.meta.url), 'utf8')
			.split(/\r?\n/)
			.map((line) => line.trim())
			.filter((line) => line && !line.startsWith('#')),
	);
}

function assertHasAnyPattern(name, patterns, candidates) {
	assert.ok(
		candidates.some((candidate) => patterns.has(candidate)),
		`${name} .dockerignore is missing one of: ${candidates.join(', ')}`,
	);
}

function runtimeStageHasNonRootUser(dockerfile) {
	const finalStage = dockerfile.split(/^FROM\s+/im).at(-1);
	return finalStage
		.split(/\r?\n/)
		.map((line) => line.trim())
		.some((line) => /^USER\s+/i.test(line) && !/^USER\s+(?:0|root)(?=$|\s)/i.test(line));
}
