'use client';

import { FormEvent, useEffect, useState } from 'react';
import { request } from '@/lib/api';
import { PageFrame } from '@/components/PageFrame';
import { Category } from '@/lib/types';

export default function CategoriesPage() {
  const [items, setItems] = useState<Category[]>([]);
  const [name, setName] = useState('');
  const [type, setType] = useState<'expense' | 'income'>('expense');
  const [error, setError] = useState('');

  async function load() {
    setError('');
    try {
      const rows = await request<Category[]>('/categories');
      setItems(rows);
    } catch (err) {
      setError(err instanceof Error ? err.message : '加载失败');
    }
  }

  useEffect(() => {
    load();
  }, []);

  async function createCategory(e: FormEvent) {
    e.preventDefault();
    setError('');
    try {
      await request('/categories', {
        method: 'POST',
        body: JSON.stringify({ name, type }),
      });
      setName('');
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : '创建失败');
    }
  }

  return (
    <PageFrame title="分类管理" subtitle="维护预置分类和自定义分类">
      {error ? <div className="error">{error}</div> : null}
      <form className="card toolbar" onSubmit={createCategory}>
        <input value={name} onChange={(e) => setName(e.target.value)} placeholder="分类名称" required />
        <select value={type} onChange={(e) => setType(e.target.value as 'expense' | 'income')}>
          <option value="expense">支出</option>
          <option value="income">收入</option>
        </select>
        <button className="primary" type="submit">
          新增分类
        </button>
      </form>

      <div className="card">
        <table>
          <thead>
            <tr>
              <th>名称</th>
              <th>类型</th>
              <th>来源</th>
            </tr>
          </thead>
          <tbody>
            {items.map((item) => (
              <tr key={item.id}>
                <td>{item.name}</td>
                <td>{item.type === 'income' ? '收入' : '支出'}</td>
                <td>{item.isSystem ? '系统' : '自定义'}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </PageFrame>
  );
}
