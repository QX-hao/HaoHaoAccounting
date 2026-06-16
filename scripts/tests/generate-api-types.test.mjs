import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { test } from 'node:test';

const generator = readFileSync(new URL('../generate-api-types.mjs', import.meta.url), 'utf8');
const openapi = readFileSync(new URL('../../backend/api/openapi.yaml', import.meta.url), 'utf8');
const generatedClient = readFileSync(new URL('../../web/shared/api/generated-client.ts', import.meta.url), 'utf8');
const generatedTypes = readFileSync(new URL('../../web/shared/types/api.ts', import.meta.url), 'utf8');
const webApiClient = readFileSync(new URL('../../web/shared/api/client.ts', import.meta.url), 'utf8');
const webDataioApi = readFileSync(new URL('../../web/features/dataio/api.ts', import.meta.url), 'utf8');
const mobileApiClient = readFileSync(new URL('../../mobile/src/shared/api/client.ts', import.meta.url), 'utf8');
const mobileDataioApi = readFileSync(new URL('../../mobile/src/features/dataio/api.ts', import.meta.url), 'utf8');

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

test('generated types preserve property enum values', () => {
	assert.match(generator, /propertyEnumValues/);
});

test('generated error codes stay available to API clients', () => {
	assert.match(generatedTypes, /request_timeout/);
	assert.match(generatedTypes, /client_closed_request/);
	assert.match(generatedTypes, /not_acceptable/);
	assert.match(webApiClient, /ErrorResponse\['code'\]/);
	assert.match(webApiClient, /Retry-After/);
	assert.match(webApiClient, /authenticateChallenge/);
	assert.match(webApiClient, /WWW-Authenticate/);
	assert.match(webApiClient, /network_error/);
	assert.match(webApiClient, /function networkError\(err: unknown\)/);
	assert.match(mobileApiClient, /ErrorResponse\['code'\]/);
	assert.match(mobileApiClient, /Retry-After/);
	assert.match(mobileApiClient, /authenticateChallenge/);
	assert.match(mobileApiClient, /WWW-Authenticate/);
	assert.match(mobileApiClient, /network_error/);
	assert.match(mobileApiClient, /function networkError\(err: unknown\)/);
});

test('API clients send explicit Accept headers for negotiated responses', () => {
	assert.match(webApiClient, /headers\.set\('Accept', headers\.get\('Accept'\) \|\| 'application\/json'\)/);
	assert.match(webApiClient, /headers\.set\('Accept', 'application\/json'\)/);
	assert.match(webDataioApi, /return 'text\/csv'/);
	assert.match(webDataioApi, /application\/vnd\.openxmlformats-officedocument\.spreadsheetml\.sheet/);
	assert.match(mobileApiClient, /headers\.set\('Accept', headers\.get\('Accept'\) \|\| 'application\/json'\)/);
	assert.match(mobileDataioApi, /downloadText\('\/io\/export\?format=csv', 'text\/csv'\)/);
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
	assert.match(openapi, /'200':\n\s+description: Login success\n\s+headers:\n\s+Cache-Control:/);
});

test('API clients only default JSON Content-Type when a non-FormData body is present', () => {
	const expected = /if \(init\.body !== undefined && init\.body !== null && !\(init\.body instanceof FormData\)\) \{\s+headers\.set\('Content-Type', headers\.get\('Content-Type'\) \|\| 'application\/json'\);/;
	assert.match(webApiClient, expected);
	assert.match(mobileApiClient, expected);
});

test('mobile CSV export uses shared text download error handling', () => {
	assert.match(mobileApiClient, /export async function downloadText/);
	assert.match(mobileApiClient, /const data = await parseErrorBody\(resp\);/);
	assert.match(mobileDataioApi, /return downloadText\('\/io\/export\?format=csv', 'text\/csv'\)/);
	assert.doesNotMatch(mobileDataioApi, /fetch\(`\$\{API_BASE\}\/io\/export/);
});

test('API clients parse non-JSON error bodies through the shared error parser', () => {
	const expected = /if \(!resp\.ok\) \{[\s\S]+const data = await parseErrorBody\(resp\);[\s\S]+throw apiError\(resp, data\);[\s\S]+\}\s+const data = await resp\.json\(\)\.catch\(\(\) => \(\{\}\)\);/;
	assert.match(webApiClient, expected);
	assert.match(mobileApiClient, expected);
});

test('API clients route logout through shared network error handling', () => {
	assert.match(webApiClient, /await fetchAPI\('\/auth\/logout', \{/);
	assert.match(mobileApiClient, /await fetchAPI\('\/auth\/logout', \{/);
	assert.doesNotMatch(webApiClient, /fetch\(`\$\{API_BASE\}\/auth\/logout/);
	assert.doesNotMatch(mobileApiClient, /fetch\(`\$\{API_BASE\}\/auth\/logout/);
});

test('API clients support Retry-After delay seconds and HTTP-date values', () => {
	const expected = /const retryAt = Date\.parse\(value\);[\s\S]+return Math\.max\(0, Math\.ceil\(\(retryAt - Date\.now\(\)\) \/ 1000\)\);/;
	assert.match(webApiClient, expected);
	assert.match(mobileApiClient, expected);
});

test('generator rejects duplicate OpenAPI operationId values', () => {
	assert.match(generator, /operationIds\.has\(operationId\)/);
	assert.match(generator, /is used by both/);
});

test('generator requires a 2xx success response for every operation', () => {
	assert.match(generator, /missing 2xx success response/);
	assert.match(generator, /\^2\\d\\d\$/);
});

test('generator requires declared path parameters for templated paths', () => {
	assert.match(generator, /validatePathParameters/);
	assert.match(generator, /missing path parameter declaration/);
});

test('generator requires X-Request-ID on success responses', () => {
	assert.match(generator, /validateSuccessResponseHeaders/);
	assert.match(generator, /missing X-Request-ID header/);
});

test('generator requires bounded visible ASCII request ids', () => {
	assert.match(generator, /validateRequestIDSchema/);
	assert.match(generator, /components\.parameters\.RequestID.*visible ASCII pattern/s);
	assert.match(generator, /components\.headers\.RequestID.*maxLength: 128/s);
});

test('generator requires closed request schemas', () => {
	assert.match(generator, /validateRequestSchemasAreClosed/);
	assert.match(generator, /is missing additionalProperties: false/);
});

test('generator requires closed shared response schemas', () => {
	assert.match(generator, /validateSharedResponseSchemasAreClosed/);
	assert.match(openapi, /ErrorResponse:\n\s+type: object\n\s+additionalProperties: false/);
	assert.match(openapi, /OkResponse:\n\s+type: object\n\s+additionalProperties: false/);
});

test('generator requires request money fields to document cent precision', () => {
	assert.match(generator, /\['AccountRequest', 'multipleOf: 0\.01'\]/);
	assert.match(generator, /\['BudgetRequest', 'multipleOf: 0\.01'\]/);
	assert.match(generator, /\['TransactionRequest', 'multipleOf: 0\.01'\]/);
	assert.match(openapi, /balance:\n\s+type: number\n\s+minimum: 0\n\s+multipleOf: 0\.01/);
	assert.match(openapi, /amount:\n\s+type: number\n\s+exclusiveMinimum: 0\n\s+multipleOf: 0\.01/);
});

test('generator requires method-not-allowed responses for operations', () => {
	assert.match(generator, /missing 405 response/);
});

test('generator requires timeout responses when internal errors are declared', () => {
	assert.match(generator, /declares 500 response but is missing 504 timeout response/);
});

test('generator requires body error responses for every request body operation', () => {
	assert.match(
		generator,
		/if \(operationHasRequestBody\(methodBlock\)\) \{[\s\S]+is missing 400 response[\s\S]+is missing 413 response[\s\S]+is missing 415 response/,
	);
});

test('generator requires Allow header on method-not-allowed response', () => {
	assert.match(generator, /components\.responses\.MethodNotAllowed is missing Allow header/);
});

test('generator requires WWW-Authenticate header on unauthorized response', () => {
	assert.match(generator, /components\.responses\.Unauthorized is missing WWW-Authenticate header/);
	assert.match(generator, /components\.headers\.WWWAuthenticate is missing invalid_token guidance/);
});

test('generator requires Retry-After header on rate-limited response', () => {
	assert.match(generator, /components\.responses\.RateLimited is missing Retry-After header/);
	assert.match(generator, /components\.headers\.RetryAfter must document HTTP-date or delay-seconds/);
});

test('generator requires not-acceptable responses for operations with response bodies', () => {
	assert.match(generator, /returns a response body but is missing 406 response/);
	assert.match(generator, /components\.responses\.NotAcceptable is missing application\/json error response/);
	assert.match(generator, /components\.responses\.NotAcceptable is missing Vary header/);
});

test('generator requires Vary headers for negotiated responses', () => {
	assert.match(generator, /components\.headers\.Vary is missing string schema/);
	assert.match(generator, /negotiated response is missing Vary header/);
});

test('generator requires documented accepted datetime query formats', () => {
	assert.match(generator, /requireDateTimeQueryParameter/);
	assert.match(generator, /YYYY-MM-DD HH:mm:ss/);
	assert.match(generator, /Date-only values cover the entire day/);
	assert.match(openapi, /name: start[\s\S]+Accepts RFC3339, YYYY-MM-DD, YYYY-MM-DD HH:mm:ss, or YYYY\/MM\/DD/);
	assert.match(openapi, /name: end[\s\S]+Date-only values cover the entire day/);
});

test('generator requires documented download filename headers', () => {
	assert.match(generator, /GET \/io\/export is missing Content-Disposition response header/);
	assert.match(generator, /components\.headers\.ContentDisposition is missing filename\* guidance/);
	assert.match(webApiClient, /function safeDecodeURIComponent\(value: string\)/);
	assert.match(webApiClient, /catch \{\n\s+return '';\n\s+\}/);
});

test('generator requires documented import headers', () => {
	assert.match(generator, /ImportTextRequest', 'occurred_at,type,amount,category,account,note,tags'/);
	assert.match(generator, /ImportFileRequest', 'occurred_at,type,amount,category,account,note,tags'/);
});

test('generator requires bounded pagination response schema', () => {
	assert.match(generator, /validatePaginationSchema/);
	assert.match(generator, /Pagination\.pageSize is missing maximum: 200/);
	assert.match(generator, /Pagination\.total is missing minimum: 0/);
});

test('generator requires login rate limit response', () => {
	assert.match(generator, /validateAuthOperationContract/);
	assert.match(generator, /POST \/auth\/login is missing 429 rate limited response/);
});

test('generator requires explicit auth security contract', () => {
	assert.match(generator, /must explicitly declare security: \[\]/);
	assert.match(generator, /must require bearer authentication/);
});
