import assert from 'node:assert/strict';
import { existsSync, readFileSync } from 'node:fs';
import { test } from 'node:test';

const dependabot = readRepositoryFile('.github/dependabot.yml');
const config = parseDependabotConfig(dependabot);

const allowedTopLevelKeys = new Set(['version', 'updates']);
const allowedUpdateKeys = new Set(['package-ecosystem', 'directory', 'directories', 'schedule', 'open-pull-requests-limit', 'target-branch', 'commit-message', 'cooldown', 'rebase-strategy', 'groups']);
const allowedScheduleKeys = new Set(['interval', 'day', 'time', 'timezone']);
const allowedCommitMessageKeys = new Set(['prefix']);
const allowedCooldownKeys = new Set(['default-days']);
const allowedWeeklyDays = new Set(['monday', 'tuesday', 'wednesday', 'thursday', 'friday', 'saturday', 'sunday']);
const supportedEcosystems = new Set(['docker', 'github-actions', 'gomod', 'npm']);

test('dependabot config uses the supported GitHub schema subset', () => {
	assert.equal(config.version, 2);
	assert.deepEqual(new Set(config.topLevelKeys), allowedTopLevelKeys);
	assert.equal(config.updates.length, 5);
	const scheduledMinutes = [];

	for (const update of config.updates) {
		assertAllowedKeys(update.keys, allowedUpdateKeys, updateContext(update));
		assert.ok(supportedEcosystems.has(update.ecosystem), `unsupported package ecosystem ${update.ecosystem}`);
		assertDependabotDirectories(update);
		assert.equal(update.openPullRequestsLimit, 1, `${updateContext(update)} must keep bot PR fan-out low`);
		assert.equal(update.rebaseStrategy, 'disabled', `${updateContext(update)} must not create bot-only rebase commits`);
		assert.equal(update.targetBranch, 'dev-pxhao', `${updateContext(update)} must target the development branch`);
		assertAllowedKeys(update.commitMessage.keys, allowedCommitMessageKeys, `${update.ecosystem} commit-message`);
		assert.match(update.commitMessage.prefix, /^deps\([a-z][a-z0-9-]*\)$/, `${updateContext(update)} must use a scoped bot commit prefix`);

		assertAllowedKeys(update.schedule.keys, allowedScheduleKeys, `${updateContext(update)} schedule`);
		assert.equal(update.schedule.interval, 'weekly');
		assert.ok(allowedWeeklyDays.has(update.schedule.day), `${updateContext(update)} uses invalid schedule day`);
		assert.equal(update.schedule.day, 'monday');
		assertValidDependabotTime(update.schedule.time, updateContext(update));
		scheduledMinutes.push(minutesAfterMidnight(update.schedule.time));
		assert.equal(update.schedule.timezone, 'Asia/Shanghai');

		assertAllowedKeys(update.cooldown.keys, allowedCooldownKeys, `${updateContext(update)} cooldown`);
		assertDependabotCooldownDays(update.cooldown.defaultDays, updateContext(update));
	}
	assertDependabotSchedulesAreStaggered(scheduledMinutes);
});

test('dependabot update entries map to tracked repository manifests', () => {
	for (const update of config.updates) {
		for (const directory of update.directories) {
			const manifest = watchedManifest(update.ecosystem, directory);
			assert.ok(manifest, `${update.ecosystem} ${directory} must map to a supported manifest`);
			if (Array.isArray(manifest)) {
				assert.ok(manifest.some((path) => existsSync(repositoryURL(path))), `${update.ecosystem} ${directory} must have one tracked manifest`);
			} else {
				assert.ok(existsSync(repositoryURL(manifest)), `${update.ecosystem} ${directory} must have ${manifest}`);
			}
		}
	}
});

test('dependabot watches every GitHub Actions workflow file', () => {
	const workflows = watchedManifest('github-actions', '/');
	assert.ok(Array.isArray(workflows), 'github-actions root update must list workflow manifests');
	for (const workflow of workflows) {
		assert.ok(existsSync(repositoryURL(workflow)), `github-actions Dependabot must track ${workflow}`);
	}
});

test('dependabot uses multi-directory updates only for Docker images', () => {
	for (const update of config.updates) {
		if (update.ecosystem === 'docker') {
			assert.ok(update.keys.includes('directories'), 'docker updates should keep one grouped multi-directory block');
			assert.equal(update.directory, undefined, 'docker updates should not split back into one directory block');
			continue;
		}
		assert.ok(update.keys.includes('directory'), `${update.ecosystem} updates should stay single-directory`);
		assert.ok(!update.keys.includes('directories'), `${update.ecosystem} updates should not use multi-directory configuration`);
	}
});

test('dependabot groups every watched ecosystem into one maintenance PR', () => {
	for (const update of config.updates) {
		assert.equal(update.groups.length, 1, `${updateContext(update)} must use exactly one group`);
		const [group] = update.groups;
		assert.deepEqual(group.patterns, ['*']);
		assert.match(group.name, /^[a-z][a-z|_-]*[a-z]$/);
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
	const commitMessage = parseChildScalars(block, 'commit-message');
	const cooldown = parseChildScalars(block, 'cooldown');
	const directories = parseDirectories(block, scalars.get('directory'));
	const groups = parseGroups(block);

	return {
		keys: rootEntries.map(({ key }) => key),
		ecosystem: scalars.get('package-ecosystem'),
		directory: scalars.get('directory'),
		directories,
		openPullRequestsLimit: scalars.get('open-pull-requests-limit'),
		rebaseStrategy: scalars.get('rebase-strategy'),
		targetBranch: scalars.get('target-branch'),
		schedule: {
			keys: schedule.keys,
			interval: schedule.values.get('interval'),
			day: schedule.values.get('day'),
			time: schedule.values.get('time'),
			timezone: schedule.values.get('timezone'),
		},
		commitMessage: {
			keys: commitMessage.keys,
			prefix: commitMessage.values.get('prefix'),
		},
		cooldown: {
			keys: cooldown.keys,
			defaultDays: cooldown.values.get('default-days'),
		},
		groups,
	};
}

function parseDirectories(block, directory) {
	if (directory !== undefined) {
		return [directory];
	}

	const values = [];
	for (const line of sectionLines(block, 'directories')) {
		const match = line.match(/^      - (.+)$/);
		if (match) {
			values.push(parseScalar(match[1]));
		}
	}
	return values;
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
			'github-actions:/': ['.github/workflows/ci.yaml', '.github/workflows/codeql.yaml'],
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

function assertDependabotCooldownDays(days, context) {
	assert.equal(Number.isInteger(days), true, `${context} cooldown default-days must be an integer`);
	assert.ok(days >= 1 && days <= 90, `${context} cooldown default-days must be between 1 and 90`);
}

function assertDependabotDirectories(update) {
	const hasDirectory = update.directory !== undefined;
	const hasDirectories = update.keys.includes('directories');
	assert.notEqual(hasDirectory, hasDirectories, `${update.ecosystem} must define exactly one of directory or directories`);
	assert.ok(update.directories.length > 0, `${update.ecosystem} must watch at least one directory`);
	for (const directory of update.directories) {
		assert.match(directory, /^\//, `${update.ecosystem} directory must be repository-root relative`);
	}
	assert.deepEqual(update.directories, [...new Set(update.directories)], `${update.ecosystem} directories must be unique`);
}

function updateContext(update) {
	return `${update.ecosystem} ${update.directories.join(',')}`;
}

function assertValidDependabotTime(value, context) {
	assert.match(value, /^\d{2}:\d{2}$/, `${context} schedule time must use hh:mm format`);
	const [hour, minute] = value.split(':').map(Number);
	assert.ok(hour >= 0 && hour <= 23, `${context} schedule hour must be 00-23`);
	assert.ok(minute >= 0 && minute <= 59, `${context} schedule minute must be 00-59`);
}

function assertDependabotSchedulesAreStaggered(minutes) {
	const sorted = [...minutes].sort((a, b) => a - b);
	assert.deepEqual(minutes, sorted, 'dependabot schedules must stay in ascending maintenance order');
	assert.deepEqual(minutes, [...new Set(minutes)], 'dependabot schedules must not share the same minute');
	// 多生态更新错峰触发，避免同一时间堆出多条 bot PR 和重复 CI。
	for (let index = 1; index < sorted.length; index++) {
		assert.ok(sorted[index] - sorted[index - 1] >= 10, 'dependabot schedules must be staggered by at least 10 minutes');
	}
}

function minutesAfterMidnight(value) {
	const [hour, minute] = value.split(':').map(Number);
	return hour * 60 + minute;
}

function indentOf(line) {
	return line.match(/^ */)[0].length;
}
