'use client';

import { FormEvent, useEffect, useState } from 'react';
import { PageFrame } from '@/components/PageFrame';
import type { Category, TransactionType } from '@/lib/types';
import { createCategory, listCategories } from './api';
import { CategoryForm } from './components/CategoryForm';
import { CategoryGrid } from './components/CategoryGrid';

export default function CategoriesFeaturePage() {
  const [items, setItems] = useState<Category[]>([]);
  const [name, setName] = useState('');
  const [type, setType] = useState<TransactionType>('expense');
  const [error, setError] = useState('');

  async function load() {
    setError('');
    try {
      setItems(await listCategories());
    } catch (err) {
      setError(err instanceof Error ? err.message : '加载失败');
    }
  }

  useEffect(() => {
    load();
  }, []);

  async function submit(e: FormEvent) {
    e.preventDefault();
    setError('');
    try {
      await createCategory({ name, type });
      setName('');
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : '创建失败');
    }
  }

  return (
    <PageFrame title="分类" subtitle="维护预置分类和自定义分类，让账单统计更干净。">
      {error ? <div className="error">{error}</div> : null}
      <CategoryForm name={name} type={type} onNameChange={setName} onTypeChange={setType} onSubmit={submit} />
      <CategoryGrid categories={items} />
    </PageFrame>
  );
}
