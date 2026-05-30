'use client';

import { FormEvent, useState } from 'react';
import { PageFrame } from '@/components/PageFrame';
import { ExportFormat, exportTransactions, importTransactions } from './api';
import { ExportPanel } from './components/ExportPanel';
import { ImportPanel } from './components/ImportPanel';

export default function DataIOFeaturePage() {
  const [format, setFormat] = useState<ExportFormat>('csv');
  const [file, setFile] = useState<File | null>(null);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');

  async function runExport() {
    setError('');
    setNotice('');
    try {
      const blob = await exportTransactions(format);
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
      const resp = await importTransactions(formData);
      setNotice(`导入完成：成功 ${resp.success} 条，失败 ${resp.failed} 条`);
      if (resp.errors.length > 0) {
        setError(resp.errors.slice(0, 5).join('\n'));
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : '导入失败');
    }
  }

  return (
    <PageFrame title="导入导出" subtitle="支持 CSV / Excel 导入和导出，方便长期整理财务数据。">
      {error ? (
        <pre className="error" style={{ whiteSpace: 'pre-wrap' }}>
          {error}
        </pre>
      ) : null}
      {notice ? <div className="success">{notice}</div> : null}

      <div className="grid two">
        <ExportPanel format={format} onFormatChange={setFormat} onExport={runExport} />
        <ImportPanel onFileChange={setFile} onImport={runImport} />
      </div>
    </PageFrame>
  );
}
