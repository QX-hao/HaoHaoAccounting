import assert from 'node:assert/strict';
import { existsSync, readFileSync } from 'node:fs';
import { test } from 'node:test';

const dependabot = readRepositoryFile('.github/dependabot.yml');
const config = parseDependabotConfig(dependabot);

const allowedTopLevelKeys = new Set(['version', 'updates']);
const allowedUpdateKeys = new Set(['package-ecosystem', 'directory', 'schedule', 'open-pull-requests-limit', 'cooldown', 'groups']);
const allowedScheduleKeys = new Set(['interval', 'day', 'time', 'timezone']);
const allowedCooldownKeys = new Set(['default-days']);
const supportedEcosystems = new Set(['docker', 'github-actions', 'gomod', 'npm']);

test('dependabot config uses the supported GitHub schema subset', () => {
	assert.equal(config.version, 2);
	assert.deepEqual(new Set(config.topLevelKeys), allowedTopLevelKeys);
	assert.equal(config.updates.length, 8);

	for (const update of config.updates) {
		assertAllowedKeys(update.keys, allowedUpdateKeys, `${update.ecosystem} ${update.directory}`);
		assert.ok(supportedEcosystems.has(update.ecosystem), `unsupported package ecosystem ${update.ecosystem}`);
		assert.match(update.directory, /^\//, `${update.ecosystem} directory must be repository-root relative`);
		assert.equal(update.openPullRequestsLimit, 1, `${update.ecosystem} ${update.directory} must keep bot PR fan-out low`);

		assertAllowedKeys(update.schedule.keys, allowedScheduleKeys, `${update.ecosystem} schedule`);
		assert.equal(update.schedule.interval, 'weekly');
		assert.equal(update.schedule.day, 'monday');
		assert.match(update.schedule.time, /^\d{2}:\d{2}$/);
		assert.equal(update.schedule.timezone, 'Asia/Shanghai');

		assertAllowedKeys(update.cooldown.keys, allowedCooldownKeys, `${update.ecosystem} cooldown`);
		assert.equal(update.cooldown.defaultDays, 7);
	}
});

test('dependabot update entries map to tracked repository manifests', () => {
	for (const update of config.updates) {
		const manifest = watchedManifest(update.ecosystem, update.directory);
		assert.ok(manifest, `${update.ecosystem} ${update.directory} must map to a supported manifest`);
		if (Array.isArray(manifest)) {
			assert.ok(manifest.some((path) => existsSync(repositoryURL(path))), `${update.ecosystem} ${update.directory} must have one tracked manifest`);
		} else {
			assert.ok(existsSync(repositoryURL(manifest)), `${update.ecosystem} ${update.directory} must have ${manifest}`);
		}
	}
});

test('dependabot groups every watched ecosystem into one maintenance PR', () => {
	for (const update of config.updates) {
		assert.equal(update.groups.length, 1, `${update.ecosystem} ${update.directory} must use exactly one group`);
		const [group] = update.groups;
		assert.deepEqual(group.patterns, ['*']);
		assert.match(group.name, /^[a-z0-9_-]+$/);
	}
});

function readRepositoryFile(path) {
	return readFileSync(repositoryURL(path), 'utf8');
}

function repositoryURL(path) {
	return new URL(`../../../${path}`, import.meta.url);
}

// 这里不用额外 YAML 依赖，只解析本仓库 dependabot.yml 使用到的固定结构。
function parseDependabotConfig(source) {
	const topLevelKeys = [...source.matchAll(/^([a-z][a-z-]*):/gm)].map((match) => match[1]);
	const version = Number(source.match(/^version:\s+(\d+)$/m)?.[1]);
	const updates = dependabotUpdateBlocks(source).map(parseUpdateBlock);
	return { topLevelKeys, version, updates };
}

function parseUpdateBlock(block) {
	const lines = meaningfulLines(block);
	const first = lines[0].match(/^  - ([a-z][a-z-]*):\s+(.+)$/);
	assert.ok(first, `invalid dependabot update block:\n${block}`);

	const rootEntries = [{ key: first[1], value: parseScalar(first[2]) }];
	for (const line of lines.slice(1)) {
		const match = line.match(/^    ([a-z][a-z-]*):(.*)$/);
		if (match) {
			rootEntries.push({ key: match[1], value: parseScalar(match[2]) });
		}
	}

	const scalars = new Map(rootEntries.map(({ key, value }) => [key, value]));
	const schedule = parseChildScalars(block, 'schedule');
	const cooldown = parseChildScalars(block, 'cooldown');
	const groups = parseGroups(block);

	return {
		keys: rootEntries.map(({ key }) => key),
		ecosystem: scalars.get('package-ecosystem'),
		directory: scalars.get('directory'),
		openPullRequestsLimit: scalars.get('open-pull-requests-limit'),
		schedule: {
			keys: schedule.keys,
			interval: schedule.values.get('interval'),
			day: schedule.values.get('day'),
			time: schedule.values.get('time'),
			timezone: schedule.values.get('timezone'),
		},
		cooldown: {
			keys: cooldown.keys,
			defaultDays: cooldown.values.get('default-days'),
		},
		groups,
	};
}

function parseChildScalars(block, key) {
	const entries = [];
	for (const line of sectionLines(block, key)) {
		const match = line.match(/^      ([a-z][a-z-]*):\s+(.+)$/);
		if (match) {
			entries.push({ key: match[1], value: parseScalar(match[2]) });
		}
	}
	return {
		keys: entries.map(({ key }) => key),
		values: new Map(entries.map(({ key, value }) => [key, value])),
	};
}

function parseGroups(block) {
	const groups = [];
	let currentGroup = null;
	for (const line of sectionLines(block, 'groups')) {
		const groupMatch = line.match(/^      ([a-z0-9_-]+):$/);
		if (groupMatch) {
			currentGroup = { name: groupMatch[1], keys: [], patterns: [] };
			groups.push(currentGroup);
			continue;
		}
		const keyMatch = line.match(/^        ([a-z][a-z-]*):$/);
		if (keyMatch && currentGroup) {
			currentGroup.keys.push(keyMatch[1]);
			continue;
		}
		const itemMatch = line.match(/^          - (.+)$/);
		if (itemMatch && currentGroup) {
			currentGroup.patterns.push(parseScalar(itemMatch[1]));
		}
	}

	for (const group of groups) {
		assertAllowedKeys(group.keys, new Set(['patterns']), `group ${group.name}`);
	}
	return groups;
}

function sectionLines(block, key) {
	const lines = block.split(/\r?\n/);
	const headerIndex = lines.findIndex((line) => line === `    ${key}:`);
	assert.notEqual(headerIndex, -1, `missing ${key} section in:\n${block}`);

	const result = [];
	for (const line of lines.slice(headerIndex + 1)) {
		if (!line.trim() || line.trim().startsWith('#')) {
			continue;
		}
		if (indentOf(line) <= 4) {
			break;
		}
		result.push(line);
	}
	return result;
}

function meaningfulLines(block) {
	return block.split(/\r?\n/).filter((line) => line.trim() && !line.trim().startsWith('#'));
}

function dependabotUpdateBlocks(source) {
	return source
		.split(/\n(?=\s+- package-ecosystem: )/)
		.filter((block) => block.includes('package-ecosystem:'));
}

function parseScalar(raw) {
	const value = raw.trim();
	if (value === '') {
		return undefined;
	}
	if (/^\d+$/.test(value)) {
		return Number(value);
	}
	const quoted = value.match(/^"(.*)"$/);
	return quoted ? quoted[1] : value;
}

function watchedManifest(ecosystem, directory) {
	const manifests = {
		'docker:/': ['Dockerfile.postgres', 'Dockerfile.redis', 'Dockerfile.mysql'],
		'docker:/backend': 'backend/Dockerfile',
		'docker:/mobile': 'mobile/Dockerfile',
		'docker:/web': 'web/Dockerfile',
		'github-actions:/': '.github/workflows/ci.yaml',
		'gomod:/backend': 'backend/go.sum',
		'npm:/mobile': 'mobile/package-lock.json',
		'npm:/web': 'web/package-lock.json',
	};
	return manifests[`${ecosystem}:${directory}`];
}

function assertAllowedKeys(keys, allowed, context) {
	for (const key of keys) {
		assert.ok(allowed.has(key), `${context} contains unsupported key ${key}`);
	}
}

function indentOf(line) {
	return line.match(/^ */)[0].length;
}
