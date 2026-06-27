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

test('backend Docker builds are reproducible without VCS stamping', () => {
	const buildCommands = [...dockerfileSource('backend').matchAll(/go build \\\n([\s\S]*?)-o \/out\/(?:haohaoaccounting|dbmigrate)/g)];
	assert.equal(buildCommands.length, 2, 'backend Dockerfile must build server and migration binaries');
	for (const command of buildCommands) {
		assert.match(command[0], /-buildvcs=false/);
		assert.match(command[0], /-trimpath/);
	}
});

test('dependabot watches every Dockerfile directory', () => {
	assert.deepEqual([...dependabotDirectories('docker')].sort(), ['/', '/backend', '/mobile', '/web']);
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
	assert.equal(blocks.length, 5);
	for (const block of blocks) {
		assert.match(block, /schedule:\n\s+interval: weekly\n\s+day: monday\n\s+time: "\d{2}:\d{2}"\n\s+timezone: Asia\/Shanghai/);
		assert.match(block, /open-pull-requests-limit: 1/);
		assert.match(block, /commit-message:\n\s+prefix: deps\([a-z][a-z0-9-]*\)/);
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
	assertWorkflowUsesMinimumActionMajor(ciWorkflow, 'actions/checkout', 7);
	assertWorkflowUsesMinimumActionMajor(ciWorkflow, 'actions/setup-node', 6);
	assertWorkflowUsesMinimumActionMajor(ciWorkflow, 'actions/setup-go', 6);
});

test('API contract verification fails on generated and runtime client drift', () => {
	const command = packageJSONByName.get('root').scripts['verify:api-contract'];
	for (const file of [
		'web/shared/types/api.ts',
		'mobile/src/shared/types/api.ts',
		'web/shared/api/generated-client.ts',
		'mobile/src/shared/api/generated-client.ts',
		'web/shared/api/client.ts',
		'mobile/src/shared/api/client.ts',
	]) {
		assert.match(command, new RegExp(escapeRegExp(file)), `verify:api-contract must check ${file}`);
	}
});

test('CI workflow keeps pull request checks on read-only credentials', () => {
	assert.match(ciWorkflow, /permissions:\n(?:\s+# .+\n)?\s+contents: read/);
	assert.doesNotMatch(ciWorkflow, /pull_request_target:/);
	assert.doesNotMatch(ciWorkflow, /^\s+[a-z-]+:\s+write$/m);
});

test('CI run steps use explicit strict bash defaults for pipefail semantics', () => {
	assert.match(ciWorkflow, /defaults:\n\s+run:\n\s+# .+\n\s+shell: bash --noprofile --norc -euo pipefail \{0\}/);
});

test('CI workflow runs only maintained branch events', () => {
	assert.match(ciWorkflow, /on:\n\s+push:\n\s+branches: \[main, dev-pxhao\]\n\s+pull_request:\n\s+branches: \[main, dev-pxhao\]\n\s+workflow_dispatch:/);
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
	const checkoutSteps = [...workflowActionUsages(ciWorkflow)].filter(({ action }) => action === 'actions/checkout');
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

test('CI npm installs skip audit and funding network noise', () => {
	for (const directory of ['web', 'mobile']) {
		assert.match(
			ciWorkflow,
			new RegExp(`- run: npm --prefix ${escapeRegExp(directory)} ci --no-audit --no-fund`),
			`${directory} CI install must stay frozen without automatic audit or funding calls`,
		);
	}
});

test('Docker npm installs use lockfiles without audit or funding network calls', () => {
	for (const name of ['web', 'mobile']) {
		assert.match(dockerfileSource(name), /^RUN npm ci --no-audit --no-fund$/m, `${name} Dockerfile must use reproducible, quiet npm installs`);
	}
});

test('frontend Docker builds disable framework telemetry', () => {
	assert.match(dockerfileSource('web'), /^ENV NEXT_TELEMETRY_DISABLED=1$/m);
	assert.match(dockerfileSource('mobile'), /^ENV EXPO_NO_TELEMETRY=1$/m);
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

test('production compose requires explicit deployment secrets and public endpoints', () => {
	for (const variable of [
		'NEXT_PUBLIC_API_BASE',
		'POSTGRES_PASSWORD',
		'REDIS_PASSWORD',
		'JWT_SECRET',
		'ADMIN_USERNAME',
		'ADMIN_PASSWORD',
		'CORS_ALLOW_ORIGINS',
		'MYSQL_ROOT_PASSWORD',
	]) {
		assert.match(compose, new RegExp(`\\$\\{${variable}:\\?set `), `${variable} must use Compose required-variable syntax`);
		assert.doesNotMatch(compose, new RegExp(`\\$\\{${variable}:-`), `${variable} must not have a production fallback`);
	}
	assert.doesNotMatch(compose, /JWT_SECRET:\s+haohao-dev-jwt-secret-change-me-32chars/);
	assert.doesNotMatch(compose, /ADMIN_PASSWORD:\s+haohao123/);
	assert.doesNotMatch(compose, /REDIS_PASSWORD:\s+haohao123/);
});

test('stateful compose services keep privilege escalation disabled without read-only roots', () => {
	assert.match(compose, /x-stateful-security: &stateful-security[\s\S]+no-new-privileges:true[\s\S]+cap_drop:\n\s+- ALL/);
	for (const service of ['postgres', 'redis', 'mysql']) {
		const block = composeServiceBlock(service);
		assert.match(block, /<<: \*stateful-security/);
		assert.doesNotMatch(block, /read_only:\s+true/);
	}
});

test('production compose network boundaries keep stateful services internal-only', () => {
	assert.match(compose, /^  internal:\n\s+internal: true$/m);

	const web = composeServiceBlock('web');
	assert.match(web, /ports:\n\s+- "3000:3000"/);
	assert.match(web, /networks:\n\s+- public/);
	assert.doesNotMatch(web, /\n\s+- internal/);

	const backend = composeServiceBlock('backend');
	assert.match(backend, /ports:\n\s+- "8080:8080"/);
	assert.match(backend, /networks:\n\s+- public\n\s+- internal/);

	const dbmigrate = composeServiceBlock('dbmigrate');
	assert.match(dbmigrate, /networks:\n\s+- internal/);
	assert.doesNotMatch(dbmigrate, /\n\s+- public/);
	assert.doesNotMatch(dbmigrate, /ports:/);
	assert.doesNotMatch(dbmigrate, /expose:/);

	for (const [service, port] of [
		['postgres', '5432'],
		['redis', '6379'],
		['mysql', '3306'],
	]) {
		const block = composeServiceBlock(service);
		// 状态服务只暴露给 Compose 内部网络，避免数据库端口意外发布到宿主机。
		assert.match(block, /networks:\n\s+- internal/);
		assert.doesNotMatch(block, /\n\s+- public/);
		assert.doesNotMatch(block, /ports:/);
		assert.match(block, new RegExp(`expose:\\n\\s+- "${port}"`));
	}
});

test('backend compose stop grace period exceeds application shutdown timeout', () => {
	const backend = composeServiceBlock('backend');
	const backendEnv = composeAnchorBlock('x-backend-env');

	assert.match(backend, /stop_grace_period: \$\{BACKEND_STOP_GRACE_PERIOD:-30s\}/);
	assert.match(backendEnv, /HTTP_SHUTDOWN_TIMEOUT: \$\{HTTP_SHUTDOWN_TIMEOUT:-10s\}/);
	const stopGrace = defaultDurationSeconds(backend, 'BACKEND_STOP_GRACE_PERIOD');
	const shutdownTimeout = defaultDurationSeconds(backendEnv, 'HTTP_SHUTDOWN_TIMEOUT');

	// 容器停机宽限必须大于应用优雅停机预算，给 HTTP server shutdown 和日志 flush 留出余量。
	assert.ok(stopGrace > shutdownTimeout, `backend stop_grace_period ${stopGrace}s must exceed HTTP_SHUTDOWN_TIMEOUT ${shutdownTimeout}s`);
});

test('production compose services rotate json-file logs', () => {
	assert.match(compose, /x-default-logging: &default-logging[\s\S]+driver: json-file[\s\S]+max-size: "10m"[\s\S]+max-file: "5"/);
	for (const service of ['web', 'backend', 'dbmigrate', 'postgres', 'redis', 'mysql']) {
		const block = composeServiceBlock(service);
		assert.match(block, /logging: \*default-logging/, `${service} must use the shared logging policy`);
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

test('mobile nginx sets baseline browser security headers', () => {
	const indexLocation = mobileNginx.match(/location = \/index\.html \{[\s\S]+?\n    \}/)?.[0] || '';
	const staticLocation = mobileNginx.match(/location ~\* \\\.\([\s\S]+?\n    \}/)?.[0] || '';
	for (const [header, value] of [
		['Referrer-Policy', 'strict-origin-when-cross-origin'],
		['Permissions-Policy', 'camera=(), geolocation=(), microphone=(), payment=()'],
		['X-Content-Type-Options', 'nosniff'],
		['X-Frame-Options', 'DENY'],
	]) {
		assert.match(mobileNginx, new RegExp(`add_header ${escapeRegExp(header)} "${escapeRegExp(value)}" always;`));
		assert.match(indexLocation, new RegExp(`add_header ${escapeRegExp(header)} "${escapeRegExp(value)}" always;`));
		assert.match(staticLocation, new RegExp(`add_header ${escapeRegExp(header)} "${escapeRegExp(value)}" always;`));
	}
	assert.doesNotMatch(mobileNginx, /Strict-Transport-Security/);
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

function composeAnchorBlock(anchor) {
	const match = compose.match(new RegExp(`^${escapeRegExp(anchor)}:.*\\n([\\s\\S]*?)(?=^services:)`, 'm'));
	assert.ok(match, `missing compose anchor ${anchor}`);
	return match[0];
}

function defaultDurationSeconds(source, variable) {
	const match = source.match(new RegExp(`\\$\\{${escapeRegExp(variable)}:-([0-9]+)s\\}`));
	assert.ok(match, `${variable} must have a seconds default`);
	return Number(match[1]);
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

function assertWorkflowUsesMinimumActionMajor(workflow, action, minimumMajor) {
	const usages = workflowActionUsages(workflow).filter((usage) => usage.action === action);
	assert.ok(usages.length > 0, `${action} must be used by this workflow`);

	for (const { ref, versionComment } of usages) {
		assert.match(ref, /^[a-f0-9]{40}$/, `${action} must be pinned to a full commit SHA`);
		const major = Number(versionComment.match(/^v(\d+)$/)?.[1]);
		assert.ok(major >= minimumMajor, `${action}@${ref} must document v${minimumMajor} or newer`);
	}
}

function workflowActionUsages(workflow) {
	return [...workflow.matchAll(/^\s+-?\s*uses:\s+([^@\s#]+\/[^@\s#]+)@([a-f0-9]{40})\s+#\s+(v\d+)$/gm)].map((match) => ({
		action: match[1],
		index: match.index,
		ref: match[2],
		versionComment: match[3],
	}));
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
	const directories = new Set();
	for (const block of dependabotUpdateBlocks()) {
		if (!new RegExp(`package-ecosystem: ${escapeRegExp(ecosystem)}\\n`).test(block)) {
			continue;
		}
		const singleDirectory = block.match(/^\s+directory:\s+([^\n]+)$/m);
		if (singleDirectory) {
			directories.add(singleDirectory[1].trim());
			continue;
		}
		for (const match of block.matchAll(/^\s+-\s+(\/[^\n]*)$/gm)) {
			directories.add(match[1].trim());
		}
	}
	return directories;
}

function dependabotUpdateBlocks() {
	return dependabot
		.split(/\n(?=\s+- package-ecosystem: )/)
		.filter((block) => block.includes('package-ecosystem:'));
}

function escapeRegExp(value) {
	return value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}
