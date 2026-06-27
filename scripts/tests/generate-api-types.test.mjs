import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { test } from 'node:test';

const generator = readFileSync(new URL('../generate-api-types.mjs', import.meta.url), 'utf8');
const openapi = readFileSync(new URL('../../backend/api/openapi.yaml', import.meta.url), 'utf8');
const healthOpenapi = readFileSync(new URL('../../backend/api/health-openapi.yaml', import.meta.url), 'utf8');
const generatedClient = readFileSync(new URL('../../web/shared/api/generated-client.ts', import.meta.url), 'utf8');
const generatedTypes = readFileSync(new URL('../../web/shared/types/api.ts', import.meta.url), 'utf8');
const webApiClient = readFileSync(new URL('../../web/shared/api/client.ts', import.meta.url), 'utf8');
const webApiReadme = readFileSync(new URL('../../web/shared/api/README.md', import.meta.url), 'utf8');
const webConfig = readFileSync(new URL('../../web/lib/config.ts', import.meta.url), 'utf8');
const webDataioApi = readFileSync(new URL('../../web/features/dataio/api.ts', import.meta.url), 'utf8');
const webAppShell = readFileSync(new URL('../../web/components/AppShell.tsx', import.meta.url), 'utf8');
const webEnvExample = readFileSync(new URL('../../web/.env.example', import.meta.url), 'utf8');
const mobileApiClient = readFileSync(new URL('../../mobile/src/shared/api/client.ts', import.meta.url), 'utf8');
const mobileApiReadme = readFileSync(new URL('../../mobile/src/shared/api/README.md', import.meta.url), 'utf8');
const mobileConfig = readFileSync(new URL('../../mobile/src/shared/config.ts', import.meta.url), 'utf8');
const mobileDataioApi = readFileSync(new URL('../../mobile/src/features/dataio/api.ts', import.meta.url), 'utf8');
const mobileUseSession = readFileSync(new URL('../../mobile/src/features/auth/useSession.ts', import.meta.url), 'utf8');
const mobileEnvExample = readFileSync(new URL('../../mobile/.env.example', import.meta.url), 'utf8');
const goHTTPUtilResponses = readFileSync(new URL('../../backend/internal/httputil/response.go', import.meta.url), 'utf8');
const goRequestDTOs = new Map([
	['auth', readFileSync(new URL('../../backend/internal/modules/auth/dto.go', import.meta.url), 'utf8')],
	['accounts', readFileSync(new URL('../../backend/internal/modules/accounts/dto.go', import.meta.url), 'utf8')],
	['budgets', readFileSync(new URL('../../backend/internal/modules/budgets/dto.go', import.meta.url), 'utf8')],
	['categories', readFileSync(new URL('../../backend/internal/modules/categories/dto.go', import.meta.url), 'utf8')],
	['transactions', readFileSync(new URL('../../backend/internal/modules/transactions/dto.go', import.meta.url), 'utf8')],
	['ai', readFileSync(new URL('../../backend/internal/modules/ai/dto.go', import.meta.url), 'utf8')],
	['dataio', readFileSync(new URL('../../backend/internal/modules/dataio/dto.go', import.meta.url), 'utf8')],
]);

function openapiSchema(schemaName) {
	const schemasStart = openapi.indexOf('  schemas:');
	const schemas = openapi.slice(schemasStart);
	const marker = `    ${schemaName}:\n`;
	const start = schemas.indexOf(marker);
	assert.notEqual(start, -1, `${schemaName} schema not found`);
	const rest = schemas.slice(start + marker.length);
	const next = rest.search(/^    [A-Za-z][A-Za-z0-9]*:\n/m);
	return marker + rest.slice(0, next === -1 ? rest.length : next);
}

function openapiTopLevelBlock(key) {
	const marker = `${key}:\n`;
	const start = openapi.indexOf(marker);
	assert.notEqual(start, -1, `${key} block not found`);
	const rest = openapi.slice(start + marker.length);
	const next = rest.search(/^[A-Za-z][A-Za-z0-9_-]*:\n/m);
	return marker + rest.slice(0, next === -1 ? rest.length : next);
}

function openapiPathBlock(apiPath) {
	const marker = `  ${apiPath}:\n`;
	const start = openapi.indexOf(marker);
	assert.notEqual(start, -1, `${apiPath} path not found`);
	const rest = openapi.slice(start + marker.length);
	const next = rest.search(/^  \//m);
	return marker + rest.slice(0, next === -1 ? rest.length : next);
}

function openAPIOperationBlocks() {
	const paths = openapi.slice(openapi.indexOf('paths:'), openapi.indexOf('components:'));
	const matches = [...paths.matchAll(/^    (get|post|put|delete|patch|options|head|trace):$/gm)];
	return matches.map((match, index) => {
		const start = match.index;
		const end = matches[index + 1]?.index ?? paths.length;
		return paths.slice(start, end);
	});
}

function responseBlock(source, status) {
	const marker = `        '${status}':\n`;
	const start = source.indexOf(marker);
	assert.notEqual(start, -1, `${status} response not found`);
	const rest = source.slice(start + marker.length);
	const next = rest.search(/^        '[0-9]{3}':/m);
	return marker + rest.slice(0, next === -1 ? rest.length : next);
}

function componentResponseBlock(responseName) {
	const responsesStart = openapi.indexOf('  responses:');
	const schemasStart = openapi.indexOf('  schemas:');
	const responses = openapi.slice(responsesStart, schemasStart);
	const marker = `    ${responseName}:\n`;
	const start = responses.indexOf(marker);
	assert.notEqual(start, -1, `${responseName} response not found`);
	const rest = responses.slice(start + marker.length);
	const next = rest.search(/^    [A-Za-z][A-Za-z0-9]*:\n/m);
	return marker + rest.slice(0, next === -1 ? rest.length : next);
}

test('OpenAPI YAML files disallow duplicate mapping keys', () => {
	for (const [name, source] of [
		['backend/api/openapi.yaml', openapi],
		['backend/api/health-openapi.yaml', healthOpenapi],
	]) {
		const duplicates = duplicateYamlMappingKeys(source);
		assert.deepEqual(duplicates, [], `${name} contains duplicate mapping keys`);
	}
});

test('YAML duplicate key scanner treats sequence item maps independently', () => {
	assert.deepEqual(duplicateYamlMappingKeys('items:\n  - name: one\n    description: first\n  - name: two\n    description: second\n'), []);
	assert.deepEqual(duplicateYamlMappingKeys('root:\n  value: one\n  value: two\n'), ['value at line 3']);
});

test('generated clients preserve explicit numeric zero query params', () => {
	assert.match(generator, /if \(value === undefined \|\| value === null \|\| value === ''\) return;/);
	assert.doesNotMatch(generator, /typeof value === 'number' && value === 0/);
	assert.doesNotMatch(generatedClient, /typeof value === 'number' && value === 0/);
});

test('generated clients allow omitting fully optional query params', () => {
	assert.match(generator, /function methodArgs\(args, optionalParams\)/);
	assert.match(generatedClient, /getCategories: \(params: \{\n\s+type\?: TransactionType;\n\} = \{\}\) => \{/);
	assert.match(generatedClient, /setQueryParam\(search, 'type', params\?\.type\);/);
	assert.doesNotMatch(generatedClient, /deleteAccountsById: \(params: \{\n\s+id: number \| string;\n\s+\} = \{\}\) => \{/);
});

test('generated clients URL-encode path parameters', () => {
	assert.match(generator, /encodeURIComponent\(String\(params\.\$\{name\}\)\)/);
	assert.match(generatedClient, /path = path\.replace\('\{id\}', encodeURIComponent\(String\(params\.id\)\)\);/);
	assert.doesNotMatch(generatedClient, /path = path\.replace\('\{id\}', String\(params\.id\)\);/);
});

test('generated types preserve property enum values', () => {
	assert.match(generator, /propertyEnumValues/);
});

test('generated types parse quoted empty string enum values', () => {
	assert.match(generator, /parseInlineYAMLScalar/);
	assert.doesNotMatch(generatedTypes, /"''"/);
	assert.match(generatedTypes, /type: "income" \| "expense" \| "";/);
});

test('generated error codes stay available to API clients', () => {
	assert.match(generatedTypes, /request_timeout/);
	assert.match(generatedTypes, /client_closed_request/);
	assert.match(generatedTypes, /not_acceptable/);
	assert.match(webApiClient, /ErrorResponse\['code'\]/);
	assert.match(webApiClient, /Retry-After/);
	assert.match(webApiClient, /rateLimitLimit/);
	assert.match(webApiClient, /rateLimitRemaining/);
	assert.match(webApiClient, /rateLimitResetSeconds/);
	assert.match(webApiClient, /authenticateChallenge/);
	assert.match(webApiClient, /WWW-Authenticate/);
	assert.match(webApiClient, /network_error/);
	assert.match(webApiClient, /function networkError\(err: unknown, timedOut = false\)/);
	assert.match(webApiClient, /super\(message, \{ cause \}\)/);
	assert.match(webApiClient, /new ApiError\(message, 0, 'network_error', '', null, null, null, null, '', err\)/);
	assert.match(mobileApiClient, /ErrorResponse\['code'\]/);
	assert.match(mobileApiClient, /Retry-After/);
	assert.match(mobileApiClient, /rateLimitLimit/);
	assert.match(mobileApiClient, /rateLimitRemaining/);
	assert.match(mobileApiClient, /rateLimitResetSeconds/);
	assert.match(mobileApiClient, /authenticateChallenge/);
	assert.match(mobileApiClient, /WWW-Authenticate/);
	assert.match(mobileApiClient, /network_error/);
	assert.match(mobileApiClient, /function networkError\(err: unknown, timedOut = false\)/);
	assert.match(mobileApiClient, /super\(message, \{ cause \}\)/);
	assert.match(mobileApiClient, /new ApiError\(message, 0, 'network_error', '', null, null, null, null, '', err\)/);
});

test('OpenAPI documents CORS allowlist rejection behavior', () => {
	const info = openapiTopLevelBlock('info');
	assert.match(info, /CORS_ALLOW_ORIGINS/);
	assert.match(info, /rejected by CORS middleware with 403/);
	assert.match(info, /do not include Access-Control-Allow-Origin/);
	assert.match(generator, /OpenAPI info\.description must document CORS allowlist rejection behavior/);
});

test('OpenAPI servers describe the local API base URL', () => {
	const servers = openapiTopLevelBlock('servers');
	assert.match(servers, /url: http:\/\/localhost:8080\/api\/v1/);
	assert.match(servers, /description: Local development API server\./);
	assert.match(generator, /validateOpenAPIServers\(source\)/);
	assert.match(generator, /OpenAPI servers must include the local development API base URL/);
	assert.match(generator, /OpenAPI local server must include a human-readable description/);
});

test('OpenAPI top-level tags describe operation groups', () => {
	const tags = openapiTopLevelBlock('tags');
	for (const tag of ['auth', 'accounts', 'budgets', 'categories', 'transactions', 'ai', 'reports', 'dataio']) {
		assert.match(tags, new RegExp(`- name: ${tag}\\n\\s+description: \\S.+`), `${tag} tag is missing description`);
	}
	assert.match(generator, /validateOpenAPITags\(source\)/);
	assert.match(generator, /OpenAPI top-level tags must declare API groups/);
	assert.match(generator, /OpenAPI tag \$\{name\} is missing description/);
	assert.match(generator, /OpenAPI tag \$\{name\} is declared more than once/);
	assert.match(generator, /uses undeclared tag/);
});

test('generated clients use OpenAPI tags as module groups', () => {
	assert.match(generator, /must declare exactly one tag for generated client grouping/);
	assert.match(generator, /const group = endpoint\.tags\[0\]\.replace/);
	assert.doesNotMatch(generator, /const first = endpoint\.path\.split/);
	assert.match(generatedClient, /auth: \{[\s\S]+getMe:/);
	assert.doesNotMatch(generatedClient, /me: \{/);
});

test('API clients send request ids for log correlation', () => {
	for (const source of [webApiClient, mobileApiClient]) {
		assert.match(source, /function ensureRequestId\(headers: Headers\)/);
		assert.match(source, /headers\.has\('X-Request-ID'\)/);
		assert.match(source, /headers\.set\('X-Request-ID', newRequestId\(\)\)/);
		assert.match(source, /function withRequestId\(headers: HeadersInit\): Headers/);
		assert.match(source, /function newRequestId\(\)/);
	}
	assert.match(webApiClient, /return `web-\$\{Date\.now\(\)\.toString\(36\)\}-\$\{Math\.random\(\)\.toString\(36\)\.slice\(2, 10\)\}`;/);
	assert.match(mobileApiClient, /return `mobile-\$\{Date\.now\(\)\.toString\(36\)\}-\$\{Math\.random\(\)\.toString\(36\)\.slice\(2, 10\)\}`;/);
});

test('API clients send explicit Accept headers for negotiated responses', () => {
	assert.match(webApiClient, /headers\.set\('Accept', headers\.get\('Accept'\) \|\| 'application\/json'\)/);
	assert.match(webApiClient, /headers\.set\('Accept', 'application\/json'\)/);
	assert.match(webDataioApi, /return 'text\/csv'/);
	assert.match(webDataioApi, /application\/vnd\.openxmlformats-officedocument\.spreadsheetml\.sheet/);
	assert.match(mobileApiClient, /headers\.set\('Accept', headers\.get\('Accept'\) \|\| 'application\/json'\)/);
	assert.match(mobileDataioApi, /downloadText\('\/io\/export\?format=csv', 'text\/csv'\)/);
});

test('public API base env values are validated before client requests', () => {
	for (const source of [webConfig, mobileConfig]) {
		assert.match(source, /function normalizePublicApiBase\(value: string \| undefined, fallback: string, name: string\)/);
		assert.match(source, /new URL\(raw\)/);
		assert.match(source, /absolute http\(s\) URL/);
		assert.match(source, /url\.protocol !== 'http:' && url\.protocol !== 'https:'/);
		assert.match(source, /url\.username \|\| url\.password \|\| url\.search \|\| url\.hash/);
		assert.match(source, /replace\(\/\\\/\+\$\/, ''\)/);
	}
	assert.match(webConfig, /process\.env\.NEXT_PUBLIC_API_BASE/);
	assert.match(webConfig, /DEFAULT_API_BASE = 'http:\/\/localhost:8080\/api\/v1'/);
	assert.match(mobileConfig, /process\.env\.EXPO_PUBLIC_API_BASE/);
	assert.match(mobileConfig, /DEFAULT_API_BASE = 'http:\/\/127\.0\.0\.1:8080\/api\/v1'/);
	assert.match(mobileApiClient, /import \{ API_BASE \} from '\.\.\/config';/);
	assert.doesNotMatch(mobileApiClient, /process\.env\.EXPO_PUBLIC_API_BASE/);
});

test('public API base env examples document client exposure and URL shape', () => {
	for (const source of [webEnvExample, mobileEnvExample]) {
		assert.match(source, /Public client variable/);
		assert.match(source, /absolute http\(s\) URL/);
		assert.match(source, /without a trailing slash/);
		assert.match(source, /do not put secrets here/);
	}
});

test('OpenAPI response components document no-store API cache headers', () => {
	assert.match(generator, /validateNoStoreHeaders/);
	assert.match(generator, /components\.headers\.\$\{componentName\} is missing \$\{expectedValue\}/);
	assert.match(generator, /components\.responses\.\$\{responseName\} is missing \$\{headerName\} no-store header/);
	assert.match(generator, /validateSuccessResponseCacheHeaders/);
	assert.match(generator, /response is missing \$\{header\} no-store header/);
	assert.match(openapi, /CacheControl:[\s\S]+enum: \[no-store\]/);
	assert.match(openapi, /Pragma:[\s\S]+enum: \[no-cache\]/);
	assert.match(openapi, /Expires:[\s\S]+enum: \['0'\]/);
	assert.match(openapi, /CacheControl:[\s\S]+example: no-store/);
	assert.match(openapi, /Pragma:[\s\S]+example: no-cache/);
	assert.match(openapi, /Expires:[\s\S]+example: '0'/);
	assert.match(openapi, /'200':\n\s+description: Login success\n\s+headers:\n\s+Cache-Control:/);
});

test('API clients only default JSON Content-Type when a non-FormData body is present', () => {
	const expected = /if \(init\.body !== undefined && init\.body !== null && !\(init\.body instanceof FormData\)\) \{[\s\S]*?headers\.set\('Content-Type', headers\.get\('Content-Type'\) \|\| 'application\/json'\);/;
	assert.match(webApiClient, expected);
	assert.match(mobileApiClient, expected);
});

test('mobile CSV export uses shared text download error handling', () => {
	assert.match(mobileApiClient, /export async function download\(path: string, accept = '\*\/\*'\): Promise<DownloadResult>/);
	assert.match(mobileApiClient, /export async function downloadText/);
	assert.match(mobileApiClient, /filenameFromDisposition\(resp\.headers\.get\('Content-Disposition'\)\) \|\| 'download'/);
	assert.match(mobileApiClient, /const data = await parseErrorBody\(resp\);/);
	assert.match(mobileDataioApi, /return downloadText\('\/io\/export\?format=csv', 'text\/csv'\)/);
	assert.match(mobileDataioApi, /export function exportTransactionsFile\(format: ExportFormat\)/);
	assert.match(mobileDataioApi, /application\/vnd\.openxmlformats-officedocument\.spreadsheetml\.sheet/);
	assert.doesNotMatch(mobileDataioApi, /fetch\(`\$\{API_BASE\}\/io\/export/);
});

test('API clients fall back to a stable download filename', () => {
	assert.match(webApiClient, /filenameFromDisposition\(resp\.headers\.get\('Content-Disposition'\)\) \|\| 'download'/);
	assert.match(mobileApiClient, /filenameFromDisposition\(resp\.headers\.get\('Content-Disposition'\)\) \|\| 'download'/);
});

test('API clients parse non-JSON error bodies through the shared error parser', () => {
	const expected = /if \(!resp\.ok\) \{[\s\S]+const data = await parseErrorBody\(resp\);[\s\S]+throw apiError\(resp, data\);[\s\S]+\}\s+const data = await resp\.json\(\)\.catch\(\(\) => \(\{\}\)\);/;
	assert.match(webApiClient, expected);
	assert.match(mobileApiClient, expected);
});

test('API clients parse structured JSON error media types', () => {
	for (const source of [webApiClient, mobileApiClient]) {
		assert.match(source, /function isJSONContentType\(contentType: string\)/);
		assert.match(source, /mediaType === 'application\/json'/);
		assert.match(source, /mediaType\.startsWith\('application\/'\) && mediaType\.endsWith\('\+json'\)/);
		assert.match(source, /if \(isJSONContentType\(contentType\)\)/);
	}
});

test('API clients ignore non-object JSON error bodies', () => {
	for (const source of [webApiClient, mobileApiClient]) {
		assert.match(source, /function isObjectRecord\(value: unknown\): value is Record<string, unknown>/);
		assert.match(source, /const data = await resp\.json\(\)\.catch\(\(\) => \(\{\}\)\);/);
		assert.match(source, /return isObjectRecord\(data\) \? data as ApiErrorBody : \{\};/);
		assert.match(source, /!Array\.isArray\(value\)/);
	}
});

test('API clients route logout through shared network error handling', () => {
	assert.match(webApiClient, /await fetchAPI\('\/auth\/logout', \{/);
	assert.match(mobileApiClient, /await fetchAPI\('\/auth\/logout', \{/);
	assert.match(webApiClient, /export async function logout\(\): Promise<void> \{[\s\S]+clearToken\(\);[\s\S]+await fetchAPI\('\/auth\/logout'/);
	assert.match(mobileApiClient, /export async function logout\(\) \{[\s\S]+await clearToken\(\);[\s\S]+await fetchAPI\('\/auth\/logout'/);
	assert.doesNotMatch(webAppShell, /clearToken/);
	assert.doesNotMatch(mobileUseSession.match(/async function signOut\(\) \{[\s\S]+?\n  \}/)?.[0] || '', /clearToken/);
	assert.match(mobileApiClient, /const sessionInvalidationListeners = new Set<\(\) => void>\(\);/);
	assert.match(mobileApiClient, /export function onSessionInvalidated\(listener: \(\) => void\)/);
	assert.match(mobileApiClient, /function notifySessionInvalidated\(\)/);
	assert.match(mobileApiClient, /resp\.status === 401[\s\S]+await clearToken\(\);[\s\S]+notifySessionInvalidated\(\);/);
	assert.match(mobileUseSession, /onSessionInvalidated\(\(\) => \{[\s\S]+setAuthed\(false\);[\s\S]+\}\)/);
	assert.doesNotMatch(webApiClient, /fetch\(`\$\{API_BASE\}\/auth\/logout/);
	assert.doesNotMatch(mobileApiClient, /fetch\(`\$\{API_BASE\}\/auth\/logout/);
});

test('API clients bound fetch calls with timeout and preserve caller abort signals', () => {
	for (const source of [webApiClient, mobileApiClient]) {
		assert.match(source, /const API_REQUEST_TIMEOUT_MS = 30_000;/);
		assert.match(source, /function requestSignal\(callerSignal\?: AbortSignal \| null\)/);
		assert.match(source, /const controller = new AbortController\(\);/);
		assert.match(source, /let timedOut = false;/);
		assert.match(source, /setTimeout\(\(\) => \{\n\s+timedOut = true;\n\s+controller\.abort\(\);\n\s+\}, API_REQUEST_TIMEOUT_MS\)/);
		assert.match(source, /callerSignal\.addEventListener\('abort', abort, \{ once: true \}\)/);
		assert.match(source, /return await fetch\(`\$\{API_BASE\}\$\{path\}`, \{ \.\.\.init, credentials: 'omit', signal: request\.signal \}\);/);
		assert.match(source, /finally \{\n\s+request\.cleanup\(\);\n\s+\}/);
	}
});

test('API clients expose local request timeouts as structured API errors', () => {
	assert.match(webApiClient, /throw networkError\(err, request\.timedOut\(\)\);/);
	assert.match(mobileApiClient, /throw networkError\(err, request\.timedOut\(\)\);/);
	for (const source of [webApiClient, mobileApiClient]) {
		assert.match(source, /timedOut: \(\) => timedOut/);
		assert.match(source, /function networkError\(err: unknown, timedOut = false\)/);
		assert.match(source, /if \(timedOut\) \{[\s\S]+new ApiError\([^,]+, 0, 'request_timeout'/);
		assert.match(source, /new ApiError\(message, 0, 'network_error'/);
	}
});

test('API client READMEs document shared runtime contracts', () => {
	for (const source of [webApiReadme, mobileApiReadme]) {
		for (const text of [
			'`fetchAPI`',
			'`X-Request-ID`',
			'`Content-Type: application/json` only when the body is not `FormData`',
			'30 second `AbortController` timeout',
			'`RequestInit.signal`',
			'`credentials: omit`',
			'`application/*+json`',
			'`request_timeout`',
			'`Retry-After`',
			'`RateLimit-*`',
			'`WWW-Authenticate`',
			'`Content-Disposition` `filename*`',
		]) {
			assert.match(source, new RegExp(escapeRegExp(text)));
		}
	}
});

test('API clients support Retry-After delay seconds and HTTP-date values', () => {
	const expected = /const retryAt = Date\.parse\(value\);[\s\S]+return Math\.max\(0, Math\.ceil\(\(retryAt - Date\.now\(\)\) \/ 1000\)\);/;
	assert.match(webApiClient, expected);
	assert.match(mobileApiClient, expected);
	for (const source of [webApiClient, mobileApiClient]) {
		assert.match(source, /\/\^\\d\+\$\/\.test\(value\)/);
		assert.match(source, /Number\.isSafeInteger\(seconds\)/);
		assert.match(source, /\/\^\[\+-\]\?\\d\/\.test\(value\)/);
		assert.doesNotMatch(source, /Math\.ceil\(seconds\)/);
	}
});

test('API clients expose rate limit response headers on errors', () => {
	for (const source of [webApiClient, mobileApiClient]) {
		assert.match(source, /function nonNegativeIntegerHeader\(resp: Response, name: string\): number \| null/);
		assert.match(source, /Number\.isInteger\(parsed\) \|\| parsed < 0/);
		assert.match(source, /nonNegativeIntegerHeader\(resp, 'RateLimit-Limit'\)/);
		assert.match(source, /nonNegativeIntegerHeader\(resp, 'RateLimit-Remaining'\)/);
		assert.match(source, /nonNegativeIntegerHeader\(resp, 'RateLimit-Reset'\)/);
	}
});

test('API clients only trust error-body status values in the error range', () => {
	for (const source of [webApiClient, mobileApiClient]) {
		assert.match(source, /status >= 400 && status <= 599/);
		assert.doesNotMatch(source, /status >= 100 && status <= 599/);
	}
});

test('API clients expose successful response headers with parsed data', () => {
	for (const source of [webApiClient, mobileApiClient]) {
		assert.match(source, /export type ApiResponse<T> = \{\n\s+data: T;\n\s+headers: Headers;\n\};/);
		assert.match(source, /export async function request<T>\(path: string, init: RequestInit = \{\}\): Promise<T> \{\n\s+const response = await requestWithResponse<T>\(path, init\);\n\s+return response\.data;\n\}/);
		assert.match(source, /return \{ data: data as T, headers: resp\.headers \};/);
	}
	assert.match(webApiClient, /export async function requestWithResponse<T>\(path: string, init: RequestInit = \{\}\): Promise<ApiResponse<T>>/);
	assert.match(webApiClient, /export async function uploadWithResponse<T>\(path: string, formData: FormData\): Promise<ApiResponse<T>>/);
	assert.match(mobileApiClient, /export async function requestWithResponse<T>\(path: string, init: RequestInit = \{\}\): Promise<ApiResponse<T>>/);
	assert.match(mobileApiClient, /export function uploadWithResponse<T>\(path: string, formData: FormData\)/);
	assert.match(webApiReadme, /successful response headers such as `Location` and `Link`/);
	assert.match(mobileApiReadme, /successful response headers such as `Location` and `Link`/);
});

test('generator rejects duplicate OpenAPI operationId values', () => {
	assert.match(generator, /operationIds\.has\(operationId\)/);
	assert.match(generator, /is used by both/);
});

test('generator requires operation ids to match generated client method names', () => {
	assert.match(generator, /const expectedOperationId = endpointMethodName\(method, apiPath\);/);
	assert.match(generator, /operationId must be \$\{expectedOperationId\}/);

	for (const operation of openapiOperations(openapi)) {
		assert.equal(
			operation.operationId,
			expectedOperationId(operation.method, operation.path),
			`${operation.method.toUpperCase()} ${operation.path} operationId drifted`,
		);
	}
});

test('generator scans every OpenAPI path item HTTP operation instead of silently ignoring it', () => {
	assert.match(generator, /const openAPIHTTPMethods = \['get', 'put', 'post', 'delete', 'options', 'head', 'patch', 'trace'\];/);
	assert.match(generator, /function openAPIMethodMatches\(pathBlock\)/);
	assert.match(generator, /function openAPIMethodLineRegexp\(\)/);
	assert.doesNotMatch(generator, /\(get\|post\|put\|delete\)/);
});

test('generator requires human-readable operation summaries', () => {
	assert.match(generator, /operationSummary\(methodBlock\)/);
	assert.match(generator, /is missing summary/);
	for (const operationId of openapi.matchAll(/operationId:\s+([A-Za-z][A-Za-z0-9]*)/g)) {
		const blockStart = openapi.lastIndexOf('\n    ', operationId.index);
		const nextOperation = openapi.indexOf('\n      operationId:', operationId.index + 1);
		const blockEnd = nextOperation === -1 ? openapi.indexOf('\ncomponents:', operationId.index) : nextOperation;
		const block = openapi.slice(blockStart, blockEnd);
		assert.match(block, /^\s+summary:\s+\S.+$/m, `${operationId[1]} is missing summary`);
	}
});

test('generator requires human-readable operation descriptions', () => {
	assert.match(generator, /operationDescription\(methodBlock\)/);
	assert.match(generator, /is missing description/);
	for (const operationId of openapi.matchAll(/operationId:\s+([A-Za-z][A-Za-z0-9]*)/g)) {
		const blockStart = openapi.lastIndexOf('\n    ', operationId.index);
		const nextOperation = openapi.indexOf('\n      operationId:', operationId.index + 1);
		const blockEnd = nextOperation === -1 ? openapi.indexOf('\ncomponents:', operationId.index) : nextOperation;
		const block = openapi.slice(blockStart, blockEnd);
		assert.match(block, /^\s+description:\s+\S.+$/m, `${operationId[1]} is missing description`);
	}
});

test('OpenAPI core schemas include readable examples', () => {
	for (const schemaName of ['LoginRequest', 'LoginResponse', 'AccountRequest', 'TransactionRequest', 'ErrorResponse']) {
		const schema = openapiSchema(schemaName);
		assert.match(schema, /^\s+example:\n\s+\S/m, `${schemaName} is missing example`);
	}
});

test('OpenAPI key auth responses include media type examples', () => {
	const login = openapiPathBlock('/auth/login');
	const loginSuccess = responseBlock(login, '200');
	assert.match(loginSuccess, /application\/json:[\s\S]+example:[\s\S]+token:/);

	const unauthorized = componentResponseBlock('Unauthorized');
	assert.match(unauthorized, /application\/json:[\s\S]+example:[\s\S]+code: unauthorized/);
});

test('generator requires auth schema field direction metadata', () => {
	assert.match(generator, /validateAuthSchemaFieldDirection/);
	assert.match(generator, /LoginRequest\.password is missing writeOnly: true/);
	assert.match(generator, /LoginRequest\.password is missing password format/);
	assert.match(generator, /LoginResponse\.token is missing readOnly: true/);
	assert.match(openapiSchema('LoginRequest'), /password:\n\s+type: string\n\s+minLength: 1\n\s+format: password\n\s+writeOnly: true/);
	assert.match(openapiSchema('LoginResponse'), /token:\n\s+type: string\n\s+readOnly: true/);
});

test('OpenAPI shared error responses include structured examples', () => {
	assert.match(generator, /validateErrorResponseExamples\(openapi\)/);
	assert.match(generator, /components\.responses\.\$\{responseName\} example is missing \$\{field\}/);
	assert.match(generator, /components\.responses\.\$\{responseName\} example must use status \$\{expected\.status\} and code \$\{expected\.code\}/);
	const contracts = {
		BadRequest: [400, 'bad_request'],
		InvalidRequest: [400, 'invalid_request'],
		Unauthorized: [401, 'unauthorized'],
		Forbidden: [403, 'forbidden'],
		NotFound: [404, 'not_found'],
		MethodNotAllowed: [405, 'method_not_allowed'],
		RateLimited: [429, 'rate_limited'],
		PayloadTooLarge: [413, 'payload_too_large'],
		UnsupportedMediaType: [415, 'unsupported_media_type'],
		NotAcceptable: [406, 'not_acceptable'],
		InternalError: [500, 'internal_error'],
		GatewayTimeout: [504, 'request_timeout'],
		ClientClosedRequest: [499, 'client_closed_request'],
		Error: [500, 'internal_error'],
	};
	for (const [responseName, [status, code]] of Object.entries(contracts)) {
		const response = componentResponseBlock(responseName);
		assert.match(response, /\$ref: '#\/components\/schemas\/ErrorResponse'/);
		assert.match(response, /example:[\s\S]+error:/, `${responseName} is missing error example`);
		assert.match(response, /example:[\s\S]+code:/, `${responseName} is missing code example`);
		assert.match(response, /example:[\s\S]+status:/, `${responseName} is missing status example`);
		assert.match(response, /example:[\s\S]+requestId:/, `${responseName} is missing requestId example`);
		assert.match(response, new RegExp(`example:[\\s\\S]+status: ${status}`), `${responseName} example has wrong status`);
		assert.match(response, new RegExp(`example:[\\s\\S]+code: ${code}`), `${responseName} example has wrong code`);
	}
});

test('ErrorResponse keeps status and request ids as stable response fields', () => {
	assert.match(generator, /validateErrorResponseSchema/);
	assert.match(generator, /ErrorResponse\.\$\{propertyName\} is missing required/);
	assert.match(generator, /ErrorResponse\.status is missing minimum: 400/);
	assert.match(generator, /ErrorResponse\.status is missing maximum: 599/);
	const schema = openapiSchema('ErrorResponse');
	assert.match(schema, /required: \[[^\]]*status[^\]]*\]/);
	assert.match(schema, /required: \[[^\]]*requestId[^\]]*\]/);
	assert.match(schema, /status:\n\s+type: integer\n\s+minimum: 400\n\s+maximum: 599/);
	assert.match(generatedTypes, /status: number;/);
	assert.doesNotMatch(generatedTypes, /status\?: number;/);
	assert.match(generatedTypes, /requestId: string;/);
	assert.doesNotMatch(generatedTypes, /requestId\?: string;/);
	assert.match(goHTTPUtilResponses, /Status\s+int\s+`json:"status"`/);
	assert.match(goHTTPUtilResponses, /RequestID\s+string\s+`json:"requestId"`/);
	assert.doesNotMatch(goHTTPUtilResponses, /Status\s+int\s+`json:"status,omitempty"`/);
	assert.doesNotMatch(goHTTPUtilResponses, /RequestID\s+string\s+`json:"requestId,omitempty"`/);
});

test('OpenAPI ErrorResponse code enum stays aligned with Go response constants', () => {
	const schema = openapiSchema('ErrorResponse');
	const enumValues = schema.match(/code:[\s\S]+?enum: \[([^\]]+)\]/)?.[1]
		.split(',')
		.map((value) => value.trim());
	const goCodes = [...goHTTPUtilResponses.matchAll(/Code[A-Za-z]+\s+=\s+"([^"]+)"/g)].map((match) => match[1]);

	assert.ok(enumValues, 'ErrorResponse.code must declare an enum');
	assert.deepEqual(new Set(enumValues), new Set(goCodes));
});

test('generator requires a 2xx success response for every operation', () => {
	assert.match(generator, /missing 2xx success response/);
	assert.match(generator, /\^2\\d\\d\$/);
});

test('generator requires declared path parameters for templated paths', () => {
	assert.match(generator, /validatePathParameters/);
	assert.match(generator, /missing path parameter declaration/);
	assert.match(generator, /path parameter \$\{param\.name\} must be required/);
	assert.match(openapi, /Id:\n\s+name: id\n\s+in: path\n\s+required: true\n\s+description: Positive resource identifier\.[\s\S]+example: 1/);
});

test('generator fails fast for unresolved response and header component refs', () => {
	assert.match(generator, /validateSchemaRefs\(source, new Set\(schemaNames\)\)/);
	assert.match(generator, /OpenAPI references unknown schema component \$\{schemaName\}/);
	assert.match(generator, /OpenAPI references unknown response component \$\{componentName\}/);
	assert.match(generator, /OpenAPI references unknown header component \$\{componentName\}/);
	assert.match(generator, /function responseComponentBlock[\s\S]+if \(start === -1\) \{[\s\S]+throw new Error\(`OpenAPI references unknown response component \$\{componentName\}`\);/);
	assert.match(generator, /function componentHeaderBlock[\s\S]+if \(start === -1\) \{[\s\S]+throw new Error\(`OpenAPI references unknown header component \$\{componentName\}`\);/);
});

test('generator merges OpenAPI path item parameters into each operation', () => {
	assert.match(generator, /parseEffectiveOperationParameters/);
	assert.match(generator, /mergeOpenAPIParameters/);
	assert.match(generator, /OpenAPI .* Path Item .* operation .*覆盖/);
	assert.match(generator, /parameterKey\(parameter\)/);
	assert.match(generator, /validateOperationQueryContract\(method, apiPath, methodBlock, responseStatuses, parameters, `\$\{sharedPathBlock\}\\n\$\{methodBlock\}`\)/);
	assert.match(generator, /has duplicate parameter \$\{parameter\.name\} in \$\{parameter\.in\}/);
});

test('generator requires explicit OpenAPI parameter names and locations', () => {
	assert.match(generator, /has a parameter missing name/);
	assert.match(generator, /parameter \$\{name\} is missing in/);
	assert.match(generator, /parameter \$\{name\} has unsupported in \$\{parameterIn\}/);
	assert.match(generator, /parseParameterRequired\(paramBlock, context, name\)/);
	assert.match(generator, /parameter \$\{name\} required must be true or false/);
	assert.doesNotMatch(generator, /in:\s+paramBlock\.match\([^;]+ \|\| 'query'/);
	assert.doesNotMatch(generator, /required:\s+paramBlock\.includes\('required: true'\)/);
});

test('generator requires X-Request-ID on success responses', () => {
	assert.match(generator, /validateSuccessResponseHeaders/);
	assert.match(generator, /missing X-Request-ID header/);
});

test('generator requires bounded visible ASCII request ids', () => {
	assert.match(generator, /validateRequestIDSchema/);
	assert.match(generator, /components\.parameters\.RequestID.*visible ASCII pattern/s);
	assert.match(generator, /components\.headers\.RequestID.*maxLength: 128/s);
	assert.match(generator, /is missing request id example/);
	assert.match(openapi, /parameters:[\s\S]+RequestID:[\s\S]+example: client-request-123/);
	assert.match(openapi, /headers:[\s\S]+RequestID:[\s\S]+example: client-request-123/);
});

test('generator requires closed request schemas', () => {
	assert.match(generator, /validateRequestSchemasAreClosed/);
	assert.match(generator, /is missing additionalProperties: false/);
});

test('generator requires schema required fields to exist in properties', () => {
	assert.match(generator, /validateSchemaRequiredProperties/);
	assert.match(generator, /\.required references missing property \$\{propertyName\}/);
	assert.match(generator, /parseProperties\(nestedBlock\(schema, 'properties:'\)\)/);
});

test('generator requires closed shared response schemas', () => {
	assert.match(generator, /validateSharedResponseSchemasAreClosed/);
	assert.match(openapi, /ErrorResponse:\n\s+type: object\n\s+additionalProperties: false/);
	assert.match(openapi, /OkResponse:\n\s+type: object\n\s+additionalProperties: false/);
});

test('generator requires closed core response schemas', () => {
	assert.match(generator, /validateCoreResponseSchemasAreClosed/);
	for (const schemaName of ['CurrentUser', 'LoginResponse', 'Account', 'Budget', 'Category', 'Transaction']) {
		assert.match(openapi, new RegExp(`${schemaName}:\\n\\s+type: object\\n\\s+additionalProperties: false`));
	}
});

test('generator requires stable current user response fields to be required', () => {
	assert.match(generator, /validateCurrentUserResponseSchema/);
	assert.match(generator, /CurrentUser\.id is missing readOnly: true/);
	const authDTO = goRequestDTOs.get('auth');
	assert.match(openapiSchemaPropertyBlock(openapiSchema('CurrentUser'), 'id'), /readOnly: true/);
	for (const fieldName of ['username', 'phone', 'email', 'wechatId']) {
		assert.doesNotMatch(generatedTypes, new RegExp(`${fieldName}\\?:`));
		assert.match(authDTO, new RegExp(`json:"${fieldName}"`));
		assert.doesNotMatch(authDTO, new RegExp(`json:"${fieldName},omitempty"`));
	}
});

test('generator requires server-owned core resource fields to be read-only', () => {
	assert.match(generator, /validateCoreResourceReadOnlyFields/);
	assert.match(generator, /\$\{schemaName\}\.\$\{propertyName\} is missing readOnly: true/);
	for (const schemaName of ['Account', 'Budget', 'Category', 'Transaction']) {
		const schema = openapiSchema(schemaName);
		for (const propertyName of ['id', 'userId', 'createdAt', 'updatedAt']) {
			assert.match(openapiSchemaPropertyBlock(schema, propertyName), /readOnly: true/, `${schemaName}.${propertyName} is missing readOnly`);
		}
	}
});

test('generator requires core resource timestamps in response schemas', () => {
	assert.match(generator, /validateCoreResourceTimestampSchemas/);
	for (const schemaName of ['Account', 'Budget', 'Category', 'Transaction']) {
		const schema = openapiSchema(schemaName);
		assert.match(schema, /required: \[[^\]]*createdAt[^\]]*updatedAt[^\]]*\]/);
		assert.match(schema, /createdAt:\n\s+type: string\n\s+format: date-time/);
		assert.match(schema, /updatedAt:\n\s+type: string\n\s+format: date-time/);
	}
	assert.match(generatedTypes, /createdAt: string;/);
	assert.match(generatedTypes, /updatedAt: string;/);
});

test('generator requires closed paginated response schemas', () => {
	assert.match(generator, /validatePaginatedResponseSchemasAreClosed/);
	assert.match(openapi, /TransactionListResponse:\n\s+type: object\n\s+additionalProperties: false/);
	assert.match(openapi, /Pagination:\n\s+type: object\n\s+additionalProperties: false/);
});

test('generator requires closed report response schemas', () => {
	assert.match(generator, /validateReportResponseSchemasAreClosed/);
	for (const schemaName of [
		'CategoryStat',
		'AccountStat',
		'MonthTrend',
		'TrendPoint',
		'CategoryTrendPoint',
		'AccountBalancePoint',
		'BudgetExecution',
		'SummaryTableRow',
		'PeriodTotals',
		'PeriodCompare',
		'Summary',
	]) {
		assert.match(openapi, new RegExp(`${schemaName}:\\n\\s+type: object\\n\\s+additionalProperties: false`));
	}
});

test('generator requires bounded report response schemas', () => {
	assert.match(generator, /validateReportResponseBounds/);
	assert.match(generator, /Summary\.trendGranularity is missing day\/week\/month enum/);
	for (const [schemaName, fields] of [
		['CategoryStat', [['categoryId', 1], ['amount', 0]]],
		['AccountStat', [['accountId', 1], ['amount', 0]]],
		['MonthTrend', [['income', 0], ['expense', 0]]],
		['TrendPoint', [['income', 0], ['expense', 0]]],
		['CategoryTrendPoint', [['categoryId', 1], ['amount', 0]]],
		['AccountBalancePoint', [['accountId', 1]]],
		['BudgetExecution', [['budgetId', 1], ['categoryId', 0], ['budget', 0], ['expense', 0], ['usageRate', 0]]],
		['SummaryTableRow', [['income', 0], ['expense', 0], ['txCount', 0]]],
		['PeriodTotals', [['income', 0], ['expense', 0]]],
		['Summary', [['income', 0], ['expense', 0]]],
	]) {
		const schema = openapiSchema(schemaName);
		for (const [propertyName, minimum] of fields) {
			assert.match(openapiSchemaPropertyBlock(schema, propertyName), new RegExp(`minimum: ${minimum}`), `${schemaName}.${propertyName} is missing minimum`);
		}
	}
	for (const [schemaName, fields] of [
		['AccountBalancePoint', ['net', 'balance']],
		['BudgetExecution', ['remaining']],
		['SummaryTableRow', ['balance']],
		['Summary', ['balance']],
	]) {
		const schema = openapiSchema(schemaName);
		for (const propertyName of fields) {
			const property = openapiSchemaPropertyBlock(schema, propertyName);
			assert.match(property, /multipleOf: 0\.01/, `${schemaName}.${propertyName} is missing cent precision`);
			assert.doesNotMatch(property, /minimum: 0/, `${schemaName}.${propertyName} must allow negative values`);
		}
	}
	assert.match(openapiSchemaPropertyBlock(openapiSchema('Summary'), 'trendGranularity'), /enum: \[day, week, month\]/);
});

test('generator requires realistic report response examples', () => {
	assert.match(generator, /validateReportResponseExamples/);
	for (const schemaName of [
		'CategoryStat',
		'AccountStat',
		'MonthTrend',
		'TrendPoint',
		'CategoryTrendPoint',
		'AccountBalancePoint',
		'BudgetExecution',
		'SummaryTableRow',
		'PeriodTotals',
		'PeriodCompare',
		'Summary',
	]) {
		assert.match(openapiSchema(schemaName), /example:/, `${schemaName} is missing example`);
	}
	const summary = openapiSchema('Summary');
	assert.match(summary, /trendGranularity: day/);
	assert.match(summary, /dailySummaries:/);
	assert.match(summary, /balance: -150/);
	assert.match(summary, /usageRate: 0\.5/);
});

test('generator requires summary response date range fields', () => {
	assert.match(generator, /validateSummaryResponseSchema/);
	assert.match(openapi, /start:\n\s+type: string\n\s+format: date-time/);
	assert.match(openapi, /end:\n\s+type: string\n\s+format: date-time/);
	assert.match(generatedTypes, /start: string;/);
	assert.match(generatedTypes, /end: string;/);
});

test('generator requires stable summary response fields to be required', () => {
	for (const fieldName of [
		'categoryId',
		'accountId',
		'monthlyTrend',
		'trendGranularity',
		'trend',
		'categoryTrend',
		'accountBalanceTrend',
		'budgetExecution',
		'dailySummaries',
		'monthlySummaries',
		'periodCompare',
	]) {
		assert.doesNotMatch(generatedTypes, new RegExp(`${fieldName}\\?:`));
	}
});

test('generator requires closed import response schemas', () => {
	assert.match(generator, /validateImportResponseSchemasAreClosed/);
	for (const schemaName of ['ImportPreviewRow', 'ImportPreview', 'ImportResult', 'ImportJob']) {
		assert.match(openapi, new RegExp(`${schemaName}:\\n\\s+type: object\\n\\s+additionalProperties: false`));
	}
});

test('generator requires bounded import response schemas', () => {
	assert.match(generator, /validateImportResponseBounds/);
	assert.match(generator, /ImportPreviewRow: \{ line: 1 \}/);
	assert.match(generator, /ImportJob: \{ id: 1, total: 0, success: 0, failed: 0, skipped: 0 \}/);
	for (const [schemaName, fields] of [
		['ImportPreviewRow', [['line', 1]]],
		['ImportPreview', [['size', 0], ['totalRows', 0], ['validRows', 0], ['failedRows', 0], ['duplicateRows', 0], ['maxRows', 0], ['maxFileBytes', 0]]],
		['ImportResult', [['total', 0], ['success', 0], ['failed', 0], ['skipped', 0]]],
		['ImportJob', [['id', 1], ['total', 0], ['success', 0], ['failed', 0], ['skipped', 0]]],
	]) {
		const schema = openapiSchema(schemaName);
		for (const [propertyName, minimum] of fields) {
			assert.match(openapiSchemaPropertyBlock(schema, propertyName), new RegExp(`minimum: ${minimum}`), `${schemaName}.${propertyName} is missing minimum`);
		}
	}
	const previewAmount = openapiSchemaPropertyBlock(openapiSchema('ImportPreviewRow'), 'amount');
	assert.match(previewAmount, /minimum: 0/);
	assert.match(previewAmount, /multipleOf: 0\.01/);
	const previewType = openapiSchemaPropertyBlock(openapiSchema('ImportPreviewRow'), 'type');
	assert.match(previewType, /enum: \[income, expense, ''\]/);
	assert.match(generator, /ImportPreviewRow\.type must allow an empty string for failed preview rows/);
	const importPreviewRows = Number(goRequestDTOs.get('dataio').match(/ImportPreviewRows\s+=\s+(\d+)/)?.[1]);
	assert.ok(importPreviewRows > 0, 'ImportPreviewRows must be a positive Go constant');
	assert.match(openapiSchemaPropertyBlock(openapiSchema('ImportPreview'), 'rows'), new RegExp(`maxItems: ${importPreviewRows}`));
});

test('generator requires realistic import response examples', () => {
	assert.match(generator, /validateImportResponseExamples/);
	assert.match(generator, /ImportPreview: \['filename: preview\.csv', 'maxRows: 5000', 'maxFileBytes: 5242880', 'valid: false', "type: ''", 'error: invalid occurred_at'\]/);
	const maxRows = Number(goRequestDTOs.get('dataio').match(/MaxImportRows\s+=\s+(\d+)/)?.[1]);
	const maxFileBytesExpression = goRequestDTOs.get('dataio').match(/MaxImportFileBytes\s+=\s+([^\n]+)/)?.[1];
	assert.equal(maxRows, 5000);
	assert.equal(maxFileBytesExpression, '5 * 1024 * 1024');

	const preview = openapiSchema('ImportPreview');
	assert.match(preview, /example:\n[\s\S]+filename: preview\.csv/);
	assert.match(preview, new RegExp(`maxRows: ${maxRows}`));
	assert.match(preview, /maxFileBytes: 5242880/);
	assert.match(preview, /valid: false/);
	assert.match(preview, /type: ''/);
	assert.match(preview, /error: invalid occurred_at/);

	for (const schemaName of ['ImportPreviewRow', 'ImportResult', 'ImportJob']) {
		assert.match(openapiSchema(schemaName), /example:/, `${schemaName} is missing example`);
	}
});

test('generator requires import job server-owned fields to be read-only', () => {
	assert.match(generator, /validateImportJobReadOnlyFields/);
	assert.match(generator, /ImportJob\.\$\{propertyName\} is missing readOnly: true/);
	const schema = openapiSchema('ImportJob');
	for (const propertyName of ['id', 'createdAt', 'updatedAt']) {
		assert.match(openapiSchemaPropertyBlock(schema, propertyName), /readOnly: true/, `ImportJob.${propertyName} is missing readOnly`);
	}
});

test('generator requires closed AI response schemas', () => {
	assert.match(generator, /validateAIResponseSchemasAreClosed/);
	for (const schemaName of ['AIParseResult', 'AIParseResponse']) {
		assert.match(openapi, new RegExp(`${schemaName}:\\n\\s+type: object\\n\\s+additionalProperties: false`));
	}
});

test('generator requires AI parse confidence in the response schema', () => {
	assert.match(generator, /validateAIResponseSchema/);
	assert.match(openapi, /confidence:\n\s+type: number\n\s+minimum: 0\n\s+maximum: 1/);
	assert.match(generatedTypes, /confidence: number;/);
});

test('generator requires request money fields to document cent precision', () => {
	assert.match(generator, /\['AccountRequest', 'multipleOf: 0\.01'\]/);
	assert.match(generator, /\['BudgetRequest', 'required: \[month, categoryId, amount\]'\]/);
	assert.match(generator, /\['BudgetRequest', 'minimum: 1'\]/);
	assert.match(generator, /\['BudgetRequest', 'multipleOf: 0\.01'\]/);
	assert.match(generator, /\['TransactionRequest', 'multipleOf: 0\.01'\]/);
	assert.match(openapi, /balance:\n\s+type: number\n\s+minimum: 0\n\s+multipleOf: 0\.01/);
	assert.match(openapi, /BudgetRequest:\n\s+type: object\n\s+additionalProperties: false\n\s+required: \[month, categoryId, amount\]/);
	assert.match(openapi, /amount:\n\s+type: number\n\s+exclusiveMinimum: 0\n\s+multipleOf: 0\.01/);
});

test('generator requires method-not-allowed responses for operations', () => {
	assert.match(generator, /missing 405 response/);
});

test('generator requires timeout responses when internal errors are declared', () => {
	assert.match(generator, /declares 500 response but is missing 504 timeout response/);
	assert.match(generator, /declares 500 response but is missing 499 client closed response/);
	for (const operation of openAPIOperationBlocks()) {
		if (!operation.includes("'500':")) {
			continue;
		}
		assert.match(operation, /'504':[\s\S]+\$ref: '#\/components\/responses\/GatewayTimeout'/);
		assert.match(operation, /'499':[\s\S]+\$ref: '#\/components\/responses\/ClientClosedRequest'/);
	}
});

test('generator requires body error responses for every request body operation', () => {
	assert.match(
		generator,
		/if \(operationHasRequestBody\(methodBlock\)\) \{[\s\S]+is missing 400 response[\s\S]+is missing 413 response[\s\S]+is missing 415 response/,
	);
	assert.doesNotMatch(generator, /operationResponseBlock\(methodBlock/);
});

test('OpenAPI request bodies match generated client assumptions', () => {
	assert.match(generator, /function validateRequestBodyContract\(method, apiPath, methodBlock\)/);
	assert.match(generator, /requestBody is missing description/);
	assert.match(generator, /requestBody must be required/);
	assert.match(generator, /requestBody must use application\/json or multipart\/form-data/);
	assert.match(generator, /requestBody must not mix JSON and multipart content/);
	assert.match(generator, /JSON requestBody must document Content-Type parameter handling/);
	assert.match(generator, /JSON requestBody must document structured JSON media type handling/);
	assert.match(generator, /multipart requestBody must document boundary parameter handling/);
	assert.match(generator, /multipart requestBody must document non-empty boundary requirement/);
	assert.match(generator, /multipart requestBody must document required file field/);
	assert.match(generator, /requestBody is missing a component schema reference/);
	assert.match(generator, /requestBody 400 response must use InvalidRequest/);
	assert.match(openapi, /application\/json:[\s\S]+description: 'Accepts `Content-Type: application\/json`, optional media type parameters such as `charset=utf-8`, and `application\/\*\+json` structured JSON media types\.'/);
	assert.match(openapi, /multipart\/form-data:[\s\S]+description: 'Accepts `Content-Type: multipart\/form-data; boundary=<non-empty-boundary>` with a required `file` field\.'/);

	const requestBodies = [...openapi.matchAll(/\n      requestBody:\n([\s\S]*?)(?=\n      responses:)/g)];
	assert.ok(requestBodies.length > 0, 'OpenAPI should contain request bodies');
	for (const [, requestBody] of requestBodies) {
		assert.match(requestBody, /^\s+description:\s+\S.+$/m);
		assert.match(requestBody, /required: true/);
		assert.match(requestBody, /(application\/json|multipart\/form-data):/);
		assert.match(requestBody, /\$ref: '#\/components\/schemas\/[A-Za-z][A-Za-z0-9]*'/);
	}
	for (const operation of openAPIOperationBlocks()) {
		if (!operation.includes('requestBody:')) {
			continue;
		}
		assert.match(operation, /'400':\n\s+\$ref: '#\/components\/responses\/InvalidRequest'/);
	}
});

test('Go request DTOs enforce OpenAPI request schema constraints at binding', () => {
	const expectations = [
		['auth', /Username string `json:"username" binding:"required,min=1"`/],
		['auth', /Password string `json:"password" binding:"required,min=1"`/],
		['accounts', /Name\s+string\s+`json:"name" binding:"required,min=1"`/],
		['accounts', /Type\s+string\s+`json:"type" binding:"required,min=1"`/],
		['accounts', /Balance\s+float64\s+`json:"balance" binding:"omitempty,min=0"`/],
		['budgets', /Month\s+string\s+`json:"month" binding:"required,datetime=2006-01"`/],
		['budgets', /CategoryID\s+uint\s+`json:"categoryId" binding:"required,min=1"`/],
		['budgets', /Amount\s+\*float64\s+`json:"amount" binding:"required,gt=0"`/],
		['categories', /Name string `json:"name" binding:"required,min=1"`/],
		['categories', /Type string `json:"type" binding:"required,oneof=income expense"`/],
		['transactions', /Type\s+string\s+`json:"type" binding:"required,oneof=income expense"`/],
		['transactions', /Amount\s+float64\s+`json:"amount" binding:"required,gt=0"`/],
		['transactions', /CategoryID\s+uint\s+`json:"categoryId" binding:"required,min=1"`/],
		['transactions', /AccountID\s+uint\s+`json:"accountId" binding:"required,min=1"`/],
		['transactions', /Note\s+string\s+`json:"note" binding:"required,min=1"`/],
		['transactions', /PageSize\s+\*int\s+`form:"pageSize" binding:"omitempty,min=1,max=200"`/],
		['transactions', /Keyword\s+string\s+`form:"q" binding:"omitempty,max=100"`/],
		['ai', /Text string `json:"text" binding:"required,min=1"`/],
		['dataio', /Content\s+string\s+`json:"content" binding:"required,min=1,max=5242880"`/],
	];
	for (const [moduleName, pattern] of expectations) {
		assert.match(goRequestDTOs.get(moduleName), pattern, `${moduleName} request DTO is missing ${pattern}`);
	}
});

test('generator requires Allow header on method-not-allowed response', () => {
	assert.match(generator, /components\.responses\.MethodNotAllowed is missing Allow header/);
	assert.match(generator, /components\.headers\.Allow must document comma-separated supported methods with an example/);
	assert.match(openapi, /Allow:[\s\S]+Comma-separated HTTP methods supported by the target route\./);
	assert.match(openapi, /Allow:[\s\S]+example: GET, POST/);
});

test('generator requires WWW-Authenticate header on unauthorized response', () => {
	assert.match(generator, /components\.responses\.Unauthorized is missing WWW-Authenticate header/);
	assert.match(generator, /components\.headers\.WWWAuthenticate is missing bearer realm guidance/);
	assert.match(generator, /components\.headers\.WWWAuthenticate is missing invalid_token guidance/);
	assert.match(generator, /components\.headers\.WWWAuthenticate is missing error_description guidance/);
	assert.match(generator, /components\.headers\.WWWAuthenticate is missing bearer challenge example/);
	assert.match(openapi, /WWWAuthenticate:[\s\S]+example: Bearer realm="haohao-accounting-api", error="invalid_token"/);
});

test('generator requires Retry-After header on rate-limited response', () => {
	assert.match(generator, /components\.responses\.RateLimited is missing Retry-After header/);
	assert.match(generator, /components\.responses\.RateLimited is missing \$\{headerName\} header/);
	assert.match(generator, /components\.headers\.RetryAfter must document remaining wait time as HTTP-date or non-negative integer delay-seconds/);
	assert.match(generator, /components\.headers\.\$\{componentName\} must document a bounded integer rate-limit value/);
	assert.match(openapi, /RetryAfter:[\s\S]+HTTP-date or non-negative integer delay-seconds/);
	assert.match(openapi, /RetryAfter:[\s\S]+example: '60'/);
	assert.match(openapi, /RateLimitLimit:[\s\S]+Maximum number of failed login attempts/);
	assert.match(openapi, /RateLimited:[\s\S]+RateLimit-Limit:[\s\S]+RateLimit-Remaining:[\s\S]+RateLimit-Reset:/);
});

test('generator requires Location header on accepted async resources', () => {
	assert.match(generator, /validateAcceptedResponseHeaders/);
	assert.match(generator, /\$\{method\.toUpperCase\(\)\} \$\{apiPath\} 202 response is missing Location header/);
	assert.match(generator, /components\.headers\.Location must document queued resource URLs with an example/);
	assert.match(openapi, /Location:[\s\S]+Relative URL of the created or queued API resource/);
	assert.match(openapi, /postIoImportJobs[\s\S]+'202':[\s\S]+Location:[\s\S]+\$ref: '#\/components\/headers\/Location'/);
});

test('generator requires Location header on created resources', () => {
	assert.match(generator, /validateCreatedResponseHeaders/);
	assert.match(generator, /\$\{method\.toUpperCase\(\)\} \$\{apiPath\} 201 response is missing Location header/);
	for (const operationId of ['postAccounts', 'postBudgets', 'postCategories', 'postTransactions']) {
		assert.match(openapi, new RegExp(`${operationId}[\\s\\S]+'201':[\\s\\S]+Location:[\\s\\S]+\\$ref: '#/components/headers/Location'`));
	}
});

test('generator requires not-acceptable responses for operations with response bodies', () => {
	assert.match(generator, /returns a response body but is missing 406 response/);
	assert.match(generator, /components\.responses\.NotAcceptable is missing application\/json error response/);
	assert.match(generator, /components\.responses\.NotAcceptable is missing Vary header/);
});

test('generator requires Vary headers for negotiated responses', () => {
	assert.match(generator, /components\.headers\.Vary is missing string schema/);
	assert.match(generator, /components\.headers\.Vary must document Accept negotiation/);
	assert.match(generator, /components\.headers\.Vary must document CORS origin variance/);
	assert.match(generator, /components\.headers\.Vary must document CORS preflight variance/);
	assert.match(generator, /components\.headers\.Vary is missing combined negotiation and CORS example/);
	assert.match(openapi, /Vary:[\s\S]+example: Accept, Origin, Access-Control-Request-Method, Access-Control-Request-Headers/);
	assert.match(generator, /negotiated response is missing Vary header/);
});

test('generator requires documented accepted datetime query formats', () => {
	assert.match(generator, /requireDateTimeQueryParameter/);
	assert.match(generator, /YYYY-MM-DD HH:mm:ss/);
	assert.match(generator, /Date-only values cover the entire day/);
	assert.match(openapi, /name: start[\s\S]+Accepts RFC3339, YYYY-MM-DD, YYYY-MM-DD HH:mm:ss, or YYYY\/MM\/DD/);
	assert.match(openapi, /name: start[\s\S]+example: '2026-06-01T00:00:00\+08:00'/);
	assert.match(openapi, /name: end[\s\S]+Date-only values cover the entire day/);
	assert.match(openapi, /name: end[\s\S]+example: '2026-06-30'/);
	assert.match(generator, /requireParameterText\(method, apiPath, parameterSourceBlock, 'trend', 'default: month'\)/);
	assert.match(generator, /requireParameterText\(method, apiPath, parameterSourceBlock, 'trend', 'example: month'\)/);
	assert.match(generator, /requireParameterText\(method, apiPath, parameterSourceBlock, 'categoryId', 'Category id filter'\)/);
	assert.match(generator, /requireParameterText\(method, apiPath, parameterSourceBlock, 'accountId', 'Account id filter'\)/);
	assert.match(generator, /query 400 response must use InvalidRequest/);
	assert.match(openapi, /name: categoryId\n\s+in: query\n\s+description: Category id filter\.[\s\S]+example: 1/);
	assert.match(openapi, /name: accountId\n\s+in: query\n\s+description: Account id filter\.[\s\S]+example: 1/);
	assert.match(openapi, /name: trend\n\s+in: query\n\s+description: Trend aggregation granularity\. Defaults to month when omitted\.[\s\S]+default: month/);
	assert.match(openapi, /name: trend[\s\S]+example: month/);
	assert.match(generator, /requireParameterText\(method, apiPath, parameterSourceBlock, 'month', 'Budget month in YYYY-MM format'\)/);
	assert.match(generator, /requireParameterText\(method, apiPath, parameterSourceBlock, 'month', "example: '2026-06'"\)/);
	assert.match(openapi, /name: month\n\s+in: query\n\s+description: Budget month in YYYY-MM format\.[\s\S]+pattern: '\^\\d\{4\}-\\d\{2\}\$'/);
	assert.match(openapi, /name: month[\s\S]+example: '2026-06'/);
	for (const operation of openAPIOperationBlocks()) {
		if (!operation.includes('in: query')) {
			continue;
		}
		assert.match(operation, /'400':\n\s+\$ref: '#\/components\/responses\/InvalidRequest'/);
	}
});

test('generator requires documented download filename headers', () => {
	assert.match(generator, /GET \/io\/export is missing Content-Disposition response header/);
	assert.match(generator, /operationHasJSONSuccessContent\(methodBlock, source\)/);
	assert.match(generator, /result\.filter\(\(endpoint\) => endpoint\.jsonClientEndpoint\)/);
	assert.match(generator, /requireParameterText\(method, apiPath, parameterSourceBlock, 'format', 'default: csv'\)/);
	assert.match(generator, /requireParameterText\(method, apiPath, parameterSourceBlock, 'format', 'Values are trimmed and case-insensitive'\)/);
	assert.match(generator, /requireParameterText\(method, apiPath, parameterSourceBlock, 'format', 'defaults to csv when omitted'\)/);
	assert.match(generator, /requireParameterText\(method, apiPath, parameterSourceBlock, 'format', 'example: csv'\)/);
	assert.match(openapi, /name: format[\s\S]+description: Export file format\. Values are trimmed and case-insensitive; defaults to csv when omitted\.[\s\S]+default: csv/);
	assert.match(openapi, /name: format[\s\S]+example: csv/);
	assert.match(generator, /components\.headers\.ContentDisposition is missing filename\* guidance/);
	assert.match(openapi, /ContentDisposition:[\s\S]+example: attachment; filename="transactions\.csv"; filename\*=UTF-8''transactions\.csv/);
	assert.doesNotMatch(generatedClient, /getIoExport/);
	assert.match(webDataioApi, /download\(`\/io\/export\?format=\$\{format\}`, exportAccept\(format\)\)/);
	assert.match(mobileDataioApi, /download\(`\/io\/export\?format=\$\{format\}`, exportAccept\(format\)\)/);
	for (const source of [webApiClient, mobileApiClient]) {
		assert.match(source, /function contentDispositionParams\(disposition: string\)/);
		assert.match(source, /function splitHeaderParameters\(value: string\)/);
		assert.match(source, /function unquoteHeaderValue\(value: string\)/);
		assert.match(source, /function decodeExtendedFilename\(value: string\)/);
		assert.match(source, /function safeDecodeURIComponent\(value: string\)/);
		assert.match(source, /params\.get\('filename\*'\)/);
		assert.match(source, /params\.get\('filename'\)/);
		assert.match(source, /if \(match\[1\] && match\[1\]\.toLowerCase\(\) !== 'utf-8'\) \{/);
		assert.match(source, /return safeDecodeURIComponent\(match\[2\]\);/);
		assert.match(source, /catch \{\n\s+return '';\n\s+\}/);
	}
});

test('generator requires browser data clearing headers on logout success', () => {
	assert.match(generator, /POST \/auth\/logout 200 response is missing Clear-Site-Data header/);
	assert.match(generator, /components\.headers\.ClearSiteData must document logout browser data clearing directives/);
	assert.match(openapi, /postAuthLogout[\s\S]+'200':[\s\S]+Clear-Site-Data:[\s\S]+\$ref: '#\/components\/headers\/ClearSiteData'/);
	assert.match(openapi, /ClearSiteData:[\s\S]+Browser-side data cleared after logout/);
	assert.match(openapi, /ClearSiteData:[\s\S]+example: '"cache", "cookies", "storage"'/);
});

test('generator requires documented import headers', () => {
	assert.match(generator, /ImportTextRequest', 'occurred_at,type,amount,category,account,note,tags'/);
	assert.match(generator, /ImportFileRequest', 'occurred_at,type,amount,category,account,note,tags'/);
	assert.match(generator, /ImportTextRequest', 'UTF-8 BOM'/);
	assert.match(generator, /ImportTextRequest', 'skipDuplicates defaults to true'/);
	assert.match(generator, /ImportFileRequest', 'UTF-8 BOM'/);
	assert.match(generator, /ImportFileRequest', 'skipDuplicates defaults to true'/);
	assert.match(generator, /ImportFileRequest', 'invalid values return invalid_request'/);
	assert.match(openapi, /ImportTextRequest:[\s\S]+UTF-8 BOM on the first header is accepted/);
	assert.match(openapi, /ImportTextRequest:[\s\S]+skipDuplicates:[\s\S]+description: skipDuplicates defaults to true when omitted\.[\s\S]+default: true/);
	assert.match(openapi, /ImportFileRequest:[\s\S]+UTF-8 BOM on the first header is accepted/);
	assert.match(openapi, /ImportFileRequest:[\s\S]+skipDuplicates:[\s\S]+description: skipDuplicates defaults to true when omitted; invalid values return invalid_request\.[\s\S]+default: true/);
});

test('generator requires bounded pagination response schema', () => {
	assert.match(generator, /validatePaginationSchema/);
	assert.match(generator, /Pagination\.pageSize is missing maximum: 200/);
	assert.match(generator, /Pagination\.total is missing minimum: 0/);
	assert.match(generator, /requireParameterText\(method, apiPath, parameterSourceBlock, 'page', 'default: 1'\)/);
	assert.match(generator, /requireParameterText\(method, apiPath, parameterSourceBlock, 'page', 'example: 1'\)/);
	assert.match(generator, /requireParameterText\(method, apiPath, parameterSourceBlock, 'pageSize', 'default: 20'\)/);
	assert.match(generator, /requireParameterText\(method, apiPath, parameterSourceBlock, 'pageSize', 'example: 20'\)/);
	assert.match(generator, /requireParameterText\(method, apiPath, parameterSourceBlock, 'type', 'Transaction type filter'\)/);
	assert.match(generator, /requireParameterText\(method, apiPath, parameterSourceBlock, 'type', 'example: expense'\)/);
	assert.match(generator, /apiPath === '\/categories'[\s\S]+requireParameterText\(method, apiPath, parameterSourceBlock, 'type', 'Transaction type filter'\)/);
	assert.match(generator, /requireParameterText\(method, apiPath, parameterSourceBlock, 'categoryId', 'Category id filter'\)/);
	assert.match(generator, /requireParameterText\(method, apiPath, parameterSourceBlock, 'categoryId', 'example: 1'\)/);
	assert.match(generator, /requireParameterText\(method, apiPath, parameterSourceBlock, 'accountId', 'Account id filter'\)/);
	assert.match(generator, /requireParameterText\(method, apiPath, parameterSourceBlock, 'accountId', 'example: 1'\)/);
	assert.match(generator, /requireParameterText\(method, apiPath, parameterSourceBlock, 'q', 'matched against transaction notes and tags'\)/);
	assert.match(generator, /requireParameterText\(method, apiPath, parameterSourceBlock, 'q', 'Maximum 100 characters'\)/);
	assert.match(generator, /requireParameterText\(method, apiPath, parameterSourceBlock, 'q', 'maxLength: 100'\)/);
	assert.match(generator, /requireParameterText\(method, apiPath, parameterSourceBlock, 'q', 'example: lunch'\)/);
	assert.match(openapi, /name: page\n\s+in: query\n\s+description: Page number\. Defaults to 1 when omitted\.[\s\S]+default: 1/);
	assert.match(openapi, /name: page[\s\S]+example: 1/);
	assert.match(openapi, /name: pageSize\n\s+in: query\n\s+description: Number of items per page\. Defaults to 20 when omitted\.[\s\S]+default: 20/);
	assert.match(openapi, /name: pageSize[\s\S]+example: 20/);
	assert.match(openapi, /name: type\n\s+in: query\n\s+description: Transaction type filter\.[\s\S]+\$ref: '#\/components\/schemas\/TransactionType'[\s\S]+example: expense/);
	assert.match(openapi, /\/categories:[\s\S]+name: type\n\s+in: query\n\s+description: Transaction type filter\.[\s\S]+\$ref: '#\/components\/schemas\/TransactionType'[\s\S]+example: expense/);
	assert.match(openapi, /name: categoryId\n\s+in: query\n\s+description: Category id filter\.[\s\S]+example: 1/);
	assert.match(openapi, /name: accountId\n\s+in: query\n\s+description: Account id filter\.[\s\S]+example: 1/);
	assert.match(openapi, /name: q\n\s+in: query\n\s+description: Keyword filter matched against transaction notes and tags\. Maximum 100 characters\.[\s\S]+maxLength: 100/);
	assert.match(openapi, /name: q[\s\S]+example: lunch/);
	assert.match(openapi, /Link:[\s\S]+example: <\/api\/v1\/transactions\?page=2&pageSize=20>; rel="next"/);
	assert.match(openapi, /Link:[\s\S]+sensitive query parameters are omitted from generated links/);
	assert.match(openapi, /TotalCount:[\s\S]+example: 95/);
});

test('generator requires login rate limit response', () => {
	assert.match(generator, /validateAuthOperationContract/);
	assert.match(generator, /POST \/auth\/login is missing 429 rate limited response/);
});

test('generator requires explicit auth security contract', () => {
	assert.match(generator, /must explicitly declare security: \[\]/);
	assert.match(generator, /must require bearer authentication/);
	assert.match(generator, /must document 401 for bearer authentication/);
	assert.match(generator, /const publicOperations = new Set\(\['POST \/auth\/login'\]\)/);
	assert.match(generator, /validateSecuritySchemes\(source\)/);
	assert.match(generator, /components\.securitySchemes\.bearerAuth must document HTTP bearer JWT auth/);
	assert.match(generator, /components\.securitySchemes\.bearerAuth is missing Authorization header guidance/);
	assert.match(generator, /components\.securitySchemes\.bearerAuth is missing repeated Authorization header guidance/);
	assert.match(generator, /components\.securitySchemes\.bearerAuth is missing JWT not-before claim guidance/);
	assert.match(openapi, /bearerAuth:[\s\S]+scheme: bearer[\s\S]+bearerFormat: JWT[\s\S]+Authorization: Bearer <JWT>[\s\S]+repeated `Authorization` headers are rejected[\s\S]+`nbf` not-before claim/);
});

function duplicateYamlMappingKeys(source) {
	const duplicates = [];
	const stack = [{ indent: -1, keys: new Set() }];
	const keyPattern = /^(\s*)(-\s+)?([^:]+):(?:\s*(.*))?$/;

	for (const [index, line] of source.split(/\r?\n/).entries()) {
		if (!line.trim() || line.trimStart().startsWith('#')) continue;
		const match = line.match(keyPattern);
		if (!match) continue;

		const indent = match[1].length;
		while (stack.length > 1 && indent <= stack.at(-1).indent) {
			stack.pop();
		}

		const scope = match[2] ? { indent, keys: new Set() } : stack.at(-1);
		if (match[2]) {
			stack.push(scope);
		}
		const key = match[3].trim().replace(/^['"]|['"]$/g, '');
		if (scope.keys.has(key)) {
			duplicates.push(`${key} at line ${index + 1}`);
		}
		scope.keys.add(key);
		if (!match[4]?.trim()) {
			stack.push({ indent, keys: new Set() });
		}
	}

	return duplicates;
}

function escapeRegExp(value) {
	return value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

function openapiSchemaPropertyBlock(schema, propertyName) {
	const propertiesStart = schema.indexOf('      properties:\n');
	assert.notEqual(propertiesStart, -1, 'schema is missing properties block');
	const properties = schema.slice(propertiesStart);
	const match = properties.match(new RegExp(`^        ${escapeRegExp(propertyName)}:\\n`, 'm'));
	assert.ok(match && match.index !== undefined, `schema is missing property ${propertyName}`);
	const rest = properties.slice(match.index);
	const nextProperty = rest.search(/\n        [A-Za-z][A-Za-z0-9]*:/);
	return nextProperty === -1 ? rest : rest.slice(0, nextProperty);
}

function expectedOperationId(method, apiPath) {
	const cleanPath = apiPath.replace(/^\//, '').replace(/\{id\}/g, 'by-id');
	const words = cleanPath.split(/[/-]/).filter(Boolean);
	const base = words.map((word, index) => (index === 0 ? word : upperFirst(word))).join('');
	return `${method}${upperFirst(base)}`;
}

function openapiOperations(source) {
	const pathsBlock = source.slice(source.indexOf('paths:'), source.indexOf('components:'));
	const pathMatches = [...pathsBlock.matchAll(/^  (\/[^:]+):$/gm)];
	const operations = [];
	for (const [pathIndex, pathMatch] of pathMatches.entries()) {
		const path = pathMatch[1];
		const pathStart = pathMatch.index;
		const pathEnd = pathMatches[pathIndex + 1]?.index ?? pathsBlock.length;
		const pathBlock = pathsBlock.slice(pathStart, pathEnd);
		const methodMatches = [...pathBlock.matchAll(/^    (get|put|post|delete|options|head|patch|trace):$/gm)];
		for (const [methodIndex, methodMatch] of methodMatches.entries()) {
			const method = methodMatch[1];
			const methodStart = methodMatch.index;
			const methodEnd = methodMatches[methodIndex + 1]?.index ?? pathBlock.length;
			const methodBlock = pathBlock.slice(methodStart, methodEnd);
			const operationId = methodBlock.match(/operationId:\s+([A-Za-z][A-Za-z0-9]*)/)?.[1] || '';
			operations.push({ method, operationId, path });
		}
	}
	return operations;
}

function upperFirst(value) {
	return value.charAt(0).toUpperCase() + value.slice(1);
}
