export type Account = {
  id: number;
  userId: number;
  name: string;
  type: string;
  balance: number;
};

export type Category = {
  id: number;
  name: string;
  type: 'income' | 'expense';
  isSystem: boolean;
};

export type Transaction = {
  id: number;
  type: 'income' | 'expense';
  amount: number;
  note: string;
  tags: string;
  occurredAt: string;
  category: Category;
  account: Account;
};
