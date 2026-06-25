import assert from 'node:assert/strict';
import { existsSync, readFileSync } from 'node:fs';
import { test } from 'node:test';

const dockerignores = [
	['root', '../../.dockerignore'],
	['backend', '../../backend/.dockerignore'],
	['web', '../../web/.dockerignore'],
	['mobile', '../../mobile/.dockerignore'],
].map(([name, path]) => [name, dockerignorePatterns(path)]);
const compose = readFileSync(new URL('../../docker-compose.yaml', import.meta.url), 'utf8');
const localCompose = readFileSync(new URL('../../docker-compose.local.yaml', import.meta.url), 'utf8');
const mobileNginx = readFileSync(new URL('../../mobile/nginx.conf', import.meta.url), 'utf8');
const dependabot = readFileSync(new URL('../../.github/dependabot.yml', import.meta.url), 'utf8');
const ciWorkflow = readFileSync(new URL('../../.github/workflows/ci.yaml', import.meta.url), 'utf8');
const rootReadme = readFileSync(new URL('../../readme.md', import.meta.url), 'utf8');
const nvmrc = readFileSync(new URL('../../.nvmrc', import.meta.url), 'utf8').trim();
const packageJSONs = [
	['root', '../../package.json'],
	['web', '../../web/package.json'],
	['mobile', '../../mobile/package.json'],
].map(([name, path]) => [name, JSON.parse(readFileSync(new URL(path, import.meta.url), 'utf8'))]);
const packageLocks = [
	['web', '../../web/package-lock.json'],
	['mobile', '../../mobile/package-lock.json'],
].map(([name, path]) => [name, JSON.parse(readFileSync(new URL(path, import.meta.url), 'utf8'))]);
const packageJSONByName = new Map(packageJSONs);
const packageLockByName = new Map(packageLocks);
const ciNpmPackageDirs = [...ciWorkflow.matchAll(/npm --prefix ([^\s]+) ci/g)].map((match) => match[1]).sort();
const dockerfiles = [
	['backend', '../../backend/Dockerfile'],
	['web', '../../web/Dockerfile'],
	['mobile', '../../mobile/Dockerfile'],
	['postgres', '../../Dockerfile.postgres'],
	['redis', '../../Dockerfile.redis'],
	['mysql', '../../Dockerfile.mysql'],
].map(([name, path]) => [name, readFileSync(new URL(path, import.meta.url), 'utf8')]);

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

test('root docker build context excludes local prototype archives', () => {
	const patterns = dockerignoreByName('root');

	assertHasAnyPattern('root', patterns, ['*.zip', '**/*.zip']);
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

test('Dockerfiles pin explicit base image versions', () => {
	for (const [name, dockerfile] of dockerfiles) {
		assert.doesNotMatch(dockerfile, /^FROM\s+\S+:latest(?:\s|$)/im, `${name} Dockerfile must not use latest base images`);
	}
	assert.match(dockerfileSource('backend'), /^FROM golang:1\.23-bookworm AS builder$/m);
	assert.match(dockerfileSource('backend'), /^FROM alpine:3\.21$/m);
	assert.match(dockerfileSource('web'), /^FROM node:22-bookworm-slim AS deps$/m);
	assert.match(dockerfileSource('web'), /^FROM node:22-bookworm-slim AS runner$/m);
	assert.match(dockerfileSource('mobile'), /^FROM nginxinc\/nginx-unprivileged:1\.27-alpine$/m);
	assert.match(dockerfileSource('postgres'), /^FROM postgres:16$/m);
	assert.match(dockerfileSource('redis'), /^FROM redis:7$/m);
	assert.match(dockerfileSource('mysql'), /^FROM mysql:8\.4$/m);
});

test('backend runtime image includes the migration command and SQL migrations', () => {
	const dockerfile = readFileSync(new URL('../../backend/Dockerfile', import.meta.url), 'utf8');

	assert.match(dockerfile, /go build[\s\S]+-o \/out\/dbmigrate[\s\S]+\.\/cmd\/dbmigrate/);
	assert.match(dockerfile, /COPY --from=builder \/out\/dbmigrate \/app\/dbmigrate/);
	assert.match(dockerfile, /COPY --from=builder \/src\/migrations \/app\/migrations/);
});

test('dependabot watches every Dockerfile directory', () => {
	for (const directory of ['/', '/backend', '/web', '/mobile']) {
		assert.match(dependabot, new RegExp(`package-ecosystem: docker\\n\\s+directory: ${escapeRegExp(directory)}`));
	}
});

test('dependabot only watches npm packages with lockfiles', () => {
	const npmDependabotDirs = dependabotDirectories('npm');
	assert.deepEqual([...npmDependabotDirs].sort(), ciNpmPackageDirs.map((directory) => `/${directory}`));
	for (const directory of ciNpmPackageDirs) {
		assert.ok(packageLockByName.has(directory), `${directory} npm package must have a tracked package-lock.json`);
	}
});

test('dependabot update blocks are rate-limited and scheduled in one maintenance window', () => {
	const blocks = dependabotUpdateBlocks();
	assert.equal(blocks.length, 8);
	for (const block of blocks) {
		assert.match(block, /schedule:\n\s+interval: weekly\n\s+day: monday\n\s+time: "\d{2}:\d{2}"\n\s+timezone: Asia\/Shanghai/);
		assert.match(block, /open-pull-requests-limit: 1/);
		assert.match(block, /cooldown:\n\s+default-days: 7/);
	}
});

test('CI workflow runs the same verification commands documented for local checks', () => {
	for (const [job, command] of [
		['deployment-config', 'npm run verify:compose'],
		['api-contract', 'npm run verify:api-contract'],
		['backend', 'npm run verify:backend'],
		['web', 'npm run verify:web'],
		['web', 'npm run verify:web:e2e'],
		['mobile', 'npm run verify:mobile'],
	]) {
		assert.match(ciWorkflow, new RegExp(`^  ${job}:\\n[\\s\\S]+?- run: ${escapeRegExp(command)}`, 'm'));
	}
	assert.match(ciWorkflow, /permissions:\n\s+contents: read/);
	assert.match(ciWorkflow, /concurrency:\n\s+group: \$\{\{ github\.workflow \}\}-\$\{\{ github\.ref \}\}\n\s+cancel-in-progress: true/);
	assert.match(ciWorkflow, /actions\/setup-node@v4/);
	assert.match(ciWorkflow, /actions\/setup-go@v5/);
});

test('CI workflow keeps pull request checks on read-only credentials', () => {
	assert.match(ciWorkflow, /permissions:\n(?:\s+# .+\n)?\s+contents: read/);
	assert.doesNotMatch(ciWorkflow, /pull_request_target:/);
	assert.doesNotMatch(ciWorkflow, /^\s+[a-z-]+:\s+write$/m);
});

test('CI run steps use explicit bash defaults for pipefail semantics', () => {
	assert.match(ciWorkflow, /defaults:\n\s+run:\n\s+shell: bash/);
});

test('CI workflow only runs branch events that can affect main', () => {
	assert.match(ciWorkflow, /on:\n\s+push:\n\s+branches: \[main\]\n\s+pull_request:\n\s+branches: \[main\]\n\s+workflow_dispatch:/);
});

test('CI jobs pin the hosted runner image instead of following latest', () => {
	const floatingRunners = [...ciWorkflow.matchAll(/runs-on:\s+ubuntu-latest/g)];
	assert.deepEqual(floatingRunners, []);
	const pinnedRunners = [...ciWorkflow.matchAll(/runs-on:\s+ubuntu-24\.04/g)];
	assert.equal(pinnedRunners.length, 5);
});

test('CI jobs set bounded timeouts instead of using the GitHub default', () => {
	const jobs = ciJobBlocks();
	assert.deepEqual([...jobs.keys()].sort(), ['api-contract', 'backend', 'deployment-config', 'mobile', 'web']);
	for (const [job, block] of jobs) {
		const match = block.match(/^\s+timeout-minutes:\s+(\d+)$/m);
		assert.ok(match, `${job} job must set timeout-minutes`);
		const timeoutMinutes = Number(match[1]);
		assert.ok(timeoutMinutes > 0, `${job} timeout-minutes must be positive`);
		assert.ok(timeoutMinutes <= 20, `${job} timeout-minutes must stay below the repository CI budget`);
	}
});

test('CI checkout steps avoid persisting write-capable credentials', () => {
	assert.match(ciWorkflow, /permissions:\n\s+contents: read/);
	const checkoutSteps = [...ciWorkflow.matchAll(/- uses: actions\/checkout@v4/g)];
	assert.ok(checkoutSteps.length > 0, 'CI must use actions/checkout');
	for (const match of checkoutSteps) {
		const step = ciStepFrom(match.index);
		assert.match(step, /with:\n\s+persist-credentials: false/);
	}
});

test('CI npm installs use lockfiles tracked by Dependabot', () => {
	const npmDependabotDirs = dependabotDirectories('npm');
	assert.ok(ciNpmPackageDirs.length > 0, 'CI must install at least one npm package with npm ci');
	for (const directory of ciNpmPackageDirs) {
		assert.ok(packageJSONByName.has(directory), `${directory} npm ci must have package.json metadata`);
		assert.ok(existsSync(new URL(`../../${directory}/package-lock.json`, import.meta.url)), `${directory} npm ci must have package-lock.json`);
		assert.ok(npmDependabotDirs.has(`/${directory}`), `${directory} npm package must be watched by Dependabot`);
		assert.match(ciWorkflow, new RegExp(`cache-dependency-path: ${escapeRegExp(directory)}\\/package-lock\\.json`));
	}
});

test('Node toolchain contract stays aligned across package metadata, CI, and Docker', () => {
	assert.equal(nvmrc, '22');
	assert.equal(packageJSONs[0][1].private, true, 'root package.json must stay private because it only owns repository-level scripts');
	assert.equal(packageJSONs[0][1].dependencies, undefined, 'root package.json must not add runtime dependencies without a root lockfile');
	assert.equal(packageJSONs[0][1].devDependencies, undefined, 'root package.json must not add dev dependencies without a root lockfile');
	for (const [name, pkg] of packageJSONs) {
		assert.equal(pkg.engines?.node, '22.x', `${name} package.json must pin the Node major used by .nvmrc and Docker`);
		assert.equal(pkg.engines?.npm, '>=10 <12', `${name} package.json must declare the supported npm range`);
	}
	for (const name of ciNpmPackageDirs) {
		const pkg = packageJSONByName.get(name);
		const lock = packageLockByName.get(name);
		assert.ok(pkg, `${name} npm package must have package.json metadata`);
		assert.ok(lock, `${name} npm package must have a package-lock.json`);
		assert.deepEqual(lock.packages?.['']?.engines, pkg.engines, `${name} package-lock.json must mirror package.json engines`);
	}
	assert.match(ciWorkflow, /node-version-file: \.nvmrc/);
	for (const directory of ciNpmPackageDirs) {
		assert.match(ciWorkflow, new RegExp(`cache-dependency-path: ${escapeRegExp(directory)}\\/package-lock\\.json`));
	}
	assert.match(dockerfileSource('web'), /^FROM node:22-bookworm-slim AS deps$/m);
	assert.match(dockerfileSource('web'), /^FROM node:22-bookworm-slim AS builder$/m);
	assert.match(dockerfileSource('web'), /^FROM node:22-bookworm-slim AS runner$/m);
	assert.match(dockerfileSource('mobile'), /^FROM node:22-bookworm-slim AS deps$/m);
	assert.match(dockerfileSource('mobile'), /^FROM node:22-bookworm-slim AS builder$/m);
	assert.match(rootReadme, /package\.json` 通过 `engines` 声明同一主版本/);
	assert.doesNotMatch(rootReadme, /npm install/);
	assert.match(rootReadme, /cd web\ncp \.env\.example \.env\.local\nnpm ci\nnpm run dev/);
	assert.match(rootReadme, /cd mobile\ncp \.env\.example \.env\nnpm ci\nnpm run start/);
});

test('migration job stays internal and one-shot', () => {
	const dbmigrate = composeServiceBlock('dbmigrate');
	assert.match(dbmigrate, /restart: "no"/);
	assert.match(dbmigrate, /entrypoint: \["\/app\/dbmigrate"\]/);
	assert.match(dbmigrate, /postgres:\n\s+condition: service_healthy/);
	assert.match(dbmigrate, /networks:\n\s+- internal/);
	assert.doesNotMatch(dbmigrate, /ports:/);
	assert.doesNotMatch(dbmigrate, /expose:/);
	assert.doesNotMatch(dbmigrate, /healthcheck:/);
});

test('stateful compose services keep privilege escalation disabled without read-only roots', () => {
	assert.match(compose, /x-stateful-security: &stateful-security[\s\S]+no-new-privileges:true[\s\S]+cap_drop:\n\s+- ALL/);
	for (const service of ['postgres', 'redis', 'mysql']) {
		const block = composeServiceBlock(service);
		assert.match(block, /<<: \*stateful-security/);
		assert.doesNotMatch(block, /read_only:\s+true/);
	}
});

test('stateful compose healthchecks avoid password command arguments', () => {
	const redis = composeServiceBlock('redis');
	assert.match(redis, /REDISCLI_AUTH=\\?"\$\${REDIS_PASSWORD}\\?" redis-cli ping/);
	assert.doesNotMatch(redis, /redis-cli\s+-a/);

	const mysql = composeServiceBlock('mysql');
	assert.match(mysql, /MYSQL_PWD=\\?"\$\${MYSQL_ROOT_PASSWORD}\\?" mysqladmin ping/);
	assert.doesNotMatch(mysql, /mysqladmin[^\n]+-p/);
});

test('local compose healthchecks avoid password command arguments', () => {
	const redis = composeServiceBlock('redis', localCompose);
	assert.match(redis, /REDISCLI_AUTH=\\?"\$\${REDIS_PASSWORD}\\?" redis-cli ping/);
	assert.doesNotMatch(redis, /redis-cli\s+-a/);

	const mysql = composeServiceBlock('mysql', localCompose);
	assert.match(mysql, /MYSQL_PWD=\\?"\$\${MYSQL_ROOT_PASSWORD}\\?" mysqladmin ping/);
	assert.doesNotMatch(mysql, /mysqladmin[^\n]+-p/);
});

test('local redis compose command keeps password out of process arguments', () => {
	const redis = composeServiceBlock('redis', localCompose);
	const command = redisComposeCommandBlock(redis);
	assert.match(command, /umask 077/);
	assert.match(command, /exec redis-server \/tmp\/redis\.conf/);
	assert.match(command, /requirepass %s/);
	assert.match(command, /\$\$\{REDIS_PASSWORD\}/);
	assert.doesNotMatch(command, /--requirepass/);
});

test('redis compose command keeps password out of process arguments', () => {
	const redis = composeServiceBlock('redis');
	const command = redisComposeCommandBlock(redis);
	assert.match(command, /umask 077/);
	assert.match(command, /exec redis-server \/tmp\/redis\.conf/);
	assert.match(command, /requirepass %s/);
	assert.match(command, /\$\$\{REDIS_PASSWORD\}/);
	assert.doesNotMatch(command, /--requirepass/);
	assert.doesNotMatch(command, /(?<!\$)\$\{REDIS_PASSWORD/);
});

test('mobile nginx keeps the SPA entrypoint revalidated', () => {
	assert.match(mobileNginx, /server_tokens off;/);
	assert.match(mobileNginx, /location = \/index\.html \{[\s\S]+add_header Cache-Control "no-cache" always;/);
	assert.match(mobileNginx, /location \/ \{[\s\S]+try_files \$uri \$uri\/ \/index\.html;/);
});

test('mobile nginx caches static assets immutably without SPA fallback', () => {
	const staticLocation = mobileNginx.match(/location ~\* \\\.\([\s\S]+?\n    \}/)?.[0] || '';
	assert.match(staticLocation, /add_header Cache-Control "public, max-age=31536000, immutable" always;/);
	assert.match(staticLocation, /try_files \$uri =404;/);
	assert.doesNotMatch(staticLocation, /\/index\.html/);
	assert.doesNotMatch(mobileNginx, /expires 30d/);
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

function dockerignoreByName(name) {
	const patterns = dockerignores.find(([candidate]) => candidate === name)?.[1];
	assert.ok(patterns, `missing ${name} .dockerignore patterns`);
	return patterns;
}

function runtimeStageHasNonRootUser(dockerfile) {
	const finalStage = dockerfile.split(/^FROM\s+/im).at(-1);
	return finalStage
		.split(/\r?\n/)
		.map((line) => line.trim())
		.some((line) => /^USER\s+/i.test(line) && !/^USER\s+(?:0|root)(?=$|\s)/i.test(line));
}

function dockerfileSource(name) {
	const dockerfile = dockerfiles.find(([candidate]) => candidate === name)?.[1];
	assert.ok(dockerfile, `missing ${name} Dockerfile source`);
	return dockerfile;
}

function composeServiceBlock(service, source = compose) {
	const match = source.match(new RegExp(`^  ${service}:\\n([\\s\\S]*?)(?=^  [A-Za-z0-9_-]+:|^volumes:)`, 'm'));
	assert.ok(match, `missing compose service ${service}`);
	return match[0];
}

function redisComposeCommandBlock(redis) {
	const match = redis.match(/^\s+command:\n([\s\S]*?)(?=^\s+environment:)/m);
	assert.ok(match, 'missing redis command');
	return match[0];
}

function ciStepFrom(start) {
	const nextStep = ciWorkflow.indexOf('\n      - ', start + 1);
	return ciWorkflow.slice(start, nextStep === -1 ? undefined : nextStep);
}

function ciJobBlocks() {
	// GitHub 默认 job 超时是 360 分钟，这里把每个 CI job 的较短上限固定成仓库契约。
	const jobs = new Map();
	const jobsStart = ciWorkflow.indexOf('\njobs:\n');
	assert.notEqual(jobsStart, -1, 'CI workflow must define jobs');
	const jobsSource = ciWorkflow.slice(jobsStart + '\njobs:\n'.length);
	const jobMatches = [...jobsSource.matchAll(/^  ([A-Za-z0-9_-]+):\n/gm)];
	for (const [index, match] of jobMatches.entries()) {
		const end = jobMatches[index + 1]?.index ?? jobsSource.length;
		jobs.set(match[1], jobsSource.slice(match.index, end));
	}
	return jobs;
}

function dependabotDirectories(ecosystem) {
	return new Set(
		[...dependabot.matchAll(new RegExp(`package-ecosystem: ${escapeRegExp(ecosystem)}\\n\\s+directory: ([^\\n]+)`, 'g'))].map(
			(match) => match[1].trim(),
		),
	);
}

function dependabotUpdateBlocks() {
	return dependabot
		.split(/\n(?=\s+- package-ecosystem: )/)
		.filter((block) => block.includes('package-ecosystem:'));
}

function escapeRegExp(value) {
	return value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}
