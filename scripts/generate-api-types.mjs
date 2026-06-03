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
const schemas = Object.fromEntries(schemaNames.map((name, index) => {
  const marker = `    ${name}:`;
  const start = schemasSource.indexOf(marker);
  const nextName = schemaNames[index + 1];
  const end = nextName ? schemasSource.indexOf(`    ${nextName}:`, start + marker.length) : schemasSource.length;
  return [name, schemasSource.slice(start, end)];
}));

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
  lines.push("  if (typeof value === 'number' && value === 0) return;");
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

  lines.push(`${methodName}: (${args.join(', ')}) => {`);
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
      lines.push(`  setQueryParam(search, '${param.name}', params.${param.name});`);
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

function parseEndpoints(openapi) {
  const pathsBlock = openapi.slice(openapi.indexOf('paths:'), openapi.indexOf('components:'));
  const pathMatches = [...pathsBlock.matchAll(/^  (\/[^:]+):$/gm)];
  const result = [];
  for (const [index, match] of pathMatches.entries()) {
    const apiPath = match[1];
    const start = match.index;
    const end = pathMatches[index + 1]?.index ?? pathsBlock.length;
    const pathBlock = pathsBlock.slice(start, end);
    const methodMatches = [...pathBlock.matchAll(/^    (get|post|put|delete):$/gm)];
    for (const [methodIndex, methodMatch] of methodMatches.entries()) {
      const method = methodMatch[1];
      const methodStart = methodMatch.index;
      const methodEnd = methodMatches[methodIndex + 1]?.index ?? pathBlock.length;
      const methodBlock = pathBlock.slice(methodStart, methodEnd);
      result.push({
        path: apiPath,
        method,
        name: endpointMethodName(method, apiPath),
        requestSchema: requestSchema(methodBlock),
        responseSchema: responseSchema(methodBlock),
        multipart: methodBlock.includes('multipart/form-data'),
        parameters: parseOperationParameters(methodBlock),
      });
    }
  }
  return result;
}

function parseOperationParameters(block) {
  const parametersBlock = nestedBlock(block, 'parameters:');
  if (!parametersBlock) return [];

  const paramBlocks = parametersBlock.split(/\n(?=\s+- name:|\s+- \$ref:)/).map((item) => item.trim()).filter(Boolean);
  return paramBlocks.map((paramBlock) => {
    if (paramBlock.includes("$ref: '#/components/parameters/Id'")) {
      return { name: 'id', in: 'path', type: 'integer' };
    }
    return {
      name: paramBlock.match(/name:\s+([A-Za-z][A-Za-z0-9]*)/)?.[1] || '',
      in: paramBlock.match(/in:\s+([A-Za-z]+)/)?.[1] || 'query',
      type: paramBlock.match(/type:\s+([A-Za-z]+)/)?.[1] || 'string',
      enumValues: enumValues(paramBlock),
      ref: paramBlock.match(/\$ref:\s+'#\/components\/schemas\/([^']+)'/)?.[1] || '',
    };
  }).filter((param) => param.name);
}

function requestSchema(block) {
  const requestBlock = nestedBlock(block, 'requestBody:');
  if (!requestBlock) return '';
  return requestBlock.match(/\$ref:\s+'#\/components\/schemas\/([^']+)'/)?.[1] || '';
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
