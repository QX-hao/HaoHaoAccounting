// Generated from backend/api/openapi.yaml. Do not edit by hand.
import type {
  TransactionType,
  CurrentUser,
  LoginRequest,
  LoginResponse,
  Account,
  AccountRequest,
  Budget,
  BudgetRequest,
  Category,
  CategoryRequest,
  Transaction,
  TransactionRequest,
  TransactionListResponse,
  Pagination,
  AIParseRequest,
  AIParseResult,
  AIParseResponse,
  CategoryStat,
  AccountStat,
  MonthTrend,
  TrendPoint,
  CategoryTrendPoint,
  AccountBalancePoint,
  BudgetExecution,
  SummaryTableRow,
  PeriodTotals,
  PeriodCompare,
  Summary,
  ImportFileRequest,
  ImportTextRequest,
  ImportPreviewRow,
  ImportPreview,
  ImportResult,
  ImportJob,
  ErrorResponse,
  OkResponse,
} from '../types/api';

export type ApiRequest = <T>(path: string, init?: RequestInit) => Promise<T>;
export type ApiUpload = <T>(path: string, formData: FormData) => Promise<T>;

export type ApiRuntime = {
  request: ApiRequest;
  upload?: ApiUpload;
};

export function createApiClient(runtime: ApiRuntime) {
  return {
    auth: {
      postAuthLogin: (body: LoginRequest) => {
        let path = "/auth/login";
        return runtime.request<LoginResponse>(path, { method: 'POST', body: JSON.stringify(body) });
      },
      postAuthRefresh: () => {
        let path = "/auth/refresh";
        return runtime.request<LoginResponse>(path, { method: 'POST' });
      },
      postAuthLogout: () => {
        let path = "/auth/logout";
        return runtime.request<OkResponse>(path, { method: 'POST' });
      },
    },
    me: {
      getMe: () => {
        let path = "/me";
        return runtime.request<CurrentUser>(path);
      },
    },
    accounts: {
      getAccounts: () => {
        let path = "/accounts";
        return runtime.request<Account[]>(path);
      },
      postAccounts: (body: AccountRequest) => {
        let path = "/accounts";
        return runtime.request<Account>(path, { method: 'POST', body: JSON.stringify(body) });
      },
      putAccountsById: (params: {
  id: number | string;
}, body: AccountRequest) => {
        let path = "/accounts/{id}";
        path = path.replace('{id}', encodeURIComponent(String(params.id)));
        return runtime.request<Account>(path, { method: 'PUT', body: JSON.stringify(body) });
      },
      deleteAccountsById: (params: {
  id: number | string;
}) => {
        let path = "/accounts/{id}";
        path = path.replace('{id}', encodeURIComponent(String(params.id)));
        return runtime.request<OkResponse>(path, { method: 'DELETE' });
      },
    },
    budgets: {
      getBudgets: (params: {
  month?: string;
} = {}) => {
        let path = "/budgets";
        const search = new URLSearchParams();
        setQueryParam(search, 'month', params?.month);
        const query = search.toString();
        if (query) path += `?${query}`;
        return runtime.request<Budget[]>(path);
      },
      postBudgets: (body: BudgetRequest) => {
        let path = "/budgets";
        return runtime.request<Budget>(path, { method: 'POST', body: JSON.stringify(body) });
      },
      putBudgetsById: (params: {
  id: number | string;
}, body: BudgetRequest) => {
        let path = "/budgets/{id}";
        path = path.replace('{id}', encodeURIComponent(String(params.id)));
        return runtime.request<Budget>(path, { method: 'PUT', body: JSON.stringify(body) });
      },
      deleteBudgetsById: (params: {
  id: number | string;
}) => {
        let path = "/budgets/{id}";
        path = path.replace('{id}', encodeURIComponent(String(params.id)));
        return runtime.request<OkResponse>(path, { method: 'DELETE' });
      },
    },
    categories: {
      getCategories: (params: {
  type?: TransactionType;
} = {}) => {
        let path = "/categories";
        const search = new URLSearchParams();
        setQueryParam(search, 'type', params?.type);
        const query = search.toString();
        if (query) path += `?${query}`;
        return runtime.request<Category[]>(path);
      },
      postCategories: (body: CategoryRequest) => {
        let path = "/categories";
        return runtime.request<Category>(path, { method: 'POST', body: JSON.stringify(body) });
      },
      putCategoriesById: (params: {
  id: number | string;
}, body: CategoryRequest) => {
        let path = "/categories/{id}";
        path = path.replace('{id}', encodeURIComponent(String(params.id)));
        return runtime.request<Category>(path, { method: 'PUT', body: JSON.stringify(body) });
      },
      deleteCategoriesById: (params: {
  id: number | string;
}) => {
        let path = "/categories/{id}";
        path = path.replace('{id}', encodeURIComponent(String(params.id)));
        return runtime.request<OkResponse>(path, { method: 'DELETE' });
      },
    },
    transactions: {
      getTransactions: (params: {
  page?: number;
  pageSize?: number;
  start?: string;
  end?: string;
  type?: TransactionType;
  categoryId?: number;
  accountId?: number;
  q?: string;
} = {}) => {
        let path = "/transactions";
        const search = new URLSearchParams();
        setQueryParam(search, 'page', params?.page);
        setQueryParam(search, 'pageSize', params?.pageSize);
        setQueryParam(search, 'start', params?.start);
        setQueryParam(search, 'end', params?.end);
        setQueryParam(search, 'type', params?.type);
        setQueryParam(search, 'categoryId', params?.categoryId);
        setQueryParam(search, 'accountId', params?.accountId);
        setQueryParam(search, 'q', params?.q);
        const query = search.toString();
        if (query) path += `?${query}`;
        return runtime.request<TransactionListResponse>(path);
      },
      postTransactions: (body: TransactionRequest) => {
        let path = "/transactions";
        return runtime.request<Transaction>(path, { method: 'POST', body: JSON.stringify(body) });
      },
      putTransactionsById: (params: {
  id: number | string;
}, body: TransactionRequest) => {
        let path = "/transactions/{id}";
        path = path.replace('{id}', encodeURIComponent(String(params.id)));
        return runtime.request<Transaction>(path, { method: 'PUT', body: JSON.stringify(body) });
      },
      deleteTransactionsById: (params: {
  id: number | string;
}) => {
        let path = "/transactions/{id}";
        path = path.replace('{id}', encodeURIComponent(String(params.id)));
        return runtime.request<OkResponse>(path, { method: 'DELETE' });
      },
    },
    ai: {
      postAiParse: (body: AIParseRequest) => {
        let path = "/ai/parse";
        return runtime.request<AIParseResponse>(path, { method: 'POST', body: JSON.stringify(body) });
      },
    },
    reports: {
      getReportsSummary: (params: {
  start?: string;
  end?: string;
  categoryId?: number;
  accountId?: number;
  trend?: "day" | "week" | "month";
} = {}) => {
        let path = "/reports/summary";
        const search = new URLSearchParams();
        setQueryParam(search, 'start', params?.start);
        setQueryParam(search, 'end', params?.end);
        setQueryParam(search, 'categoryId', params?.categoryId);
        setQueryParam(search, 'accountId', params?.accountId);
        setQueryParam(search, 'trend', params?.trend);
        const query = search.toString();
        if (query) path += `?${query}`;
        return runtime.request<Summary>(path);
      },
    },
    dataio: {
      postIoImportPreview: (body: FormData) => {
        let path = "/io/import/preview";
        if (!runtime.upload) throw new Error('upload runtime is required');
        return runtime.upload<ImportPreview>(path, body);
      },
      postIoImport: (body: FormData) => {
        let path = "/io/import";
        if (!runtime.upload) throw new Error('upload runtime is required');
        return runtime.upload<ImportResult>(path, body);
      },
      getIoImportJobs: () => {
        let path = "/io/import/jobs";
        return runtime.request<ImportJob[]>(path);
      },
      postIoImportJobs: (body: FormData) => {
        let path = "/io/import/jobs";
        if (!runtime.upload) throw new Error('upload runtime is required');
        return runtime.upload<ImportJob>(path, body);
      },
      getIoImportJobsById: (params: {
  id: number | string;
}) => {
        let path = "/io/import/jobs/{id}";
        path = path.replace('{id}', encodeURIComponent(String(params.id)));
        return runtime.request<ImportJob>(path);
      },
      postIoImportTextPreview: (body: ImportTextRequest) => {
        let path = "/io/import/text/preview";
        return runtime.request<ImportPreview>(path, { method: 'POST', body: JSON.stringify(body) });
      },
      postIoImportText: (body: ImportTextRequest) => {
        let path = "/io/import/text";
        return runtime.request<ImportResult>(path, { method: 'POST', body: JSON.stringify(body) });
      },
      getIoExport: (params: {
  format?: "csv" | "xlsx";
  start?: string;
  end?: string;
} = {}) => {
        let path = "/io/export";
        const search = new URLSearchParams();
        setQueryParam(search, 'format', params?.format);
        setQueryParam(search, 'start', params?.start);
        setQueryParam(search, 'end', params?.end);
        const query = search.toString();
        if (query) path += `?${query}`;
        return runtime.request<unknown>(path);
      },
    },
  };
}

function setQueryParam(search: URLSearchParams, key: string, value: unknown) {
  if (value === undefined || value === null || value === '') return;
  search.set(key, String(value));
}
