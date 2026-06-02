'use client';

import { FormEvent, useEffect, useState } from 'react';
import { PageFrame } from '@/components/PageFrame';
import type { Category, TransactionType } from '@/lib/types';
import { createCategory, deleteCategory, listCategories, updateCategory } from './api';
import { CategoryForm } from './components/CategoryForm';
import { CategoryGrid } from './components/CategoryGrid';

export default function CategoriesFeaturePage() {
  const [items, setItems] = useState<Category[]>([]);
  const [name, setName] = useState('');
  const [type, setType] = useState<TransactionType>('expense');
  const [editing, setEditing] = useState<Category | null>(null);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const [busy, setBusy] = useState(false);

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
    setNotice('');
    try {
      setBusy(true);
      if (editing) {
        await updateCategory(editing.id, { name, type });
        setNotice('分类已更新');
      } else {
        await createCategory({ name, type });
        setNotice('分类已创建');
      }
      resetForm();
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : editing ? '更新失败' : '创建失败');
    } finally {
      setBusy(false);
    }
  }

  function startEdit(category: Category) {
    if (category.isSystem) return;
    setEditing(category);
    setName(category.name);
    setType(category.type);
    setError('');
    setNotice('');
  }

  async function remove(category: Category) {
    if (category.isSystem) return;
    const ok = window.confirm(`确定删除分类「${category.name}」吗？已有账单使用的分类不能删除。`);
    if (!ok) return;
    setError('');
    setNotice('');
    try {
      setBusy(true);
      await deleteCategory(category.id);
      if (editing?.id === category.id) resetForm();
      setNotice('分类已删除');
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : '删除失败');
    } finally {
      setBusy(false);
    }
  }

  function resetForm() {
    setEditing(null);
    setName('');
    setType('expense');
  }

  return (
    <PageFrame title="分类" subtitle="维护预置分类和自定义分类，让账单统计更干净。">
      {error ? <div className="error">{error}</div> : null}
      {notice ? <div className="success">{notice}</div> : null}
      <CategoryForm name={name} type={type} submitLabel={editing ? '保存修改' : '新增分类'} onNameChange={setName} onTypeChange={setType} onSubmit={submit} />
      {editing ? (
        <div className="toolbar">
          <span className="badge">正在编辑：{editing.name}</span>
          <button className="ghost" type="button" disabled={busy} onClick={resetForm}>
            取消编辑
          </button>
        </div>
      ) : null}
      <CategoryGrid categories={items} disabled={busy} onEdit={startEdit} onDelete={remove} />
    </PageFrame>
  );
}
