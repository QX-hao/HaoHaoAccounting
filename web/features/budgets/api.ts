import { api } from '@/shared/api';
import type { Budget, BudgetRequest } from '@/lib/types';

export function listBudgets(month?: string) {
  return api.budgets.getBudgets({ month }) as Promise<Budget[]>;
}

export function createBudget(payload: BudgetRequest) {
  return api.budgets.postBudgets(payload);
}

export function updateBudget(id: number, payload: BudgetRequest) {
  return api.budgets.putBudgetsById({ id }, payload);
}

export function deleteBudget(id: number) {
  return api.budgets.deleteBudgetsById({ id });
}
