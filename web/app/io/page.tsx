'use client';

import { FormEvent, useState } from 'react';
import { PageFrame } from '@/components/PageFrame';
import { upload } from '@/lib/api';
import { getToken } from '@/lib/auth';
import { API_BASE } from '@/lib/config';

export default function IOPage() {
  const [format, setFormat] = useState<'csv' | 'xlsx'>('csv');
  const [file, setFile] = useState<File | null>(null);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');

  async function runExport() {
    setError('');
    setNotice('');
    try {
      const token = getToken();
      const resp = await fetch(`${API_BASE}/io/export?format=${format}`, {
        headers: {
          Authorization: `Bearer ${token}`,
        },
      });
      if (!resp.ok) {
        const text = await resp.text();
        throw new Error(text || '导出失败');
      }
      const blob = await resp.blob();
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `transactions.${format}`;
      a.click();
      URL.revokeObjectURL(url);
      setNotice('导出成功');
    } catch (err) {
      setError(err instanceof Error ? err.message : '导出失败');
    }
  }

  async function runImport(e: FormEvent) {
    e.preventDefault();
    setError('');
    setNotice('');
    if (!file) {
      setError('请选择文件');
      return;
    }

    try {
      const formData = new FormData();
      formData.append('file', file);
      const resp = await upload<{ success: number; failed: number; errors: string[] }>('/io/import', formData);
      setNotice(`导入完成：成功 ${resp.success} 条，失败 ${resp.failed} 条`);
      if (resp.errors.length > 0) {
        setError(resp.errors.slice(0, 5).join('\n'));
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : '导入失败');
    }
  }

  return (
    <PageFrame title="导入导出" subtitle="支持 CSV / Excel 导入和导出">
      {error ? <pre className="error" style={{ whiteSpace: 'pre-wrap' }}>{error}</pre> : null}
      {notice ? <div className="success">{notice}</div> : null}

      <div className="grid two">
        <div className="card grid">
          <h3 style={{ margin: 0 }}>导出数据</h3>
          <select value={format} onChange={(e) => setFormat(e.target.value as 'csv' | 'xlsx')}>
            <option value="csv">CSV</option>
            <option value="xlsx">Excel (XLSX)</option>
          </select>
          <button className="primary" type="button" onClick={runExport}>
            立即导出
          </button>
        </div>

        <form className="card grid" onSubmit={runImport}>
          <h3 style={{ margin: 0 }}>导入数据</h3>
          <input
            type="file"
            accept=".csv,.xlsx"
            onChange={(e) => setFile(e.target.files?.[0] || null)}
          />
          <button className="secondary" type="submit">
            开始导入
          </button>
        </form>
      </div>
    </PageFrame>
  );
}
