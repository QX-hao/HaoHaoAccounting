import fs from 'node:fs';
import path from 'node:path';

const root = path.resolve(new URL('..', import.meta.url).pathname);
const openapiPath = path.join(root, 'backend/api/openapi.yaml');
const typeOutputPaths = [
  path.join(root, 'web/shared/types/api.ts'),
  path.join(root, 'mobile/src/shared/types/api.ts'),
];
const clientOutputPaths = [
  path.join(root, 'web/shared/api/generated-client.ts'),
  path.join(root, 'mobile/src/shared/api/generated-client.ts'),
];

const source = fs.readFileSync(openapiPath, 'utf8');
const schemasSource = source.slice(source.indexOf('  schemas:'));
const schemaNames = [...schemasSource.matchAll(/^    ([A-Za-z][A-Za-z0-9]*):$/gm)].map((match) => match[1]);
const duplicateSchemaNames = schemaNames.filter((name, index) => schemaNames.indexOf(name) !== index);
if (duplicateSchemaNames.length > 0) {
  throw new Error(`Duplicate schemas: ${[...new Set(duplicateSchemaNames)].join(', ')}`);
}
const schemas = Object.fromEntries(schemaNames.map((name, index) => {
  const marker = `    ${name}:`;
  const start = schemasSource.indexOf(marker);
  const nextName = schemaNames[index + 1];
  const end = nextName ? schemasSource.indexOf(`    ${nextName}:`, start + marker.length) : schemasSource.length;
  return [name, schemasSource.slice(start, end)];
}));

validateOpenAPIDescription(source);
validateOpenAPIServers(source);
validateOpenAPITags(source);
validateSchemaConstraints(schemas);
validateParameterConstraints(source);
validateResponseComponents(source);
validateSecuritySchemes(source);

const generated = [
  '// Generated from backend/api/openapi.yaml. Do not edit by hand.',
  '',
  ...schemaNames.map((name) => emitType(name, schemas[name])),
  '',
].join('\n');

for (const outputPath of typeOutputPaths) {
  fs.mkdirSync(path.dirname(outputPath), { recursive: true });
  fs.writeFileSync(outputPath, generated);
}

const endpoints = parseEndpoints(source);
const client = emitClient(endpoints);
for (const outputPath of clientOutputPaths) {
  fs.mkdirSync(path.dirname(outputPath), { recursive: true });
  fs.writeFileSync(outputPath, client);
}

function validateOpenAPIDescription(openapi) {
  const info = topLevelBlock(openapi, 'info');
  if (!info.includes('CORS_ALLOW_ORIGINS') || !info.includes('Access-Control-Allow-Origin')) {
    throw new Error('OpenAPI info.description must document CORS allowlist rejection behavior');
  }
}

function validateOpenAPIServers(openapi) {
  const servers = topLevelBlock(openapi, 'servers');
  if (!servers.includes('url: http://localhost:8080/api/v1')) {
    throw new Error('OpenAPI servers must include the local development API base URL');
  }
  if (!servers.includes('description: Local development API server.')) {
    throw new Error('OpenAPI local server must include a human-readable description');
  }
}

function validateOpenAPITags(openapi) {
  const tags = declaredOpenAPITags(openapi);
  if (tags.size === 0) {
    throw new Error('OpenAPI top-level tags must declare API groups');
  }
}

function validateSchemaConstraints(allSchemas) {
  const checks = [
    ['LoginRequest', 'minLength: 1'],
    ['AccountRequest', 'minLength: 1'],
    ['AccountRequest', 'minimum: 0'],
    ['AccountRequest', 'multipleOf: 0.01'],
    ['BudgetRequest', "pattern: '^\\d{4}-\\d{2}$'"],
    ['BudgetRequest', 'required: [month, categoryId, amount]'],
    ['BudgetRequest', 'minimum: 1'],
    ['BudgetRequest', 'exclusiveMinimum: 0'],
    ['BudgetRequest', 'multipleOf: 0.01'],
    ['CategoryRequest', 'minLength: 1'],
    ['TransactionRequest', 'exclusiveMinimum: 0'],
    ['TransactionRequest', 'multipleOf: 0.01'],
    ['TransactionRequest', 'minimum: 1'],
    ['TransactionRequest', 'minLength: 1'],
    ['AIParseRequest', 'minLength: 1'],
    ['ImportTextRequest', 'maxLength: 5242880'],
    ['ImportTextRequest', 'occurred_at,type,amount,category,account,note,tags'],
    ['ImportTextRequest', 'UTF-8 BOM'],
    ['ImportFileRequest', 'occurred_at,type,amount,category,account,note,tags'],
    ['ImportFileRequest', 'UTF-8 BOM'],
  ];
  for (const [schemaName, requiredText] of checks) {
    if (!allSchemas[schemaName]?.includes(requiredText)) {
      throw new Error(`${schemaName} is missing ${requiredText}`);
    }
  }

  validateRequestSchemasAreClosed(allSchemas);
  validateSharedResponseSchemasAreClosed(allSchemas);
  validateErrorResponseSchema(allSchemas.ErrorResponse || '');
  validateCoreResponseSchemasAreClosed(allSchemas);
  validateCurrentUserResponseSchema(allSchemas.CurrentUser || '');
  validateCoreResourceTimestampSchemas(allSchemas);
  validatePaginatedResponseSchemasAreClosed(allSchemas);
  validateReportResponseSchemasAreClosed(allSchemas);
  validateSummaryResponseSchema(allSchemas.Summary || '');
  validateImportResponseSchemasAreClosed(allSchemas);
  validateAIResponseSchemasAreClosed(allSchemas);
  validateAIResponseSchema(allSchemas.AIParseResult || '');
  validatePaginationSchema(allSchemas.Pagination || '');
}

function validateRequestSchemasAreClosed(allSchemas) {
  for (const schemaName of Object.keys(allSchemas)) {
    if (!schemaName.endsWith('Request')) {
      continue;
    }
    if (!allSchemas[schemaName].includes('additionalProperties: false')) {
      throw new Error(`${schemaName} is missing additionalProperties: false`);
    }
  }
}

function validateSharedResponseSchemasAreClosed(allSchemas) {
  for (const schemaName of ['ErrorResponse', 'OkResponse']) {
    if (!allSchemas[schemaName]?.includes('additionalProperties: false')) {
      throw new Error(`${schemaName} is missing additionalProperties: false`);
    }
  }
}

function validateCoreResponseSchemasAreClosed(allSchemas) {
  for (const schemaName of ['CurrentUser', 'LoginResponse', 'Account', 'Budget', 'Category', 'Transaction']) {
    if (!allSchemas[schemaName]?.includes('additionalProperties: false')) {
      throw new Error(`${schemaName} is missing additionalProperties: false`);
    }
  }
}

function validateErrorResponseSchema(schema) {
  for (const propertyName of ['error', 'code', 'requestId']) {
    if (!schemaRequiredProperties(schema).has(propertyName)) {
      throw new Error(`ErrorResponse.${propertyName} is missing required`);
    }
  }
}

function validateCurrentUserResponseSchema(schema) {
  for (const propertyName of ['id', 'name', 'username', 'phone', 'email', 'wechatId']) {
    if (!schemaRequiredProperties(schema).has(propertyName)) {
      throw new Error(`CurrentUser.${propertyName} is missing required`);
    }
  }
}

function validateCoreResourceTimestampSchemas(allSchemas) {
  for (const schemaName of ['Account', 'Budget', 'Category', 'Transaction']) {
    const schema = allSchemas[schemaName] || '';
    for (const propertyName of ['createdAt', 'updatedAt']) {
      if (!schemaRequiredProperties(schema).has(propertyName)) {
        throw new Error(`${schemaName}.${propertyName} is missing required`);
      }
      const property = schemaPropertyBlock(schema, propertyName);
      if (!property.includes('type: string')) {
        throw new Error(`${schemaName}.${propertyName} is missing string schema`);
      }
      if (!property.includes('format: date-time')) {
        throw new Error(`${schemaName}.${propertyName} is missing date-time format`);
      }
    }
  }
}

function validatePaginatedResponseSchemasAreClosed(allSchemas) {
  for (const schemaName of ['TransactionListResponse', 'Pagination']) {
    if (!allSchemas[schemaName]?.includes('additionalProperties: false')) {
      throw new Error(`${schemaName} is missing additionalProperties: false`);
    }
  }
}

function validateReportResponseSchemasAreClosed(allSchemas) {
  const reportSchemas = [
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
  ];
  for (const schemaName of reportSchemas) {
    if (!allSchemas[schemaName]?.includes('additionalProperties: false')) {
      throw new Error(`${schemaName} is missing additionalProperties: false`);
    }
  }
}

function schemaRequiredProperties(schema) {
  return new Set(parseRequired(schema));
}

function validateSummaryResponseSchema(schema) {
  for (const propertyName of [
    'start',
    'end',
    'income',
    'expense',
    'balance',
    'byCategory',
    'byAccount',
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
    if (!schemaRequiredProperties(schema).has(propertyName)) {
      throw new Error(`Summary.${propertyName} is missing required`);
    }
  }
  for (const propertyName of ['start', 'end']) {
    const property = schemaPropertyBlock(schema, propertyName);
    if (!property.includes('type: string')) {
      throw new Error(`Summary.${propertyName} is missing string schema`);
    }
    if (!property.includes('format: date-time')) {
      throw new Error(`Summary.${propertyName} is missing date-time format`);
    }
  }
}

function validateImportResponseSchemasAreClosed(allSchemas) {
  for (const schemaName of ['ImportPreviewRow', 'ImportPreview', 'ImportResult', 'ImportJob']) {
    if (!allSchemas[schemaName]?.includes('additionalProperties: false')) {
      throw new Error(`${schemaName} is missing additionalProperties: false`);
    }
  }
}

function validateAIResponseSchemasAreClosed(allSchemas) {
  for (const schemaName of ['AIParseResult', 'AIParseResponse']) {
    if (!allSchemas[schemaName]?.includes('additionalProperties: false')) {
      throw new Error(`${schemaName} is missing additionalProperties: false`);
    }
  }
}

function validateAIResponseSchema(schema) {
  const confidence = schemaPropertyBlock(schema, 'confidence');
  if (!confidence.includes('type: number')) {
    throw new Error('AIParseResult.confidence is missing number schema');
  }
  if (!confidence.includes('minimum: 0')) {
    throw new Error('AIParseResult.confidence is missing minimum: 0');
  }
  if (!confidence.includes('maximum: 1')) {
    throw new Error('AIParseResult.confidence is missing maximum: 1');
  }
}

function validatePaginationSchema(schema) {
  const page = schemaPropertyBlock(schema, 'page');
  if (!page.includes('minimum: 1')) {
    throw new Error('Pagination.page is missing minimum: 1');
  }

  const pageSize = schemaPropertyBlock(schema, 'pageSize');
  if (!pageSize.includes('minimum: 1')) {
    throw new Error('Pagination.pageSize is missing minimum: 1');
  }
  if (!pageSize.includes('maximum: 200')) {
    throw new Error('Pagination.pageSize is missing maximum: 200');
  }

  const total = schemaPropertyBlock(schema, 'total');
  if (!total.includes('minimum: 0')) {
    throw new Error('Pagination.total is missing minimum: 0');
  }
}

function schemaPropertyBlock(schema, propertyName) {
  const pattern = new RegExp(`^        ${propertyName}:\\n(?:          .+\\n)+`, 'm');
  return schema.match(pattern)?.[0] || '';
}

function validateParameterConstraints(openapi) {
  const idParameter = openapi.match(/^    Id:\n(?:      .+\n)+/m)?.[0] || '';
  if (!idParameter.includes('minimum: 1')) {
    throw new Error('components.parameters.Id is missing minimum: 1');
  }

  const requestIDParameter = openapi.match(/^    RequestID:\n(?:      .+\n)+/m)?.[0] || '';
  validateRequestIDSchema(requestIDParameter, 'components.parameters.RequestID');
}

function validateResponseComponents(openapi) {
  validateNoStoreHeaders(openapi);
  validateErrorResponseExamples(openapi);

  const methodNotAllowed = openapi.match(/^    MethodNotAllowed:\n(?:      .+\n)+/m)?.[0] || '';
  if (!methodNotAllowed.includes('Allow:')) {
    throw new Error('components.responses.MethodNotAllowed is missing Allow header');
  }
  const allow = componentHeaderBlock(openapi, 'Allow');
  if (!allow.includes('Comma-separated HTTP methods supported') || !allow.includes('example: GET, POST')) {
    throw new Error('components.headers.Allow must document comma-separated supported methods with an example');
  }

  const unauthorized = openapi.match(/^    Unauthorized:\n(?:      .+\n)+/m)?.[0] || '';
  if (!unauthorized.includes('WWW-Authenticate:')) {
    throw new Error('components.responses.Unauthorized is missing WWW-Authenticate header');
  }
  const wwwAuthenticate = componentHeaderBlock(openapi, 'WWWAuthenticate');
  if (!wwwAuthenticate.includes('realm="haohao-accounting-api"')) {
    throw new Error('components.headers.WWWAuthenticate is missing bearer realm guidance');
  }
  if (!wwwAuthenticate.includes('invalid_token')) {
    throw new Error('components.headers.WWWAuthenticate is missing invalid_token guidance');
  }
  if (!wwwAuthenticate.includes('error_description')) {
    throw new Error('components.headers.WWWAuthenticate is missing error_description guidance');
  }
  if (!wwwAuthenticate.includes('example: Bearer realm="haohao-accounting-api"')) {
    throw new Error('components.headers.WWWAuthenticate is missing bearer challenge example');
  }

  const rateLimited = openapi.match(/^    RateLimited:\n(?:      .+\n)+/m)?.[0] || '';
  if (!rateLimited.includes('Retry-After:')) {
    throw new Error('components.responses.RateLimited is missing Retry-After header');
  }
  for (const headerName of ['RateLimit-Limit:', 'RateLimit-Remaining:', 'RateLimit-Reset:']) {
    if (!rateLimited.includes(headerName)) {
      throw new Error(`components.responses.RateLimited is missing ${headerName} header`);
    }
  }
  const retryAfter = componentHeaderBlock(openapi, 'RetryAfter');
  if (!retryAfter.includes('Remaining wait time') || !retryAfter.includes('HTTP-date') || !retryAfter.includes('non-negative integer delay-seconds') || !retryAfter.includes('type: string')) {
    throw new Error('components.headers.RetryAfter must document remaining wait time as HTTP-date or non-negative integer delay-seconds');
  }
  validateRateLimitHeader(openapi, 'RateLimitLimit', 'Maximum number of failed login attempts');
  validateRateLimitHeader(openapi, 'RateLimitRemaining', 'Remaining failed login attempts');
  validateRateLimitHeader(openapi, 'RateLimitReset', 'Delay in seconds');

  const importJobAccepted = operationResponseBlockById(openapi, 'postIoImportJobs', '202');
  if (!importJobAccepted.includes('Location:')) {
    throw new Error('POST /io/import/jobs 202 response is missing Location header');
  }
  const location = componentHeaderBlock(openapi, 'Location');
  if (!location.includes('Relative URL') || !location.includes('/api/v1/io/import/jobs/1')) {
    throw new Error('components.headers.Location must document queued resource URLs with an example');
  }

  const notAcceptable = openapi.match(/^    NotAcceptable:\n(?:      .+\n)+/m)?.[0] || '';
  if (!notAcceptable.includes('application/json:')) {
    throw new Error('components.responses.NotAcceptable is missing application/json error response');
  }
  if (!notAcceptable.includes('Vary:')) {
    throw new Error('components.responses.NotAcceptable is missing Vary header');
  }

  const vary = componentHeaderBlock(openapi, 'Vary');
  if (!vary.includes('type: string')) {
    throw new Error('components.headers.Vary is missing string schema');
  }
  if (!vary.includes('Accept')) {
    throw new Error('components.headers.Vary must document Accept negotiation');
  }
  if (!vary.includes('Origin')) {
    throw new Error('components.headers.Vary must document CORS origin variance');
  }
  if (!vary.includes('Access-Control-Request-Method') || !vary.includes('Access-Control-Request-Headers')) {
    throw new Error('components.headers.Vary must document CORS preflight variance');
  }
  if (!vary.includes('example: Accept, Origin, Access-Control-Request-Method, Access-Control-Request-Headers')) {
    throw new Error('components.headers.Vary is missing combined negotiation and CORS example');
  }

  const contentDisposition = componentHeaderBlock(openapi, 'ContentDisposition');
  if (!contentDisposition.includes('filename*')) {
    throw new Error('components.headers.ContentDisposition is missing filename* guidance');
  }

  const requestID = componentHeaderBlock(openapi, 'RequestID');
  validateRequestIDSchema(requestID, 'components.headers.RequestID');
}

function validateSecuritySchemes(openapi) {
  const schemes = nestedBlock(openAPIComponentsBlock(openapi), 'securitySchemes:');
  const bearerAuth = nestedBlock(schemes, 'bearerAuth:');
  if (!bearerAuth.includes('scheme: bearer') || !bearerAuth.includes('bearerFormat: JWT')) {
    throw new Error('components.securitySchemes.bearerAuth must document HTTP bearer JWT auth');
  }
  if (!bearerAuth.includes('Authorization: Bearer <JWT>')) {
    throw new Error('components.securitySchemes.bearerAuth is missing Authorization header guidance');
  }
}

function validateNoStoreHeaders(openapi) {
  const expectedHeaders = [
    ['CacheControl', 'Cache-Control:', 'no-store'],
    ['Pragma', 'Pragma:', 'no-cache'],
    ['Expires', 'Expires:', "'0'"],
  ];
  for (const [componentName, headerName, expectedValue] of expectedHeaders) {
    const headerBlock = componentHeaderBlock(openapi, componentName);
    if (!headerBlock.includes(expectedValue)) {
      throw new Error(`components.headers.${componentName} is missing ${expectedValue}`);
    }
    for (const responseName of noStoreResponseNames()) {
      const response = responseComponentBlock(openapi, responseName);
      if (!response.includes(headerName)) {
        throw new Error(`components.responses.${responseName} is missing ${headerName} no-store header`);
      }
    }
  }
}

function validateRateLimitHeader(openapi, componentName, descriptionText) {
  const header = componentHeaderBlock(openapi, componentName);
  if (!header.includes(descriptionText) || !header.includes('type: integer') || !header.includes('minimum: 0') && !header.includes('minimum: 1')) {
    throw new Error(`components.headers.${componentName} must document a bounded integer rate-limit value`);
  }
}

function validateErrorResponseExamples(openapi) {
  for (const responseName of errorResponseNames()) {
    const response = responseComponentBlock(openapi, responseName);
    if (!response.includes("$ref: '#/components/schemas/ErrorResponse'")) {
      throw new Error(`components.responses.${responseName} is missing ErrorResponse schema`);
    }
    const example = nestedBlock(nestedBlock(response, 'application/json:'), 'example:');
    for (const field of ['error:', 'code:', 'requestId:']) {
      if (!example.includes(field)) {
        throw new Error(`components.responses.${responseName} example is missing ${field}`);
      }
    }
  }
}

function errorResponseNames() {
  return [
    'BadRequest',
    'Unauthorized',
    'Forbidden',
    'NotFound',
    'MethodNotAllowed',
    'RateLimited',
    'PayloadTooLarge',
    'UnsupportedMediaType',
    'NotAcceptable',
    'InternalError',
    'GatewayTimeout',
    'Error',
  ];
}

function noStoreResponseNames() {
  return [
    'BadRequest',
    'Unauthorized',
    'Forbidden',
    'NotFound',
    'MethodNotAllowed',
    'RateLimited',
    'PayloadTooLarge',
    'UnsupportedMediaType',
    'NotAcceptable',
    'InternalError',
    'GatewayTimeout',
    'Error',
    'Ok',
  ];
}

function validateRequestIDSchema(block, name) {
  if (!block.includes('minLength: 1')) {
    throw new Error(`${name} is missing minLength: 1`);
  }
  if (!block.includes('maxLength: 128')) {
    throw new Error(`${name} is missing maxLength: 128`);
  }
  if (!block.includes("pattern: '^[!-~]+$'")) {
    throw new Error(`${name} is missing visible ASCII pattern`);
  }
  if (!block.includes('example: client-request-123')) {
    throw new Error(`${name} is missing request id example`);
  }
}

function emitType(name, block) {
  const enumValues = parseEnum(block);
  if (enumValues.length > 0) {
    return `export type ${name} = ${enumValues.map((value) => JSON.stringify(value)).join(' | ')};\n`;
  }

  const propertiesBlock = nestedBlock(block, 'properties:');
  if (!propertiesBlock) {
    return `export type ${name} = Record<string, unknown>;\n`;
  }

  const required = new Set(parseRequired(block));
  const fields = parseProperties(propertiesBlock).map(({ key, body }) => {
    const optional = required.has(key) ? '' : '?';
    return `  ${key}${optional}: ${propertyType(body)};`;
  });

  return `export type ${name} = {\n${fields.join('\n')}\n};\n`;
}

function emitClient(endpoints) {
  const groups = groupEndpoints(endpoints);
  const lines = [
    '// Generated from backend/api/openapi.yaml. Do not edit by hand.',
    "import type {",
    ...schemaNames.map((name) => `  ${name},`),
    "} from '../types/api';",
    '',
    'export type ApiRequest = <T>(path: string, init?: RequestInit) => Promise<T>;',
    'export type ApiUpload = <T>(path: string, formData: FormData) => Promise<T>;',
    '',
    'export type ApiRuntime = {',
    '  request: ApiRequest;',
    '  upload?: ApiUpload;',
    '};',
    '',
    'export function createApiClient(runtime: ApiRuntime) {',
    '  return {',
  ];

  for (const [group, items] of groups) {
    lines.push(`    ${group}: {`);
    for (const endpoint of items) {
      lines.push(...emitEndpointMethod(endpoint).map((line) => `      ${line}`));
    }
    lines.push('    },');
  }
  lines.push('  };');
  lines.push('}');
  lines.push('');
  lines.push('function setQueryParam(search: URLSearchParams, key: string, value: unknown) {');
  lines.push("  if (value === undefined || value === null || value === '') return;");
  lines.push('  search.set(key, String(value));');
  lines.push('}');
  lines.push('');
  return lines.join('\n');
}

function emitEndpointMethod(endpoint) {
  const pathParams = [...endpoint.path.matchAll(/\{([A-Za-z][A-Za-z0-9]*)\}/g)].map((match) => match[1]);
  const queryParams = endpoint.parameters.filter((param) => param.in === 'query');
  const paramsFields = [
    ...pathParams.map((name) => `  ${name}: number | string;`),
    ...queryParams.map((param) => `  ${param.name}?: ${parameterType(param)};`),
  ];
  const bodyType = endpoint.multipart ? 'FormData' : endpoint.requestSchema;
  const needsParams = paramsFields.length > 0;
  const needsBody = Boolean(bodyType);
  const args = [];
  if (needsParams) args.push(`params: {\n${paramsFields.join('\n')}\n}`);
  if (needsBody) args.push(`body: ${bodyType}`);
  const responseType = endpoint.responseSchema || 'unknown';
  const methodName = endpoint.name;
  const lines = [];
  const optionalParams = needsParams && pathParams.length === 0 && queryParams.every((param) => !param.required);

  lines.push(`${methodName}: (${methodArgs(args, optionalParams)}) => {`);
  if (needsParams) {
    lines.push(`  let path = ${JSON.stringify(endpoint.path)};`);
    for (const name of pathParams) {
      lines.push(`  path = path.replace('{${name}}', encodeURIComponent(String(params.${name})));`);
    }
  } else {
    lines.push(`  let path = ${JSON.stringify(endpoint.path)};`);
  }
  if (queryParams.length > 0) {
    lines.push('  const search = new URLSearchParams();');
    for (const param of queryParams) {
      lines.push(`  setQueryParam(search, '${param.name}', params?.${param.name});`);
    }
    lines.push("  const query = search.toString();");
    lines.push("  if (query) path += `?${query}`;");
  }
  if (endpoint.multipart) {
    lines.push('  if (!runtime.upload) throw new Error(\'upload runtime is required\');');
    lines.push(`  return runtime.upload<${responseType}>(path, body);`);
  } else {
    const initParts = [`method: '${endpoint.method.toUpperCase()}'`];
    if (needsBody && endpoint.method !== 'get') {
      initParts.push('body: JSON.stringify(body)');
    }
    if (endpoint.method === 'get' && !needsBody) {
      lines.push(`  return runtime.request<${responseType}>(path);`);
    } else {
      lines.push(`  return runtime.request<${responseType}>(path, { ${initParts.join(', ')} });`);
    }
  }
  lines.push('},');
  return lines;
}

function methodArgs(args, optionalParams) {
  if (!optionalParams) {
    return args.join(', ');
  }
  const [paramsArg, ...rest] = args;
  return [`${paramsArg} = {}`, ...rest].join(', ');
}

function parseEndpoints(openapi) {
  const pathsBlock = openapi.slice(openapi.indexOf('paths:'), openapi.indexOf('components:'));
  const pathMatches = [...pathsBlock.matchAll(/^  (\/[^:]+):$/gm)];
  const operationIds = new Map();
  const declaredTags = declaredOpenAPITags(openapi);
  const result = [];
  for (const [index, match] of pathMatches.entries()) {
    const apiPath = match[1];
    const start = match.index;
    const end = pathMatches[index + 1]?.index ?? pathsBlock.length;
    const pathBlock = pathsBlock.slice(start, end);
    const pathParameters = parseOperationParameters(pathLevelBlock(pathBlock));
    const methodMatches = [...pathBlock.matchAll(/^    (get|post|put|delete):$/gm)];
    for (const [methodIndex, methodMatch] of methodMatches.entries()) {
      const method = methodMatch[1];
      const methodStart = methodMatch.index;
      const methodEnd = methodMatches[methodIndex + 1]?.index ?? pathBlock.length;
      const methodBlock = pathBlock.slice(methodStart, methodEnd);
      const operationId = methodBlock.match(/operationId:\s+([A-Za-z][A-Za-z0-9]*)/)?.[1] || '';
      const summary = operationSummary(methodBlock);
      const description = operationDescription(methodBlock);
      const tags = operationTags(methodBlock);
      const responseStatuses = operationResponseStatuses(methodBlock);
      const parameters = [...pathParameters, ...parseOperationParameters(methodBlock)];
      if (!operationId) {
        throw new Error(`${method.toUpperCase()} ${apiPath} is missing operationId`);
      }
      if (!summary) {
        throw new Error(`${method.toUpperCase()} ${apiPath} is missing summary`);
      }
      if (!description) {
        throw new Error(`${method.toUpperCase()} ${apiPath} is missing description`);
      }
      if (operationIds.has(operationId)) {
        throw new Error(`${operationId} is used by both ${operationIds.get(operationId)} and ${method.toUpperCase()} ${apiPath}`);
      }
      operationIds.set(operationId, `${method.toUpperCase()} ${apiPath}`);
      if (tags.length === 0) {
        throw new Error(`${method.toUpperCase()} ${apiPath} is missing tags`);
      }
      for (const tag of tags) {
        if (!declaredTags.has(tag)) {
          throw new Error(`${method.toUpperCase()} ${apiPath} uses undeclared tag ${tag}`);
        }
      }
      if (![...responseStatuses].some((status) => /^2\d\d$/.test(status))) {
        throw new Error(`${method.toUpperCase()} ${apiPath} is missing 2xx success response`);
      }
      if (!isPublicOperation(methodBlock) && !responseStatuses.has('401')) {
        throw new Error(`${method.toUpperCase()} ${apiPath} is missing 401 response`);
      }
      if (!responseStatuses.has('405')) {
        throw new Error(`${method.toUpperCase()} ${apiPath} is missing 405 response`);
      }
      if (responseStatuses.has('500') && !responseStatuses.has('504')) {
        throw new Error(`${method.toUpperCase()} ${apiPath} declares 500 response but is missing 504 timeout response`);
      }
      if (operationHasSuccessContent(methodBlock, source) && !responseStatuses.has('406')) {
        throw new Error(`${method.toUpperCase()} ${apiPath} returns a response body but is missing 406 response`);
      }
      const jsonClientEndpoint = operationHasJSONSuccessContent(methodBlock, source);
      validateNegotiatedResponseHeaders(method, apiPath, methodBlock, responseStatuses, source);
      validateAuthOperationContract(method, apiPath, methodBlock, responseStatuses);
      validatePathParameters(method, apiPath, parameters);
      validateSuccessResponseHeaders(method, apiPath, methodBlock);
      validateSuccessResponseCacheHeaders(method, apiPath, methodBlock, source);
      validateOperationQueryContract(method, apiPath, methodBlock, responseStatuses);
      if (operationHasRequestBody(methodBlock)) {
        validateRequestBodyContract(method, apiPath, methodBlock);
        if (!responseStatuses.has('400')) {
          throw new Error(`${method.toUpperCase()} ${apiPath} is missing 400 response`);
        }
        if (!responseStatuses.has('413')) {
          throw new Error(`${method.toUpperCase()} ${apiPath} is missing 413 response`);
        }
        if (!responseStatuses.has('415')) {
          throw new Error(`${method.toUpperCase()} ${apiPath} is missing 415 response`);
        }
      }
      result.push({
        path: apiPath,
        method,
        name: operationId,
        tags,
        requestSchema: requestSchema(methodBlock),
        responseSchema: responseSchema(methodBlock),
        multipart: methodBlock.includes('multipart/form-data'),
        parameters,
        jsonClientEndpoint,
      });
    }
  }
  return result.filter((endpoint) => endpoint.jsonClientEndpoint);
}

function validateRequestBodyContract(method, apiPath, methodBlock) {
  const requestBlock = nestedBlock(methodBlock, 'requestBody:');
  if (!requestBlock.match(/^\s+description:\s+\S.+$/m)) {
    throw new Error(`${method.toUpperCase()} ${apiPath} requestBody is missing description`);
  }
  if (!requestBlock.includes('required: true')) {
    throw new Error(`${method.toUpperCase()} ${apiPath} requestBody must be required`);
  }

  const hasJSON = requestBlock.includes('application/json:');
  const hasMultipart = requestBlock.includes('multipart/form-data:');
  if (!hasJSON && !hasMultipart) {
    throw new Error(`${method.toUpperCase()} ${apiPath} requestBody must use application/json or multipart/form-data`);
  }
  if (hasJSON && hasMultipart) {
    throw new Error(`${method.toUpperCase()} ${apiPath} requestBody must not mix JSON and multipart content`);
  }
  if (!requestSchema(methodBlock)) {
    throw new Error(`${method.toUpperCase()} ${apiPath} requestBody is missing a component schema reference`);
  }
}

function operationSummary(block) {
  return block.match(/^\s+summary:\s+(.+)$/m)?.[1]?.trim() || '';
}

function operationDescription(block) {
  return block.match(/^\s+description:\s+(.+)$/m)?.[1]?.trim() || '';
}

function validateAuthOperationContract(method, apiPath, methodBlock, responseStatuses) {
  const operation = `${method.toUpperCase()} ${apiPath}`;
  const publicOperations = new Set(['POST /auth/login']);
  if (publicOperations.has(operation)) {
    if (!isPublicOperation(methodBlock)) {
      throw new Error(`${operation} must explicitly declare security: []`);
    }
  } else {
    if (isPublicOperation(methodBlock)) {
      throw new Error(`${operation} must require bearer authentication`);
    }
    if (!responseStatuses.has('401')) {
      throw new Error(`${operation} must document 401 for bearer authentication`);
    }
  }
  if (apiPath === '/auth/login' && method === 'post' && !responseStatuses.has('429')) {
    throw new Error('POST /auth/login is missing 429 rate limited response');
  }
}

function validatePathParameters(method, apiPath, parameters) {
  const templateParams = [...apiPath.matchAll(/\{([A-Za-z][A-Za-z0-9]*)\}/g)].map((match) => match[1]);
  for (const name of templateParams) {
    if (!parameters.some((param) => param.name === name && param.in === 'path')) {
      throw new Error(`${method.toUpperCase()} ${apiPath} is missing path parameter declaration for ${name}`);
    }
  }

  const templateParamSet = new Set(templateParams);
  for (const param of parameters.filter((item) => item.in === 'path')) {
    if (!templateParamSet.has(param.name)) {
      throw new Error(`${method.toUpperCase()} ${apiPath} declares path parameter ${param.name} that is not present in path template`);
    }
    if (!param.required) {
      throw new Error(`${method.toUpperCase()} ${apiPath} path parameter ${param.name} must be required`);
    }
  }
}

function validateSuccessResponseHeaders(method, apiPath, methodBlock) {
  const successResponses = operationResponseBlocks(methodBlock).filter((response) => /^2\d\d$/.test(response.status));
  for (const response of successResponses) {
    if (response.block.includes("$ref: '#/components/responses/Ok'")) continue;
    if (response.block.includes('X-Request-ID:')) continue;
    throw new Error(`${method.toUpperCase()} ${apiPath} ${response.status} response is missing X-Request-ID header`);
  }
}

function validateSuccessResponseCacheHeaders(method, apiPath, methodBlock, openapi) {
  const successResponses = operationResponseBlocks(methodBlock).filter((response) => /^2\d\d$/.test(response.status));
  for (const response of successResponses) {
    for (const header of ['Cache-Control:', 'Pragma:', 'Expires:']) {
      if (responseIncludesHeader(response.block, openapi, header)) continue;
      throw new Error(`${method.toUpperCase()} ${apiPath} ${response.status} response is missing ${header} no-store header`);
    }
  }
}

function validateNegotiatedResponseHeaders(method, apiPath, methodBlock, responseStatuses, openapi) {
  if (!responseStatuses.has('406')) return;

  const successResponses = operationResponseBlocks(methodBlock).filter((response) => /^2\d\d$/.test(response.status));
  for (const response of successResponses) {
    if (responseIncludesHeader(response.block, openapi, 'Vary:')) continue;
    throw new Error(`${method.toUpperCase()} ${apiPath} ${response.status} negotiated response is missing Vary header`);
  }
}

function responseIncludesHeader(responseBlock, openapi, headerText) {
  if (responseBlock.includes(headerText)) return true;
  const componentName = responseBlock.match(/\$ref:\s+'#\/components\/responses\/([^']+)'/)?.[1] || '';
  return componentName ? responseComponentBlock(openapi, componentName).includes(headerText) : false;
}

function validateOperationQueryContract(method, apiPath, methodBlock, responseStatuses) {
  const parameters = parseOperationParameters(methodBlock).filter((param) => param.in === 'query');
  if (parameters.length === 0) return;

  if (apiPath === '/transactions' && method === 'get') {
    require400Response(method, apiPath, responseStatuses);
    requireParameterText(method, apiPath, methodBlock, 'page', 'minimum: 1');
    requireParameterText(method, apiPath, methodBlock, 'page', 'default: 1');
    requireParameterText(method, apiPath, methodBlock, 'page', 'Defaults to 1 when omitted');
    requireParameterText(method, apiPath, methodBlock, 'page', 'example: 1');
    requireParameterText(method, apiPath, methodBlock, 'pageSize', 'minimum: 1');
    requireParameterText(method, apiPath, methodBlock, 'pageSize', 'maximum: 200');
    requireParameterText(method, apiPath, methodBlock, 'pageSize', 'default: 20');
    requireParameterText(method, apiPath, methodBlock, 'pageSize', 'Defaults to 20 when omitted');
    requireParameterText(method, apiPath, methodBlock, 'pageSize', 'example: 20');
    requireDateTimeQueryParameter(method, apiPath, methodBlock, 'start');
    requireDateTimeQueryParameter(method, apiPath, methodBlock, 'end');
    requireParameterText(method, apiPath, methodBlock, 'type', 'Transaction type filter');
    requireParameterText(method, apiPath, methodBlock, 'type', 'example: expense');
    requireParameterText(method, apiPath, methodBlock, 'categoryId', 'minimum: 1');
    requireParameterText(method, apiPath, methodBlock, 'categoryId', 'Category id filter');
    requireParameterText(method, apiPath, methodBlock, 'categoryId', 'example: 1');
    requireParameterText(method, apiPath, methodBlock, 'accountId', 'minimum: 1');
    requireParameterText(method, apiPath, methodBlock, 'accountId', 'Account id filter');
    requireParameterText(method, apiPath, methodBlock, 'accountId', 'example: 1');
    requireParameterText(method, apiPath, methodBlock, 'q', 'matched against transaction notes and tags');
    requireParameterText(method, apiPath, methodBlock, 'q', 'Maximum 100 characters');
    requireParameterText(method, apiPath, methodBlock, 'q', 'maxLength: 100');
    requireParameterText(method, apiPath, methodBlock, 'q', 'example: lunch');
  }
  if (apiPath === '/budgets' && method === 'get') {
    require400Response(method, apiPath, responseStatuses);
    requireParameterText(method, apiPath, methodBlock, 'month', "pattern: '^\\d{4}-\\d{2}$'");
    requireParameterText(method, apiPath, methodBlock, 'month', 'Budget month in YYYY-MM format');
    requireParameterText(method, apiPath, methodBlock, 'month', "example: '2026-06'");
  }
  if (apiPath === '/categories' && method === 'get') {
    require400Response(method, apiPath, responseStatuses);
    requireParameterText(method, apiPath, methodBlock, 'type', "$ref: '#/components/schemas/TransactionType'");
    requireParameterText(method, apiPath, methodBlock, 'type', 'Transaction type filter');
    requireParameterText(method, apiPath, methodBlock, 'type', 'example: expense');
  }
  if (apiPath === '/reports/summary' && method === 'get') {
    require400Response(method, apiPath, responseStatuses);
    requireDateTimeQueryParameter(method, apiPath, methodBlock, 'start');
    requireDateTimeQueryParameter(method, apiPath, methodBlock, 'end');
    requireParameterText(method, apiPath, methodBlock, 'categoryId', 'minimum: 1');
    requireParameterText(method, apiPath, methodBlock, 'categoryId', 'Category id filter');
    requireParameterText(method, apiPath, methodBlock, 'categoryId', 'example: 1');
    requireParameterText(method, apiPath, methodBlock, 'accountId', 'minimum: 1');
    requireParameterText(method, apiPath, methodBlock, 'accountId', 'Account id filter');
    requireParameterText(method, apiPath, methodBlock, 'accountId', 'example: 1');
    requireParameterText(method, apiPath, methodBlock, 'trend', 'enum: [day, week, month]');
    requireParameterText(method, apiPath, methodBlock, 'trend', 'default: month');
    requireParameterText(method, apiPath, methodBlock, 'trend', 'Defaults to month when omitted');
    requireParameterText(method, apiPath, methodBlock, 'trend', 'example: month');
  }
  if (apiPath === '/io/export' && method === 'get') {
    require400Response(method, apiPath, responseStatuses);
    requireParameterText(method, apiPath, methodBlock, 'format', 'enum: [csv, xlsx]');
    requireParameterText(method, apiPath, methodBlock, 'format', 'default: csv');
    requireParameterText(method, apiPath, methodBlock, 'format', 'Defaults to csv when omitted');
    requireParameterText(method, apiPath, methodBlock, 'format', 'example: csv');
    requireDateTimeQueryParameter(method, apiPath, methodBlock, 'start');
    requireDateTimeQueryParameter(method, apiPath, methodBlock, 'end');
    if (!methodBlock.includes('Content-Disposition:')) {
      throw new Error('GET /io/export is missing Content-Disposition response header');
    }
  }
}

function require400Response(method, apiPath, responseStatuses) {
  if (!responseStatuses.has('400')) {
    throw new Error(`${method.toUpperCase()} ${apiPath} has validated query parameters but is missing 400 response`);
  }
}

function requireParameterText(method, apiPath, block, name, requiredText) {
  const parameterBlock = parameterBlockByName(block, name);
  if (!parameterBlock.includes(requiredText)) {
    throw new Error(`${method.toUpperCase()} ${apiPath} parameter ${name} is missing ${requiredText}`);
  }
}

function requireDateTimeQueryParameter(method, apiPath, block, name) {
  requireParameterText(method, apiPath, block, name, 'format: date-time');
  for (const acceptedFormat of ['RFC3339', 'YYYY-MM-DD', 'YYYY-MM-DD HH:mm:ss', 'YYYY/MM/DD']) {
    requireParameterText(method, apiPath, block, name, acceptedFormat);
  }
  requireParameterText(method, apiPath, block, name, name === 'end' ? "example: '2026-06-30'" : "example: '2026-06-01T00:00:00+08:00'");
  if (name === 'end') {
    requireParameterText(method, apiPath, block, name, 'Date-only values cover the entire day');
  }
}

function parameterBlockByName(block, name) {
  const parametersBlock = nestedBlock(block, 'parameters:');
  const paramBlocks = parametersBlock.split(/\n(?=\s+- name:|\s+- \$ref:)/).map((item) => item.trim()).filter(Boolean);
  return paramBlocks.find((paramBlock) => paramBlock.includes(`name: ${name}`)) || '';
}

function operationResponseStatuses(block) {
  return new Set(operationResponseBlocks(block).map((response) => response.status));
}

function operationHasSuccessContent(block, openapi) {
  return operationResponseBlocks(block).some((response) => {
    if (!/^2\d\d$/.test(response.status)) return false;
    if (response.block.includes('content:')) return true;
    const componentName = response.block.match(/\$ref:\s+'#\/components\/responses\/([^']+)'/)?.[1] || '';
    return componentName ? responseComponentBlock(openapi, componentName).includes('content:') : false;
  });
}

function operationHasJSONSuccessContent(block, openapi) {
  return operationResponseBlocks(block).some((response) => {
    if (!/^2\d\d$/.test(response.status)) return false;
    const responseBlock = resolvedResponseBlock(response.block, openapi);
    return responseBlock.includes('application/json:');
  });
}

function resolvedResponseBlock(block, openapi) {
  const componentName = block.match(/\$ref:\s+'#\/components\/responses\/([^']+)'/)?.[1] || '';
  return componentName ? responseComponentBlock(openapi, componentName) : block;
}

function responseComponentBlock(openapi, componentName) {
  const responsesBlock = nestedBlock(openAPIComponentsBlock(openapi), 'responses:');
  const marker = `    ${componentName}:`;
  const start = responsesBlock.indexOf(marker);
  if (start === -1) return '';
  const next = responsesBlock.slice(start + marker.length).search(/^    [A-Za-z][A-Za-z0-9]*:/m);
  return responsesBlock.slice(start, next === -1 ? undefined : start + marker.length + next);
}

function componentHeaderBlock(openapi, componentName) {
  const headersBlock = nestedBlock(openAPIComponentsBlock(openapi), 'headers:');
  const marker = `    ${componentName}:`;
  const start = headersBlock.indexOf(marker);
  if (start === -1) return '';
  const next = headersBlock.slice(start + marker.length).search(/^    [A-Za-z][A-Za-z0-9]*:/m);
  return headersBlock.slice(start, next === -1 ? undefined : start + marker.length + next);
}

function openAPIComponentsBlock(openapi) {
  const start = openapi.indexOf('components:\n');
  return start === -1 ? '' : openapi.slice(start);
}

function topLevelBlock(openapi, key) {
  const marker = `${key}:\n`;
  const start = openapi.indexOf(marker);
  if (start === -1) return '';
  const rest = openapi.slice(start + marker.length);
  const next = rest.search(/^[A-Za-z][A-Za-z0-9_-]*:\n/m);
  return marker + rest.slice(0, next === -1 ? rest.length : next);
}

function declaredOpenAPITags(openapi) {
  const tagsBlock = topLevelBlock(openapi, 'tags');
  const tagBlocks = tagsBlock.split(/\n(?=  - name: )/).filter((block) => block.includes('- name:'));
  const tags = new Set();
  for (const tagBlock of tagBlocks) {
    const name = tagBlock.match(/^\s+- name:\s+([A-Za-z][A-Za-z0-9_-]*)/m)?.[1] || '';
    if (!name) continue;
    if (!tagBlock.match(/^\s+description:\s+\S.+$/m)) {
      throw new Error(`OpenAPI tag ${name} is missing description`);
    }
    tags.add(name);
  }
  return tags;
}

function operationResponseBlocks(block) {
  const responsesBlock = nestedBlock(block, 'responses:');
  const matches = [...responsesBlock.matchAll(/^        '?(\d{3}|default)'?:/gm)];
  return matches.map((match, index) => {
    const next = matches[index + 1];
    return {
      status: match[1],
      block: responsesBlock.slice(match.index, next?.index),
    };
  });
}

function operationResponseBlockById(openapi, operationId, status) {
  const operation = operationBlockById(openapi, operationId);
  return operationResponseBlocks(operation).find((response) => response.status === status)?.block || '';
}

function operationBlockById(openapi, operationId) {
  const pathsBlock = openapi.slice(openapi.indexOf('paths:'), openapi.indexOf('components:'));
  const pathMatches = [...pathsBlock.matchAll(/^  (\/[^:]+):$/gm)];
  for (const [pathIndex, pathMatch] of pathMatches.entries()) {
    const pathStart = pathMatch.index;
    const pathEnd = pathMatches[pathIndex + 1]?.index ?? pathsBlock.length;
    const pathBlock = pathsBlock.slice(pathStart, pathEnd);
    const methodMatches = [...pathBlock.matchAll(/^    (get|post|put|delete):$/gm)];
    for (const [methodIndex, methodMatch] of methodMatches.entries()) {
      const methodStart = methodMatch.index;
      const methodEnd = methodMatches[methodIndex + 1]?.index ?? pathBlock.length;
      const methodBlock = pathBlock.slice(methodStart, methodEnd);
      if (methodBlock.includes(`operationId: ${operationId}`)) {
        return methodBlock;
      }
    }
  }
  return '';
}

function isPublicOperation(block) {
  if (/^\s+security:\s*\[\]\s*$/m.test(block)) return true;
  const securityBlock = nestedBlock(block, 'security:');
  return securityBlock.split('\n').some((line) => line.trim() === '[]');
}

function operationTags(block) {
  const inline = block.match(/tags:\s+\[([^\]]+)\]/)?.[1];
  if (inline) return inline.split(',').map((value) => value.trim()).filter(Boolean);

  const tagsBlock = nestedBlock(block, 'tags:');
  if (!tagsBlock) return [];
  return tagsBlock
    .split('\n')
    .map((line) => line.trim())
    .filter((line) => line.startsWith('- '))
    .map((line) => line.slice(2).trim());
}

function pathLevelBlock(pathBlock) {
  const firstMethodIndex = pathBlock.search(/^    (get|post|put|delete):/m);
  return firstMethodIndex === -1 ? pathBlock : pathBlock.slice(0, firstMethodIndex);
}

function parseOperationParameters(block) {
  const parametersBlock = nestedBlock(block, 'parameters:');
  if (!parametersBlock) return [];

  const paramBlocks = parametersBlock.split(/\n(?=\s+- name:|\s+- \$ref:)/).map((item) => item.trim()).filter(Boolean);
  return paramBlocks.map((paramBlock) => {
    if (paramBlock.includes("$ref: '#/components/parameters/Id'")) {
      return { name: 'id', in: 'path', type: 'integer', required: true };
    }
    return {
      name: paramBlock.match(/name:\s+([A-Za-z][A-Za-z0-9]*)/)?.[1] || '',
      in: paramBlock.match(/in:\s+([A-Za-z]+)/)?.[1] || 'query',
      type: paramBlock.match(/type:\s+([A-Za-z]+)/)?.[1] || 'string',
      enumValues: enumValues(paramBlock),
      ref: paramBlock.match(/\$ref:\s+'#\/components\/schemas\/([^']+)'/)?.[1] || '',
      required: paramBlock.includes('required: true'),
    };
  }).filter((param) => param.name);
}

function requestSchema(block) {
  const requestBlock = nestedBlock(block, 'requestBody:');
  if (!requestBlock) return '';
  return requestBlock.match(/\$ref:\s+'#\/components\/schemas\/([^']+)'/)?.[1] || '';
}

function operationHasRequestBody(block) {
  return nestedBlock(block, 'requestBody:') !== '';
}

function responseSchema(block) {
  const responseBlock = nestedBlock(block, 'responses:');
  if (!responseBlock) return 'unknown';
  const arrayRef = responseBlock.match(/schema:\n\s+type:\s+array\n\s+items:\n\s+\$ref:\s+'#\/components\/schemas\/([^']+)'/)?.[1];
  if (arrayRef) return `${arrayRef}[]`;
  const ref = responseBlock.match(/schema:\n\s+\$ref:\s+'#\/components\/schemas\/([^']+)'/)?.[1];
  if (ref) return ref;
  if (responseBlock.includes('$ref: \'#/components/responses/Ok\'')) return 'OkResponse';
  return 'unknown';
}

function endpointMethodName(method, apiPath) {
  const cleanPath = apiPath.replace(/^\//, '').replace(/\{id\}/g, 'by-id');
  const words = cleanPath.split(/[\/-]/).filter(Boolean);
  const base = words.map((word, index) => index === 0 ? word : upperFirst(word)).join('');
  const prefix = method === 'get' ? 'get' : method === 'post' ? 'post' : method === 'put' ? 'put' : 'delete';
  return `${prefix}${upperFirst(base)}`;
}

function groupEndpoints(endpoints) {
  const groups = new Map();
  for (const endpoint of endpoints) {
    const first = endpoint.path.split('/').filter(Boolean)[0] || 'root';
    const group = first === 'io' ? 'dataio' : first.replace(/[^A-Za-z0-9]/g, '');
    if (!groups.has(group)) groups.set(group, []);
    groups.get(group).push(endpoint);
  }
  return [...groups.entries()];
}

function parameterType(param) {
  if (param.ref) return param.ref;
  if (param.enumValues?.length) return param.enumValues.map((value) => JSON.stringify(value)).join(' | ');
  return primitiveType(param.type);
}

function enumValues(block) {
  const match = block.match(/enum:\s+\[([^\]]+)\]/);
  if (!match) return [];
  return match[1].split(',').map((value) => value.trim()).filter(Boolean);
}

function upperFirst(value) {
  return value.charAt(0).toUpperCase() + value.slice(1);
}

function parseEnum(block) {
  const match = block.match(/^      enum:\s+\[([^\]]+)\]/m);
  if (!match) return [];
  return match[1].split(',').map((value) => value.trim()).filter(Boolean);
}

function parseRequired(block) {
  const singleLine = block.match(/^\s+required:\s+\[([^\]]+)\]/m);
  if (singleLine) {
    return singleLine[1].split(',').map((value) => value.trim()).filter(Boolean);
  }

  const requiredBlock = nestedBlock(block, 'required:');
  if (!requiredBlock) return [];
  return requiredBlock
    .split('\n')
    .map((line) => line.trim())
    .filter((line) => line.startsWith('- '))
    .map((line) => line.slice(2).trim());
}

function parseProperties(block) {
  const result = [];
  const matches = [...block.matchAll(/^        ([A-Za-z][A-Za-z0-9]*):$/gm)];
  for (const [index, match] of matches.entries()) {
    const key = match[1];
    const start = match.index;
    const end = matches[index + 1]?.index ?? block.length;
    result.push({ key, body: block.slice(start, end) });
  }
  return result;
}

function propertyType(body) {
  const ref = body.match(/^          \$ref:\s+'#\/components\/schemas\/([^']+)'/m)?.[1];
  if (ref) return ref;

  const type = body.match(/^          type:\s+([A-Za-z]+)/m)?.[1];
  const propertyEnumValues = enumValues(body);
  if (type === 'string' && propertyEnumValues.length > 0) {
    return propertyEnumValues.map((value) => JSON.stringify(value)).join(' | ');
  }
  if (type === 'array') {
    const itemRef = body.match(/^          items:\n\s+\$ref:\s+'#\/components\/schemas\/([^']+)'/m)?.[1];
    if (itemRef) return `${itemRef}[]`;
    const itemType = body.match(/^          items:\n\s+type:\s+([A-Za-z]+)/m)?.[1];
    return `${primitiveType(itemType || 'unknown')}[]`;
  }
  if (type === 'string' && body.includes('format: binary')) return 'unknown';
  return primitiveType(type || 'unknown');
}

function primitiveType(type) {
  switch (type) {
    case 'integer':
    case 'number':
      return 'number';
    case 'boolean':
      return 'boolean';
    case 'string':
      return 'string';
    case 'object':
      return 'Record<string, unknown>';
    default:
      return 'unknown';
  }
}

function nestedBlock(block, key) {
  const pattern = new RegExp(`^\\s+${escapeRegExp(key)}$`, 'm');
  const match = block.match(pattern);
  if (!match || match.index === undefined) return '';

  const lines = block.slice(match.index).split('\n');
  const firstIndent = leadingSpaces(lines[0]);
  const selected = [];
  for (const line of lines.slice(1)) {
    if (line.trim() !== '' && leadingSpaces(line) <= firstIndent) break;
    selected.push(line);
  }
  return selected.join('\n');
}

function leadingSpaces(value) {
  return value.match(/^ */)?.[0].length ?? 0;
}

function escapeRegExp(value) {
  return value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}
