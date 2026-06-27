import { expect, type Page, type Route, test } from '@playwright/test';

type TxType = 'income' | 'expense';
type TrendGranularity = 'day' | 'week' | 'month';

type Account = {
  id: number;
  userId: number;
  name: string;
  type: string;
  balance: number;
};

type Category = {
  id: number;
  userId?: number;
  name: string;
  type: TxType;
  isSystem: boolean;
};

type Budget = {
  id: number;
  userId: number;
  month: string;
  categoryId: number;
  amount: number;
};

type Transaction = {
  id: number;
  userId: number;
  type: TxType;
  amount: number;
  categoryId: number;
  accountId: number;
  note: string;
  tags: string;
  source: string;
  occurredAt: string;
  category: Category;
  account: Account;
};

test.beforeEach(async ({ page }) => {
  const api = new MockApi();
  await api.install(page);
});

test('login and manage transactions with filters, pagination, and import preview', async ({ page }) => {
  await page.goto('/login');
  await page.getByPlaceholder('密码').fill('haohao123');
  await page.getByRole('button', { name: '进入账本' }).click();
  await expect(page).toHaveURL(/\/overview/);
  await expect(page.getByText('今天也好好记账')).toBeVisible();

  await page.getByRole('link', { name: '记一笔', exact: true }).click();
  await expect(page.getByRole('heading', { name: '手动记账' })).toBeVisible();
  await page.locator('input[type="number"]').fill('35.5');
  await page.getByRole('button', { name: '餐饮' }).click();
  await page.getByPlaceholder('备注', { exact: true }).fill('午饭 E2E');
  await page.getByPlaceholder('标签，用逗号分隔').fill('工作日');
  await page.getByRole('button', { name: '保存账单' }).click();
  await expect(page.getByText('账单已保存')).toBeVisible();
  await expect(page.getByText('午饭 E2E')).toBeVisible();

  await page.getByRole('button', { name: '编辑' }).first().click();
  await page.getByPlaceholder('备注', { exact: true }).fill('晚饭 E2E');
  await page.getByRole('button', { name: '保存修改' }).click();
  await expect(page.getByText('账单已更新')).toBeVisible();
  await expect(page.getByText('晚饭 E2E')).toBeVisible();

  await page.getByPlaceholder('搜索备注或标签').fill('晚饭');
  await page.getByRole('button', { name: '应用筛选' }).click();
  await expect(page.getByText('晚饭 E2E')).toBeVisible();
  await page.getByRole('button', { name: '重置' }).click();
  await page.getByRole('combobox').last().selectOption('10');
  await page.getByRole('button', { name: '下一页' }).click();
  await expect(page.getByText('2 / 2')).toBeVisible();
  await page.getByRole('button', { name: '上一页' }).click();
  await expect(page.getByText('1 / 2')).toBeVisible();

  page.once('dialog', (dialog) => dialog.accept());
  await page.getByRole('button', { name: '删除' }).first().click();
  await expect(page.getByText('账单已删除')).toBeVisible();
  await expect(page.getByText('晚饭 E2E')).toHaveCount(0);

  await page.getByRole('link', { name: /导入导出/ }).click();
  await page.setInputFiles('input[type="file"]', {
    name: 'import.csv',
    mimeType: 'text/csv',
    buffer: Buffer.from('occurred_at,type,amount,category,account,note,tags\n2026-06-01T12:30:00+08:00,expense,12.5,餐饮,现金,导入测试,\n'),
  });
  await page.getByRole('button', { name: '预览校验' }).click();
  await expect(page.getByText('预览完成：有效 1 条，重复风险 1 条，失败 0 条')).toBeVisible();
  await expect(page.getByText('账本中已存在相同记录')).toBeVisible();
  await page.getByRole('button', { name: '开始导入' }).click();
  await expect(page.getByText('导入任务已创建：import.csv')).toBeVisible();
  await expect(page.getByRole('heading', { name: '导入任务' })).toBeVisible();
  await expect(page.getByText(/import\.csv.*completed/)).toBeVisible();
});

test('edit and delete accounts and categories', async ({ page }) => {
  await login(page);

  await page.getByRole('link', { name: /账户/ }).click();
  await page.getByPlaceholder('账户名称').fill('测试账户');
  await page.getByRole('button', { name: '新增账户' }).click();
  await expect(page.getByText('账户已创建')).toBeVisible();
  await page.locator('.card', { hasText: '测试账户' }).getByRole('button', { name: '编辑' }).click();
  await page.getByPlaceholder('账户名称').fill('测试账户改');
  await page.getByRole('button', { name: '保存修改' }).click();
  await expect(page.getByText('账户已更新')).toBeVisible();
  await expect(page.getByText('测试账户改')).toBeVisible();
  page.once('dialog', (dialog) => dialog.accept());
  await page.locator('.card', { hasText: '测试账户改' }).getByRole('button', { name: '删除' }).click();
  await expect(page.getByText('账户已删除')).toBeVisible();
  await expect(page.getByText('测试账户改')).toHaveCount(0);

  await page.getByRole('link', { name: /分类/ }).click();
  await page.getByPlaceholder('分类名称').fill('测试分类');
  await page.getByRole('button', { name: '新增分类' }).click();
  await expect(page.getByText('分类已创建')).toBeVisible();
  await page.locator('.card', { hasText: '测试分类' }).getByRole('button', { name: '编辑' }).click();
  await page.getByPlaceholder('分类名称').fill('测试分类改');
  await page.getByRole('button', { name: '保存修改' }).click();
  await expect(page.getByText('分类已更新')).toBeVisible();
  await expect(page.getByText('测试分类改')).toBeVisible();
  page.once('dialog', (dialog) => dialog.accept());
  await page.locator('.card', { hasText: '测试分类改' }).getByRole('button', { name: '删除' }).click();
  await expect(page.getByText('分类已删除')).toBeVisible();
  await expect(page.getByText('测试分类改')).toHaveCount(0);
});

async function login(page: Page) {
  await page.goto('/login');
  await page.getByPlaceholder('密码').fill('haohao123');
  await page.getByRole('button', { name: '进入账本' }).click();
  await expect(page).toHaveURL(/\/overview/);
}

class MockApi {
  private nextAccountId = 10;
  private nextCategoryId = 10;
  private nextTransactionId = 10;
  private nextBudgetId = 10;

  private accounts: Account[] = [
    { id: 1, userId: 1, name: '现金', type: 'cash', balance: 0 },
    { id: 2, userId: 1, name: '银行卡', type: 'bank', balance: 0 },
  ];

  private categories: Category[] = [
    { id: 1, name: '餐饮', type: 'expense', isSystem: true },
    { id: 2, name: '工资', type: 'income', isSystem: true },
  ];

  private transactions: Transaction[] = [
    this.tx({ id: 1, note: '早餐', amount: 12.5, categoryId: 1, accountId: 1 }),
    this.tx({ id: 2, note: '咖啡', amount: 22, categoryId: 1, accountId: 1 }),
    this.tx({ id: 3, note: '午餐', amount: 30, categoryId: 1, accountId: 1 }),
    this.tx({ id: 4, note: '晚餐', amount: 45, categoryId: 1, accountId: 1 }),
    this.tx({ id: 5, note: '零食', amount: 9, categoryId: 1, accountId: 1 }),
    this.tx({ id: 6, note: '饮料', amount: 6, categoryId: 1, accountId: 1 }),
    this.tx({ id: 7, note: '外卖', amount: 28, categoryId: 1, accountId: 1 }),
    this.tx({ id: 8, note: '水果', amount: 18, categoryId: 1, accountId: 1 }),
    this.tx({ id: 9, note: '早餐2', amount: 11, categoryId: 1, accountId: 1 }),
    this.tx({ id: 10, note: '咖啡2', amount: 20, categoryId: 1, accountId: 1 }),
    this.tx({ id: 11, note: '午餐2', amount: 32, categoryId: 1, accountId: 1 }),
  ];

  private budgets: Budget[] = [
    { id: 1, userId: 1, month: '2026-06', categoryId: 1, amount: 500 },
  ];

  private importJobs = [{
    id: 1,
    filename: '历史导入.csv',
    status: 'completed',
    total: 1,
    success: 1,
    failed: 0,
    skipped: 0,
    errors: [],
    createdAt: '2026-06-01T12:30:00+08:00',
    updatedAt: '2026-06-01T12:30:00+08:00',
  }];

  async install(page: Page) {
    await page.route('http://localhost:8080/api/v1/**', (route) => this.handle(route));
  }

  private async handle(route: Route) {
    const request = route.request();
    const url = new URL(request.url());
    const path = url.pathname.replace('/api/v1', '');
    const method = request.method();

    if (method === 'POST' && path === '/auth/login') {
      return this.json(route, { token: 'test-token', user: { id: 1, name: '管理员', username: 'admin' } });
    }
    if (method === 'GET' && path === '/me') {
      return this.json(route, { id: 1, name: '管理员', username: 'admin' });
    }
    if (method === 'GET' && path === '/reports/summary') {
      const expense = this.transactions.reduce((sum, tx) => tx.type === 'expense' ? sum + tx.amount : sum, 0);
      const trendGranularity = reportTrendGranularity(url.searchParams.get('trend'));
      return this.json(route, {
        income: 1000,
        expense,
        balance: 900,
        byCategory: [{ categoryId: 1, category: '餐饮', amount: 12.5 }],
        byAccount: [{ accountId: 1, account: '现金', amount: 12.5 }],
        monthlyTrend: [{ month: '2026-06', income: 1000, expense: 12.5 }],
        trendGranularity,
        trend: [{ period: '2026-06', income: 1000, expense }],
        categoryTrend: [{ period: '2026-06', categoryId: 1, category: '餐饮', amount: expense }],
        accountBalanceTrend: [{ period: '2026-06', accountId: 1, account: '现金', net: 1000 - expense, balance: 1000 - expense }],
        budgetExecution: [{ budgetId: 1, month: '2026-06', categoryId: 1, category: '餐饮', budget: 500, expense, remaining: 500 - expense, usageRate: expense / 500 }],
        dailySummaries: [{ period: '2026-06-01', income: 1000, expense, balance: 1000 - expense, txCount: this.transactions.length }],
        monthlySummaries: [{ period: '2026-06', income: 1000, expense, balance: 1000 - expense, txCount: this.transactions.length }],
      });
    }
    if (path === '/accounts') return this.accountsRoute(route, method);
    if (path.startsWith('/accounts/')) return this.accountRoute(route, method, Number(path.split('/').at(-1)));
    if (path === '/budgets') return this.budgetsRoute(route, method, url.searchParams);
    if (path.startsWith('/budgets/')) return this.budgetRoute(route, method, Number(path.split('/').at(-1)));
    if (path === '/categories') return this.categoriesRoute(route, method, url.searchParams.get('type'));
    if (path.startsWith('/categories/')) return this.categoryRoute(route, method, Number(path.split('/').at(-1)));
    if (path === '/transactions') return this.transactionsRoute(route, method, url.searchParams);
    if (path.startsWith('/transactions/')) return this.transactionRoute(route, method, Number(path.split('/').at(-1)));
    if (method === 'POST' && path === '/io/import/preview') {
      return this.json(route, {
        filename: 'import.csv',
        size: 100,
        totalRows: 1,
        validRows: 1,
        failedRows: 0,
        duplicateRows: 1,
        maxRows: 5000,
        maxFileBytes: 5242880,
        truncated: false,
        rows: [{
          line: 2,
          occurredAt: '2026-06-01T12:30:00+08:00',
          type: 'expense',
          amount: 12.5,
          category: '餐饮',
          account: '现金',
          note: '导入测试',
          tags: '',
          valid: true,
          duplicate: true,
          duplicateReason: '账本中已存在相同记录',
        }],
      });
    }
    if (method === 'GET' && path === '/io/import/jobs') {
      return this.json(route, this.importJobs);
    }
    if (method === 'POST' && path === '/io/import/jobs') {
      const job = {
        id: this.importJobs.length + 1,
        filename: 'import.csv',
        status: 'completed',
        total: 1,
        success: 1,
        failed: 0,
        skipped: 0,
        errors: [],
        createdAt: '2026-06-01T12:30:00+08:00',
        updatedAt: '2026-06-01T12:30:00+08:00',
      };
      this.importJobs.unshift(job);
      return this.json(route, job, 202);
    }
    if (method === 'GET' && path.startsWith('/io/import/jobs/')) {
      const id = Number(path.split('/').at(-1));
      return this.json(route, this.importJobs.find((job) => job.id === id) || this.importJobs[0]);
    }

    return route.fulfill({ status: 404, json: { error: `unhandled ${method} ${path}` } });
  }

  private accountsRoute(route: Route, method: string) {
    if (method === 'GET') return this.json(route, this.accounts);
    if (method === 'POST') {
      const body = route.request().postDataJSON();
      const account = { id: this.nextAccountId++, userId: 1, name: body.name, type: body.type, balance: body.balance || 0 };
      this.accounts.push(account);
      return this.json(route, account, 201);
    }
    return this.json(route, { error: 'method not allowed' }, 405);
  }

  private accountRoute(route: Route, method: string, id: number) {
    const index = this.accounts.findIndex((item) => item.id === id);
    if (method === 'PUT' && index >= 0) {
      this.accounts[index] = { ...this.accounts[index], ...route.request().postDataJSON() };
      return this.json(route, this.accounts[index]);
    }
    if (method === 'DELETE' && index >= 0) {
      this.accounts.splice(index, 1);
      return this.json(route, { ok: true });
    }
    return this.json(route, { error: 'not found' }, 404);
  }

  private budgetsRoute(route: Route, method: string, params: URLSearchParams) {
    if (method === 'GET') {
      const month = params.get('month') || '';
      return this.json(route, month ? this.budgets.filter((item) => item.month === month) : this.budgets);
    }
    if (method === 'POST') {
      const body = route.request().postDataJSON();
      const budget = { id: this.nextBudgetId++, userId: 1, month: body.month, categoryId: body.categoryId || 0, amount: body.amount || 0 };
      this.budgets.push(budget);
      return this.json(route, budget, 201);
    }
    return this.json(route, { error: 'method not allowed' }, 405);
  }

  private budgetRoute(route: Route, method: string, id: number) {
    const index = this.budgets.findIndex((item) => item.id === id);
    if (method === 'PUT' && index >= 0) {
      this.budgets[index] = { ...this.budgets[index], ...route.request().postDataJSON() };
      return this.json(route, this.budgets[index]);
    }
    if (method === 'DELETE' && index >= 0) {
      this.budgets.splice(index, 1);
      return this.json(route, { ok: true });
    }
    return this.json(route, { error: 'not found' }, 404);
  }

  private categoriesRoute(route: Route, method: string, type: string | null) {
    if (method === 'GET') return this.json(route, type ? this.categories.filter((item) => item.type === type) : this.categories);
    if (method === 'POST') {
      const body = route.request().postDataJSON();
      const category = { id: this.nextCategoryId++, userId: 1, name: body.name, type: body.type, isSystem: false };
      this.categories.push(category);
      return this.json(route, category, 201);
    }
    return this.json(route, { error: 'method not allowed' }, 405);
  }

  private categoryRoute(route: Route, method: string, id: number) {
    const index = this.categories.findIndex((item) => item.id === id);
    if (method === 'PUT' && index >= 0) {
      this.categories[index] = { ...this.categories[index], ...route.request().postDataJSON() };
      return this.json(route, this.categories[index]);
    }
    if (method === 'DELETE' && index >= 0) {
      this.categories.splice(index, 1);
      return this.json(route, { ok: true });
    }
    return this.json(route, { error: 'not found' }, 404);
  }

  private transactionsRoute(route: Route, method: string, params: URLSearchParams) {
    if (method === 'GET') {
      const q = params.get('q')?.trim() || '';
      const page = Number(params.get('page') || 1);
      const pageSize = Number(params.get('pageSize') || 20);
      const filtered = q ? this.transactions.filter((item) => item.note.includes(q) || item.tags.includes(q)) : this.transactions;
      const start = (page - 1) * pageSize;
      return this.json(route, {
        items: filtered.slice(start, start + pageSize),
        pagination: { page, pageSize, total: filtered.length },
      });
    }
    if (method === 'POST') {
      const body = route.request().postDataJSON();
      const tx = this.tx({ ...body, id: this.nextTransactionId++ });
      this.transactions.unshift(tx);
      return this.json(route, tx, 201);
    }
    return this.json(route, { error: 'method not allowed' }, 405);
  }

  private transactionRoute(route: Route, method: string, id: number) {
    const index = this.transactions.findIndex((item) => item.id === id);
    if (method === 'PUT' && index >= 0) {
      this.transactions[index] = this.tx({ ...this.transactions[index], ...route.request().postDataJSON(), id });
      return this.json(route, this.transactions[index]);
    }
    if (method === 'DELETE' && index >= 0) {
      this.transactions.splice(index, 1);
      return this.json(route, { ok: true });
    }
    return this.json(route, { error: 'not found' }, 404);
  }

  private tx(input: Partial<Transaction> & { id: number; note: string; amount: number; categoryId: number; accountId: number }): Transaction {
    const category = this.categories.find((item) => item.id === input.categoryId) || this.categories[0];
    const account = this.accounts.find((item) => item.id === input.accountId) || this.accounts[0];
    const tags = Array.isArray(input.tags) ? input.tags.join(',') : input.tags || '';
    return {
      userId: 1,
      type: category.type,
      source: 'manual',
      occurredAt: '2026-06-01T12:30:00+08:00',
      category,
      account,
      ...input,
      tags,
    };
  }

  private json(route: Route, body: unknown, status = 200) {
    return route.fulfill({ status, contentType: 'application/json', body: JSON.stringify(body) });
  }
}

function reportTrendGranularity(value: string | null): TrendGranularity {
  return value === 'day' || value === 'week' || value === 'month' ? value : 'month';
}
