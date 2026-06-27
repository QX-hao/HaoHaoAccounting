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
// OpenAPI 3.x Path Item 的标准操作方法都要扫描，避免新增接口时被生成器静默跳过。
const openAPIHTTPMethods = ['get', 'put', 'post', 'delete', 'options', 'head', 'patch', 'trace'];
const openAPIHTTPMethodPattern = openAPIHTTPMethods.join('|');

const source = fs.readFileSync(openapiPath, 'utf8');
const schemasSource = source.slice(source.indexOf('  schemas:'));
const schemaNames = [...schemasSource.matchAll(/^    ([A-Za-z][A-Za-z0-9]*):$/gm)].map((match) => match[1]);
const duplicateSchemaNames = schemaNames.filter((name, index) => schemaNames.indexOf(name) !== index);
if (duplicateSchemaNames.length > 0) {
  throw new Error(`Duplicate schemas: ${[...new Set(duplicateSchemaNames)].join(', ')}`);
}

// 生成前先把 OpenAPI 当作接口契约做静态校验，避免错误规范继续生成到前端客户端。
const schemas = Object.fromEntries(schemaNames.map((name, index) => {
  const marker = `    ${name}:`;
  const start = schemasSource.indexOf(marker);
  const nextName = schemaNames[index + 1];
  const end = nextName ? schemasSource.indexOf(`    ${nextName}:`, start + marker.length) : schemasSource.length;
  return [name, schemasSource.slice(start, end)];
}));

validateOpenAPIDescription(source);
validateOpenAPIInfoContact(source);
validateOpenAPIServers(source);
validateOpenAPIExternalDocs(source);
validateOpenAPITags(source);
validateSchemaRefs(source, new Set(schemaNames));
validateSchemaConstraints(schemas);
validateParameterConstraints(source);
validateResponseComponents(source);
validateSecuritySchemes(source);

// 类型文件只从 components.schemas 生成，保证 Web 和 Mobile 使用同一份 API 数据结构。
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

// 客户端方法从 paths 解析，并在解析阶段继续校验 operationId、认证、响应头和请求体契约。
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

function validateOpenAPIInfoContact(openapi) {
  const info = topLevelBlock(openapi, 'info');
  const contact = nestedBlock(info, 'contact:');
  if (!contact.includes('name: HaoHaoAccounting Maintainers')) {
    throw new Error('OpenAPI info.contact must identify HaoHaoAccounting maintainers');
  }
  if (!contact.includes('url: https://github.com/QX-hao/HaoHaoAccounting/security')) {
    throw new Error('OpenAPI info.contact must link to the repository security contact path');
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

function validateOpenAPIExternalDocs(openapi) {
  const externalDocs = topLevelBlock(openapi, 'externalDocs');
  if (!externalDocs.includes('description: Repository API contract and verification workflow.')) {
    throw new Error('OpenAPI externalDocs must describe the repository API contract workflow');
  }
  if (!externalDocs.includes('url: https://github.com/QX-hao/HaoHaoAccounting/tree/dev-pxhao#协作与安全')) {
    throw new Error('OpenAPI externalDocs must link to repository API contract documentation');
  }
}

function validateOpenAPITags(openapi) {
  const tags = declaredOpenAPITags(openapi);
  if (tags.size === 0) {
    throw new Error('OpenAPI top-level tags must declare API groups');
  }
}

// 防止 OpenAPI 引用不存在的 schema，避免前端生成出无法落地的类型。
function validateSchemaRefs(openapi, knownSchemas) {
  for (const match of openapi.matchAll(/\$ref:\s+'#\/components\/schemas\/([^']+)'/g)) {
    const schemaName = match[1];
    if (!knownSchemas.has(schemaName)) {
      throw new Error(`OpenAPI references unknown schema component ${schemaName}`);
    }
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
    ['ImportTextRequest', 'skipDuplicates defaults to true'],
    ['ImportFileRequest', 'occurred_at,type,amount,category,account,note,tags'],
    ['ImportFileRequest', 'UTF-8 BOM'],
    ['ImportFileRequest', 'skipDuplicates defaults to true'],
    ['ImportFileRequest', 'invalid values return invalid_request'],
  ];
  for (const [schemaName, requiredText] of checks) {
    if (!allSchemas[schemaName]?.includes(requiredText)) {
      throw new Error(`${schemaName} is missing ${requiredText}`);
    }
  }

  validateSchemaRequiredProperties(allSchemas);
  validateRequestSchemasAreClosed(allSchemas);
  validateSharedResponseSchemasAreClosed(allSchemas);
  validateErrorResponseSchema(allSchemas.ErrorResponse || '');
  validateCoreResponseSchemasAreClosed(allSchemas);
  validateAuthSchemaFieldDirection(allSchemas);
  validateCurrentUserResponseSchema(allSchemas.CurrentUser || '');
  validateCoreResourceReadOnlyFields(allSchemas);
  validateCoreResourceTimestampSchemas(allSchemas);
  validatePaginatedResponseSchemasAreClosed(allSchemas);
  validateReportResponseSchemasAreClosed(allSchemas);
  validateReportResponseBounds(allSchemas);
  validateReportResponseExamples(allSchemas);
  validateSummaryResponseSchema(allSchemas.Summary || '');
  validateImportResponseSchemasAreClosed(allSchemas);
  validateImportResponseBounds(allSchemas);
  validateImportResponseExamples(allSchemas);
  validateImportJobReadOnlyFields(allSchemas.ImportJob || '');
  validateAIResponseSchemasAreClosed(allSchemas);
  validateAIResponseSchema(allSchemas.AIParseResult || '');
  validatePaginationSchema(allSchemas.Pagination || '');
}

// required 只能声明 properties 中真实存在的字段，否则客户端会相信一个不存在的必填字段。
function validateSchemaRequiredProperties(allSchemas) {
  for (const [schemaName, schema] of Object.entries(allSchemas)) {
    const required = parseRequired(schema);
    if (required.length === 0) continue;

    const declaredProperties = new Set(parseProperties(nestedBlock(schema, 'properties:')).map((property) => property.key));
    for (const propertyName of required) {
      if (!declaredProperties.has(propertyName)) {
        throw new Error(`${schemaName}.required references missing property ${propertyName}`);
      }
    }
  }
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

function validateAuthSchemaFieldDirection(allSchemas) {
  const password = schemaPropertyBlock(allSchemas.LoginRequest || '', 'password');
  if (!password.includes('writeOnly: true')) {
    throw new Error('LoginRequest.password is missing writeOnly: true');
  }
  if (!password.includes('format: password')) {
    throw new Error('LoginRequest.password is missing password format');
  }

  const token = schemaPropertyBlock(allSchemas.LoginResponse || '', 'token');
  if (!token.includes('readOnly: true')) {
    throw new Error('LoginResponse.token is missing readOnly: true');
  }
}

function validateErrorResponseSchema(schema) {
  for (const propertyName of ['error', 'code', 'status', 'requestId']) {
    if (!schemaRequiredProperties(schema).has(propertyName)) {
      throw new Error(`ErrorResponse.${propertyName} is missing required`);
    }
  }
  const status = schemaPropertyBlock(schema, 'status');
  if (!status.includes('type: integer')) {
    throw new Error('ErrorResponse.status is missing integer schema');
  }
  if (!status.includes('minimum: 400')) {
    throw new Error('ErrorResponse.status is missing minimum: 400');
  }
  if (!status.includes('maximum: 599')) {
    throw new Error('ErrorResponse.status is missing maximum: 599');
  }
}

function validateCurrentUserResponseSchema(schema) {
  for (const propertyName of ['id', 'name', 'username', 'phone', 'email', 'wechatId']) {
    if (!schemaRequiredProperties(schema).has(propertyName)) {
      throw new Error(`CurrentUser.${propertyName} is missing required`);
    }
  }
  const id = schemaPropertyBlock(schema, 'id');
  if (!id.includes('readOnly: true')) {
    throw new Error('CurrentUser.id is missing readOnly: true');
  }
}

function validateCoreResourceReadOnlyFields(allSchemas) {
  const fieldsBySchema = {
    Account: ['id', 'userId', 'createdAt', 'updatedAt'],
    Budget: ['id', 'userId', 'createdAt', 'updatedAt'],
    Category: ['id', 'userId', 'createdAt', 'updatedAt'],
    Transaction: ['id', 'userId', 'createdAt', 'updatedAt'],
  };

  for (const [schemaName, propertyNames] of Object.entries(fieldsBySchema)) {
    const schema = allSchemas[schemaName] || '';
    for (const propertyName of propertyNames) {
      const property = schemaPropertyBlock(schema, propertyName);
      if (!property.includes('readOnly: true')) {
        throw new Error(`${schemaName}.${propertyName} is missing readOnly: true`);
      }
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

function validateReportResponseBounds(allSchemas) {
  const minimumsBySchema = {
    CategoryStat: { categoryId: 1, amount: 0 },
    AccountStat: { accountId: 1, amount: 0 },
    MonthTrend: { income: 0, expense: 0 },
    TrendPoint: { income: 0, expense: 0 },
    CategoryTrendPoint: { categoryId: 1, amount: 0 },
    AccountBalancePoint: { accountId: 1 },
    BudgetExecution: { budgetId: 1, categoryId: 0, budget: 0, expense: 0, usageRate: 0 },
    SummaryTableRow: { income: 0, expense: 0, txCount: 0 },
    PeriodTotals: { income: 0, expense: 0 },
    Summary: { income: 0, expense: 0 },
  };
  for (const [schemaName, propertyMinimums] of Object.entries(minimumsBySchema)) {
    const schema = allSchemas[schemaName] || '';
    for (const [propertyName, minimum] of Object.entries(propertyMinimums)) {
      const property = schemaPropertyBlock(schema, propertyName);
      if (!property.includes(`minimum: ${minimum}`)) {
        throw new Error(`${schemaName}.${propertyName} is missing minimum: ${minimum}`);
      }
    }
  }

  const moneyFieldsBySchema = {
    CategoryStat: ['amount'],
    AccountStat: ['amount'],
    MonthTrend: ['income', 'expense'],
    TrendPoint: ['income', 'expense'],
    CategoryTrendPoint: ['amount'],
    AccountBalancePoint: ['net', 'balance'],
    BudgetExecution: ['budget', 'expense', 'remaining'],
    SummaryTableRow: ['income', 'expense', 'balance'],
    PeriodTotals: ['income', 'expense'],
    Summary: ['income', 'expense', 'balance'],
  };
  for (const [schemaName, propertyNames] of Object.entries(moneyFieldsBySchema)) {
    const schema = allSchemas[schemaName] || '';
    for (const propertyName of propertyNames) {
      const property = schemaPropertyBlock(schema, propertyName);
      if (!property.includes('multipleOf: 0.01')) {
        throw new Error(`${schemaName}.${propertyName} is missing multipleOf: 0.01`);
      }
    }
  }

  const trendGranularity = schemaPropertyBlock(allSchemas.Summary || '', 'trendGranularity');
  if (!trendGranularity.includes('enum: [day, week, month]')) {
    throw new Error('Summary.trendGranularity is missing day/week/month enum');
  }
}

function validateReportResponseExamples(allSchemas) {
  const requiredExamples = {
    CategoryStat: ['categoryId: 1', 'category: 餐饮', 'amount: 250'],
    AccountStat: ['accountId: 1', 'account: 现金', 'amount: 250'],
    MonthTrend: ["month: '2026-06'", 'income: 3000', 'expense: 250'],
    TrendPoint: ["period: '2026-06-01'", 'income: 3000', 'expense: 100'],
    CategoryTrendPoint: ['categoryId: 1', 'category: 餐饮', 'amount: 100'],
    AccountBalancePoint: ['net: -150', 'balance: 2750'],
    BudgetExecution: ['budget: 500', 'expense: 250', 'remaining: 250', 'usageRate: 0.5'],
    SummaryTableRow: ['balance: -150', 'txCount: 1'],
    PeriodTotals: ['income: 3000', 'expense: 250'],
    PeriodCompare: ['current:', 'previous:'],
    Summary: ['trendGranularity: day', 'dailySummaries:', 'monthlySummaries:', 'periodCompare:'],
  };

  for (const [schemaName, expectedTexts] of Object.entries(requiredExamples)) {
    const example = nestedBlock(allSchemas[schemaName] || '', 'example:');
    if (!example) {
      throw new Error(`${schemaName} is missing example`);
    }
    for (const expectedText of expectedTexts) {
      if (!example.includes(expectedText)) {
        throw new Error(`${schemaName} example is missing ${expectedText}`);
      }
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

function validateImportResponseBounds(allSchemas) {
  const fieldsBySchema = {
    ImportPreviewRow: { line: 1 },
    ImportPreview: { size: 0, totalRows: 0, validRows: 0, failedRows: 0, duplicateRows: 0, maxRows: 0, maxFileBytes: 0 },
    ImportResult: { total: 0, success: 0, failed: 0, skipped: 0 },
    ImportJob: { id: 1, total: 0, success: 0, failed: 0, skipped: 0 },
  };

  for (const [schemaName, propertyBounds] of Object.entries(fieldsBySchema)) {
    const schema = allSchemas[schemaName] || '';
    for (const [propertyName, minimum] of Object.entries(propertyBounds)) {
      const property = schemaPropertyBlock(schema, propertyName);
      if (!property.includes(`minimum: ${minimum}`)) {
        throw new Error(`${schemaName}.${propertyName} is missing minimum: ${minimum}`);
      }
    }
  }

  const amount = schemaPropertyBlock(allSchemas.ImportPreviewRow || '', 'amount');
  if (!amount.includes('minimum: 0')) {
    throw new Error('ImportPreviewRow.amount is missing minimum: 0');
  }
  if (!amount.includes('multipleOf: 0.01')) {
    throw new Error('ImportPreviewRow.amount is missing multipleOf: 0.01');
  }

  const rows = schemaPropertyBlock(allSchemas.ImportPreview || '', 'rows');
  if (!rows.includes('maxItems: 20')) {
    throw new Error('ImportPreview.rows is missing maxItems: 20');
  }

  const type = schemaPropertyBlock(allSchemas.ImportPreviewRow || '', 'type');
  if (!type.includes('enum: [income, expense, \'\']')) {
    throw new Error('ImportPreviewRow.type must allow an empty string for failed preview rows');
  }
}

function validateImportResponseExamples(allSchemas) {
  const requiredExamples = {
    ImportPreviewRow: ['line: 2', 'occurredAt:', 'type: expense', 'amount: 35.5', 'valid: true', 'duplicate: false'],
    ImportPreview: ['filename: preview.csv', 'maxRows: 5000', 'maxFileBytes: 5242880', 'valid: false', "type: ''", 'error: invalid occurred_at'],
    ImportResult: ['total: 2', 'success: 1', 'failed: 1', 'skipped: 0', 'line 3: invalid occurred_at'],
    ImportJob: ['id: 1', 'status: completed', 'createdAt:', 'updatedAt:', 'line 3: invalid occurred_at'],
  };

  for (const [schemaName, expectedTexts] of Object.entries(requiredExamples)) {
    const example = nestedBlock(allSchemas[schemaName] || '', 'example:');
    if (!example) {
      throw new Error(`${schemaName} is missing example`);
    }
    for (const expectedText of expectedTexts) {
      if (!example.includes(expectedText)) {
        throw new Error(`${schemaName} example is missing ${expectedText}`);
      }
    }
  }
}

function validateImportJobReadOnlyFields(schema) {
  for (const propertyName of ['id', 'createdAt', 'updatedAt']) {
    const property = schemaPropertyBlock(schema, propertyName);
    if (!property.includes('readOnly: true')) {
      throw new Error(`ImportJob.${propertyName} is missing readOnly: true`);
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
  const properties = nestedBlock(schema, 'properties:') || schema;
  const pattern = new RegExp(`^        ${escapeRegExp(propertyName)}:\\n(?:          .+\\n)+`, 'm');
  return properties.match(pattern)?.[0] || '';
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
  if (!contentDisposition.includes('ASCII-safe filename fallback') || !contentDisposition.includes('prefer filename*')) {
    throw new Error('components.headers.ContentDisposition must document ASCII fallback and filename* precedence');
  }

  const logoutSuccess = operationResponseBlockById(openapi, 'postAuthLogout', '200');
  if (!logoutSuccess.includes('Clear-Site-Data:')) {
    throw new Error('POST /auth/logout 200 response is missing Clear-Site-Data header');
  }
  const clearSiteData = componentHeaderBlock(openapi, 'ClearSiteData');
  if (!clearSiteData.includes('Browser-side data cleared after logout') || !clearSiteData.includes('"cache", "cookies", "storage"')) {
    throw new Error('components.headers.ClearSiteData must document logout browser data clearing directives');
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
  // 认证文档要覆盖运行时安全边界，避免客户端只按 OpenAPI 调用时踩到隐藏规则。
  if (!bearerAuth.includes('repeated `Authorization` headers are rejected')) {
    throw new Error('components.securitySchemes.bearerAuth is missing repeated Authorization header guidance');
  }
  if (!bearerAuth.includes('`nbf` not-before claim')) {
    throw new Error('components.securitySchemes.bearerAuth is missing JWT not-before claim guidance');
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
    for (const field of ['error:', 'code:', 'status:', 'requestId:']) {
      if (!example.includes(field)) {
        throw new Error(`components.responses.${responseName} example is missing ${field}`);
      }
    }
    const expected = errorResponseExampleContract()[responseName];
    if (expected && (!example.includes(`status: ${expected.status}`) || !example.includes(`code: ${expected.code}`))) {
      throw new Error(`components.responses.${responseName} example must use status ${expected.status} and code ${expected.code}`);
    }
  }
}

function errorResponseExampleContract() {
  return {
    BadRequest: { status: 400, code: 'bad_request' },
    InvalidRequest: { status: 400, code: 'invalid_request' },
    Unauthorized: { status: 401, code: 'unauthorized' },
    Forbidden: { status: 403, code: 'forbidden' },
    NotFound: { status: 404, code: 'not_found' },
    MethodNotAllowed: { status: 405, code: 'method_not_allowed' },
    RateLimited: { status: 429, code: 'rate_limited' },
    PayloadTooLarge: { status: 413, code: 'payload_too_large' },
    UnsupportedMediaType: { status: 415, code: 'unsupported_media_type' },
    NotAcceptable: { status: 406, code: 'not_acceptable' },
    InternalError: { status: 500, code: 'internal_error' },
    GatewayTimeout: { status: 504, code: 'request_timeout' },
    ClientClosedRequest: { status: 499, code: 'client_closed_request' },
    Error: { status: 500, code: 'internal_error' },
  };
}

function errorResponseNames() {
  return [
    'BadRequest',
    'InvalidRequest',
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
    'ClientClosedRequest',
    'Error',
  ];
}

function noStoreResponseNames() {
  return [
    'BadRequest',
    'InvalidRequest',
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
    'ClientClosedRequest',
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
  if (!block.includes('system entropy source is unavailable')) {
    throw new Error(`${name} must document generated id fallback uniqueness`);
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

// parseEndpoints 同时承担解析和契约验证：任何 OpenAPI 漂移都应在生成客户端前失败。
function parseEndpoints(openapi) {
  const pathsBlock = openapi.slice(openapi.indexOf('paths:'), openapi.indexOf('components:'));
  const pathMatches = [...pathsBlock.matchAll(/^  (\/[^:]+):$/gm)];
  const operationIds = new Map();
  const declaredTags = declaredOpenAPITags(openapi);
  const componentParameters = parseComponentParameters(openapi);
  const result = [];
  for (const [index, match] of pathMatches.entries()) {
    const apiPath = match[1];
    const start = match.index;
    const end = pathMatches[index + 1]?.index ?? pathsBlock.length;
    const pathBlock = pathsBlock.slice(start, end);
    const sharedPathBlock = pathLevelBlock(pathBlock);
    const methodMatches = openAPIMethodMatches(pathBlock);
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
      const parameters = parseEffectiveOperationParameters(sharedPathBlock, methodBlock, componentParameters, method, apiPath);
      if (!operationId) {
        throw new Error(`${method.toUpperCase()} ${apiPath} is missing operationId`);
      }
      const expectedOperationId = endpointMethodName(method, apiPath);
      if (operationId !== expectedOperationId) {
        throw new Error(`${method.toUpperCase()} ${apiPath} operationId must be ${expectedOperationId}`);
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
      if (tags.length !== 1) {
        throw new Error(`${method.toUpperCase()} ${apiPath} must declare exactly one tag for generated client grouping`);
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
      if (responseStatuses.has('500') && !responseStatuses.has('499')) {
        throw new Error(`${method.toUpperCase()} ${apiPath} declares 500 response but is missing 499 client closed response`);
      }
      if (operationHasSuccessContent(methodBlock, source) && !responseStatuses.has('406')) {
        throw new Error(`${method.toUpperCase()} ${apiPath} returns a response body but is missing 406 response`);
      }
      const jsonClientEndpoint = operationHasJSONSuccessContent(methodBlock, source);
      validateNegotiatedResponseHeaders(method, apiPath, methodBlock, responseStatuses, source);
      validateAuthOperationContract(method, apiPath, methodBlock, responseStatuses);
      validatePathParameters(method, apiPath, parameters);
      validateSuccessResponseHeaders(method, apiPath, methodBlock);
      validateCreatedResponseHeaders(method, apiPath, methodBlock);
      validateAcceptedResponseHeaders(method, apiPath, methodBlock);
      validateSuccessResponseCacheHeaders(method, apiPath, methodBlock, source);
      validateOperationQueryContract(method, apiPath, methodBlock, responseStatuses, parameters, `${sharedPathBlock}\n${methodBlock}`);
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

// 请求体契约必须显式描述、必填，并且只使用 JSON 或 multipart 其中一种格式。
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
  if (hasJSON) {
    const jsonBlock = nestedBlock(requestBlock, 'application/json:');
    // 中间件允许 JSON Content-Type 参数和 application/*+json，这些运行时兼容性也要写进 OpenAPI。
    if (!jsonBlock.includes('Content-Type: application/json') || !jsonBlock.includes('charset=utf-8')) {
      throw new Error(`${method.toUpperCase()} ${apiPath} JSON requestBody must document Content-Type parameter handling`);
    }
    if (!jsonBlock.includes('application/*+json')) {
      throw new Error(`${method.toUpperCase()} ${apiPath} JSON requestBody must document structured JSON media type handling`);
    }
  }
  if (hasMultipart) {
    const multipartBlock = nestedBlock(requestBlock, 'multipart/form-data:');
    // multipart 请求依赖非空 boundary 和 file 表单字段，OpenAPI 必须把客户端要构造的格式写清楚。
    if (!multipartBlock.includes('Content-Type: multipart/form-data; boundary=<')) {
      throw new Error(`${method.toUpperCase()} ${apiPath} multipart requestBody must document boundary parameter handling`);
    }
    if (!multipartBlock.includes('non-empty-boundary')) {
      throw new Error(`${method.toUpperCase()} ${apiPath} multipart requestBody must document non-empty boundary requirement`);
    }
    if (!multipartBlock.includes('required `file` field')) {
      throw new Error(`${method.toUpperCase()} ${apiPath} multipart requestBody must document required file field`);
    }
  }
  if (!requestSchema(methodBlock)) {
    throw new Error(`${method.toUpperCase()} ${apiPath} requestBody is missing a component schema reference`);
  }
  const badRequest = operationResponseBlocks(methodBlock).find((response) => response.status === '400')?.block || '';
  if (!badRequest.includes("$ref: '#/components/responses/InvalidRequest'")) {
    throw new Error(`${method.toUpperCase()} ${apiPath} requestBody 400 response must use InvalidRequest`);
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

function validateCreatedResponseHeaders(method, apiPath, methodBlock) {
  const createdResponses = operationResponseBlocks(methodBlock).filter((response) => response.status === '201');
  for (const response of createdResponses) {
    if (response.block.includes('Location:')) continue;
    throw new Error(`${method.toUpperCase()} ${apiPath} 201 response is missing Location header`);
  }
}

function validateAcceptedResponseHeaders(method, apiPath, methodBlock) {
  const acceptedResponses = operationResponseBlocks(methodBlock).filter((response) => response.status === '202');
  for (const response of acceptedResponses) {
    if (response.block.includes('Location:')) continue;
    throw new Error(`${method.toUpperCase()} ${apiPath} 202 response is missing Location header`);
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

function validateOperationQueryContract(method, apiPath, methodBlock, responseStatuses, operationParameters, parameterSourceBlock) {
  const parameters = operationParameters.filter((param) => param.in === 'query');
  if (parameters.length === 0) return;

  if (apiPath === '/transactions' && method === 'get') {
    require400Response(method, apiPath, responseStatuses, methodBlock);
    requireParameterText(method, apiPath, parameterSourceBlock, 'page', 'minimum: 1');
    requireParameterText(method, apiPath, parameterSourceBlock, 'page', 'default: 1');
    requireParameterText(method, apiPath, parameterSourceBlock, 'page', 'Defaults to 1 when omitted');
    requireParameterText(method, apiPath, parameterSourceBlock, 'page', 'example: 1');
    requireParameterText(method, apiPath, parameterSourceBlock, 'pageSize', 'minimum: 1');
    requireParameterText(method, apiPath, parameterSourceBlock, 'pageSize', 'maximum: 200');
    requireParameterText(method, apiPath, parameterSourceBlock, 'pageSize', 'default: 20');
    requireParameterText(method, apiPath, parameterSourceBlock, 'pageSize', 'Defaults to 20 when omitted');
    requireParameterText(method, apiPath, parameterSourceBlock, 'pageSize', 'example: 20');
    requireDateTimeQueryParameter(method, apiPath, parameterSourceBlock, 'start');
    requireDateTimeQueryParameter(method, apiPath, parameterSourceBlock, 'end');
    requireParameterText(method, apiPath, parameterSourceBlock, 'type', 'Transaction type filter');
    requireParameterText(method, apiPath, parameterSourceBlock, 'type', 'example: expense');
    requireParameterText(method, apiPath, parameterSourceBlock, 'categoryId', 'minimum: 1');
    requireParameterText(method, apiPath, parameterSourceBlock, 'categoryId', 'Category id filter');
    requireParameterText(method, apiPath, parameterSourceBlock, 'categoryId', 'example: 1');
    requireParameterText(method, apiPath, parameterSourceBlock, 'accountId', 'minimum: 1');
    requireParameterText(method, apiPath, parameterSourceBlock, 'accountId', 'Account id filter');
    requireParameterText(method, apiPath, parameterSourceBlock, 'accountId', 'example: 1');
    requireParameterText(method, apiPath, parameterSourceBlock, 'q', 'matched against transaction notes and tags');
    requireParameterText(method, apiPath, parameterSourceBlock, 'q', 'Maximum 100 characters');
    requireParameterText(method, apiPath, parameterSourceBlock, 'q', 'maxLength: 100');
    requireParameterText(method, apiPath, parameterSourceBlock, 'q', 'example: lunch');
  }
  if (apiPath === '/budgets' && method === 'get') {
    require400Response(method, apiPath, responseStatuses, methodBlock);
    requireParameterText(method, apiPath, parameterSourceBlock, 'month', "pattern: '^\\d{4}-\\d{2}$'");
    requireParameterText(method, apiPath, parameterSourceBlock, 'month', 'Budget month in YYYY-MM format');
    requireParameterText(method, apiPath, parameterSourceBlock, 'month', "example: '2026-06'");
  }
  if (apiPath === '/categories' && method === 'get') {
    require400Response(method, apiPath, responseStatuses, methodBlock);
    requireParameterText(method, apiPath, parameterSourceBlock, 'type', "$ref: '#/components/schemas/TransactionType'");
    requireParameterText(method, apiPath, parameterSourceBlock, 'type', 'Transaction type filter');
    requireParameterText(method, apiPath, parameterSourceBlock, 'type', 'example: expense');
  }
  if (apiPath === '/reports/summary' && method === 'get') {
    require400Response(method, apiPath, responseStatuses, methodBlock);
    requireDateTimeQueryParameter(method, apiPath, parameterSourceBlock, 'start');
    requireDateTimeQueryParameter(method, apiPath, parameterSourceBlock, 'end');
    requireParameterText(method, apiPath, parameterSourceBlock, 'categoryId', 'minimum: 1');
    requireParameterText(method, apiPath, parameterSourceBlock, 'categoryId', 'Category id filter');
    requireParameterText(method, apiPath, parameterSourceBlock, 'categoryId', 'example: 1');
    requireParameterText(method, apiPath, parameterSourceBlock, 'accountId', 'minimum: 1');
    requireParameterText(method, apiPath, parameterSourceBlock, 'accountId', 'Account id filter');
    requireParameterText(method, apiPath, parameterSourceBlock, 'accountId', 'example: 1');
    requireParameterText(method, apiPath, parameterSourceBlock, 'trend', 'enum: [day, week, month]');
    requireParameterText(method, apiPath, parameterSourceBlock, 'trend', 'default: month');
    requireParameterText(method, apiPath, parameterSourceBlock, 'trend', 'Defaults to month when omitted');
    requireParameterText(method, apiPath, parameterSourceBlock, 'trend', 'example: month');
  }
  if (apiPath === '/io/export' && method === 'get') {
    require400Response(method, apiPath, responseStatuses, methodBlock);
    requireParameterText(method, apiPath, parameterSourceBlock, 'format', 'enum: [csv, xlsx]');
    requireParameterText(method, apiPath, parameterSourceBlock, 'format', 'default: csv');
    requireParameterText(method, apiPath, parameterSourceBlock, 'format', 'Values are trimmed and case-insensitive');
    requireParameterText(method, apiPath, parameterSourceBlock, 'format', 'defaults to csv when omitted');
    requireParameterText(method, apiPath, parameterSourceBlock, 'format', 'example: csv');
    requireDateTimeQueryParameter(method, apiPath, parameterSourceBlock, 'start');
    requireDateTimeQueryParameter(method, apiPath, parameterSourceBlock, 'end');
    if (!methodBlock.includes('Content-Disposition:')) {
      throw new Error('GET /io/export is missing Content-Disposition response header');
    }
  }
}

function require400Response(method, apiPath, responseStatuses, methodBlock) {
  if (!responseStatuses.has('400')) {
    throw new Error(`${method.toUpperCase()} ${apiPath} has validated query parameters but is missing 400 response`);
  }
  const badRequest = operationResponseBlocks(methodBlock).find((response) => response.status === '400')?.block || '';
  if (!badRequest.includes("$ref: '#/components/responses/InvalidRequest'")) {
    throw new Error(`${method.toUpperCase()} ${apiPath} query 400 response must use InvalidRequest`);
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
  for (const match of block.matchAll(/^\s+parameters:$/gm)) {
    const parametersBlock = nestedBlock(block.slice(match.index), 'parameters:');
    const paramBlocks = parametersBlock.split(/\n(?=\s+- name:|\s+- \$ref:)/).map((item) => item.trim()).filter(Boolean);
    const parameterBlock = paramBlocks.find((paramBlock) => paramBlock.includes(`name: ${name}`));
    if (parameterBlock) return parameterBlock;
  }
  return '';
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
  if (start === -1) {
    throw new Error(`OpenAPI references unknown response component ${componentName}`);
  }
  const next = responsesBlock.slice(start + marker.length).search(/^    [A-Za-z][A-Za-z0-9]*:/m);
  return responsesBlock.slice(start, next === -1 ? undefined : start + marker.length + next);
}

function componentHeaderBlock(openapi, componentName) {
  const headersBlock = nestedBlock(openAPIComponentsBlock(openapi), 'headers:');
  const marker = `    ${componentName}:`;
  const start = headersBlock.indexOf(marker);
  if (start === -1) {
    throw new Error(`OpenAPI references unknown header component ${componentName}`);
  }
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
    if (tags.has(name)) {
      throw new Error(`OpenAPI tag ${name} is declared more than once`);
    }
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
    const methodMatches = openAPIMethodMatches(pathBlock);
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
  const firstMethodIndex = pathBlock.search(openAPIMethodLineRegexp());
  return firstMethodIndex === -1 ? pathBlock : pathBlock.slice(0, firstMethodIndex);
}

function openAPIMethodMatches(pathBlock) {
  return [...pathBlock.matchAll(new RegExp(`^    (${openAPIHTTPMethodPattern}):$`, 'gm'))];
}

function openAPIMethodLineRegexp() {
  return new RegExp(`^    (${openAPIHTTPMethodPattern}):`, 'm');
}

// 汇总 Path Item 和 operation 两层参数，让生成器遵守 OpenAPI 的参数继承规则。
function parseEffectiveOperationParameters(pathLevelBlockSource, methodBlock, componentParameters, method, apiPath) {
  const pathParameters = parseOperationParameters(pathLevelBlockSource, componentParameters, `${apiPath} path parameters`);
  const operationParameters = parseOperationParameters(methodBlock, componentParameters, `${method.toUpperCase()} ${apiPath} parameters`);
  return mergeOpenAPIParameters(pathParameters, operationParameters);
}

// OpenAPI 规定 Path Item 参数会被 operation 继承；同名同位置参数由 operation 级定义覆盖。
function mergeOpenAPIParameters(pathParameters, operationParameters) {
  const result = [];
  const indexes = new Map();
  for (const parameter of [...pathParameters, ...operationParameters]) {
    const key = parameterKey(parameter);
    if (indexes.has(key)) {
      result[indexes.get(key)] = parameter;
      continue;
    }
    indexes.set(key, result.length);
    result.push(parameter);
  }
  return result;
}

// 先解析 components.parameters，后续路径或接口里的 $ref 都必须能解析到这里。
function parseComponentParameters(openapi) {
  const parametersBlock = nestedBlock(openAPIComponentsBlock(openapi), 'parameters:');
  if (!parametersBlock) return new Map();

  const names = [...parametersBlock.matchAll(/^    ([A-Za-z][A-Za-z0-9]*):$/gm)];
  return new Map(names.map((match, index) => {
    const name = match[1];
    const start = match.index;
    const end = names[index + 1]?.index ?? parametersBlock.length;
    return [name, parseParameterBlock(parametersBlock.slice(start, end), new Map(), `component parameter ${name}`)];
  }));
}

function parseOperationParameters(block, componentParameters = new Map(), context = 'parameters') {
  const parametersBlock = nestedBlock(block, 'parameters:');
  if (!parametersBlock) return [];

  const paramBlocks = parametersBlock.split(/\n(?=\s+- name:|\s+- \$ref:)/).map((item) => item.trim()).filter(Boolean);
  const parameters = paramBlocks.map((paramBlock) => parseParameterBlock(paramBlock, componentParameters, context)).filter((param) => param.name);
  validateUniqueParameters(parameters, context);
  return parameters;
}

// 参数对象必须显式写清 name 和 in，避免少写字段时被生成器误判成 query 参数。
function parseParameterBlock(paramBlock, componentParameters, context) {
  const componentName = paramBlock.match(/\$ref:\s+'#\/components\/parameters\/([^']+)'/)?.[1] || '';
  if (componentName) {
    const parameter = componentParameters.get(componentName);
    if (!parameter) {
      throw new Error(`${context} references unknown component parameter ${componentName}`);
    }
    return { ...parameter };
  }
  const name = paramBlock.match(/name:\s+([A-Za-z][A-Za-z0-9_-]*)/)?.[1] || '';
  if (!name) {
    throw new Error(`${context} has a parameter missing name`);
  }
  const parameterIn = paramBlock.match(/in:\s+([A-Za-z]+)/)?.[1] || '';
  if (!parameterIn) {
    throw new Error(`${context} parameter ${name} is missing in`);
  }
  if (!['query', 'header', 'path', 'cookie'].includes(parameterIn)) {
    throw new Error(`${context} parameter ${name} has unsupported in ${parameterIn}`);
  }
  return {
    name,
    in: parameterIn,
    type: paramBlock.match(/type:\s+([A-Za-z]+)/)?.[1] || 'string',
    enumValues: enumValues(paramBlock),
    ref: paramBlock.match(/\$ref:\s+'#\/components\/schemas\/([^']+)'/)?.[1] || '',
    required: parseParameterRequired(paramBlock, context, name),
  };
}

function parseParameterRequired(paramBlock, context, name) {
  const match = paramBlock.match(/^\s+required:\s+(\S+)\s*$/m);
  if (!match) return false;
  if (match[1] === 'true') return true;
  if (match[1] === 'false') return false;
  throw new Error(`${context} parameter ${name} required must be true or false`);
}

function validateUniqueParameters(parameters, context) {
  const seen = new Set();
  for (const parameter of parameters) {
    const key = parameterKey(parameter);
    if (seen.has(key)) {
      throw new Error(`${context} has duplicate parameter ${parameter.name} in ${parameter.in}`);
    }
    seen.add(key);
  }
}

function parameterKey(parameter) {
  return `${parameter.in}:${parameter.name}`;
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
  return `${method}${upperFirst(base)}`;
}

function groupEndpoints(endpoints) {
  const groups = new Map();
  for (const endpoint of endpoints) {
    // 客户端模块分组以 OpenAPI tag 为准，避免路径前缀调整时生成 API 结构漂移。
    const group = endpoint.tags[0].replace(/[^A-Za-z0-9]/g, '');
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
  return match[1].split(',').map(parseInlineYAMLScalar);
}

function upperFirst(value) {
  return value.charAt(0).toUpperCase() + value.slice(1);
}

function parseEnum(block) {
  const match = block.match(/^      enum:\s+\[([^\]]+)\]/m);
  if (!match) return [];
  return match[1].split(',').map(parseInlineYAMLScalar);
}

function parseInlineYAMLScalar(value) {
  const clean = value.trim();
  if ((clean.startsWith("'") && clean.endsWith("'")) || (clean.startsWith('"') && clean.endsWith('"'))) {
    return clean.slice(1, -1);
  }
  return clean;
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
