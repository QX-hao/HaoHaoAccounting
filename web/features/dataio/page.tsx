'use client';

import { FormEvent, useEffect, useState } from 'react';
import { PageFrame } from '@/components/PageFrame';
import type { ImportJob, ImportPreview } from '@/lib/types';
import { ExportFormat, exportTransactions, getImportJob, importTransactions, listImportJobs, previewImport } from './api';
import { ExportPanel } from './components/ExportPanel';
import { ImportPanel } from './components/ImportPanel';

export default function DataIOFeaturePage() {
  const [format, setFormat] = useState<ExportFormat>('csv');
  const [file, setFile] = useState<File | null>(null);
  const [preview, setPreview] = useState<ImportPreview | null>(null);
  const [jobs, setJobs] = useState<ImportJob[]>([]);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    loadJobs();
  }, []);

  useEffect(() => {
    const running = jobs.filter((job) => job.status === 'queued' || job.status === 'running');
    if (running.length === 0) return;

    const timer = window.setInterval(async () => {
      const nextJobs = await Promise.all(running.map((job) => getImportJob(job.id).catch(() => job)));
      setJobs((current) => current.map((job) => nextJobs.find((next) => next.id === job.id) || job));
    }, 1000);
    return () => window.clearInterval(timer);
  }, [jobs]);

  async function loadJobs() {
    try {
      setJobs(await listImportJobs());
    } catch {
      setJobs([]);
    }
  }

  async function runExport() {
    setError('');
    setNotice('');
    try {
      setBusy(true);
      const { blob, filename } = await exportTransactions(format);
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = filename || `transactions.${format}`;
      a.click();
      URL.revokeObjectURL(url);
      setNotice('导出成功');
    } catch (err) {
      setError(err instanceof Error ? err.message : '导出失败');
    } finally {
      setBusy(false);
    }
  }

  function downloadTemplate() {
    const rows = [
      ['occurred_at', 'type', 'amount', 'category', 'account', 'note', 'tags', 'source'],
      ['2026-06-01T12:30:00+08:00', 'expense', '35.50', '餐饮', '微信', '午饭', '工作日,外卖', 'import'],
    ];
    const csv = rows.map((row) => row.map((cell) => `"${cell.replaceAll('"', '""')}"`).join(',')).join('\n');
    const blob = new Blob([`\uFEFF${csv}`], { type: 'text/csv;charset=utf-8' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = 'haohao_import_template.csv';
    a.click();
    URL.revokeObjectURL(url);
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
      setBusy(true);
      const formData = new FormData();
      formData.append('file', file);
      const job = await importTransactions(formData);
      setJobs((current) => [job, ...current.filter((item) => item.id !== job.id)].slice(0, 20));
      setNotice(`导入任务已创建：${job.filename}`);
    } catch (err) {
      setError(err instanceof Error ? err.message : '导入失败');
    } finally {
      setBusy(false);
    }
  }

  async function runPreview() {
    setError('');
    setNotice('');
    setPreview(null);
    if (!file) {
      setError('请选择文件');
      return;
    }

    try {
      setBusy(true);
      const formData = new FormData();
      formData.append('file', file);
      const resp = await previewImport(formData);
      setPreview(resp);
      setNotice(`预览完成：有效 ${resp.validRows} 条，重复风险 ${resp.duplicateRows} 条，失败 ${resp.failedRows} 条`);
    } catch (err) {
      setError(err instanceof Error ? err.message : '预览失败');
    } finally {
      setBusy(false);
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
        <ExportPanel format={format} disabled={busy} onFormatChange={setFormat} onExport={runExport} />
        <ImportPanel
          file={file}
          disabled={busy}
          preview={preview}
          onDownloadTemplate={downloadTemplate}
          onFileChange={(nextFile) => {
            setFile(nextFile);
            setPreview(null);
          }}
          onPreview={runPreview}
          onImport={runImport}
        />
      </div>
      <section className="panel grid">
        <div className="hero-topline">
          <div>
            <span className="eyebrow">Import Jobs</span>
            <h3>导入任务</h3>
          </div>
          <button className="ghost" type="button" onClick={loadJobs}>
            刷新
          </button>
        </div>
        {jobs.length === 0 ? <div className="muted">暂无导入任务。</div> : null}
        {jobs.map((job) => (
          <div className="notice" key={job.id}>
            <strong>{job.filename}</strong> · {job.status} · 总计 {job.total} / 成功 {job.success} / 跳过 {job.skipped} / 失败 {job.failed}
            {job.errors.length > 0 ? <pre className="error" style={{ whiteSpace: 'pre-wrap' }}>{job.errors.slice(0, 5).join('\n')}</pre> : null}
          </div>
        ))}
      </section>
    </PageFrame>
  );
}
