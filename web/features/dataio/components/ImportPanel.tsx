'use client';

import type { FormEvent } from 'react';
import type { ImportPreview } from '@/lib/types';

type Props = {
  file: File | null;
  disabled: boolean;
  preview: ImportPreview | null;
  onDownloadTemplate: () => void;
  onFileChange: (file: File | null) => void;
  onPreview: () => void;
  onImport: (event: FormEvent) => void;
};

export function ImportPanel({ file, disabled, preview, onDownloadTemplate, onFileChange, onPreview, onImport }: Props) {
  return (
    <form className="card grid" onSubmit={onImport}>
      <div>
        <span className="eyebrow">Import</span>
        <h3>导入数据</h3>
      </div>
      <div className="panel">
        <span className="eyebrow">Fields</span>
        <p className="muted">occurred_at, type, amount, category, account, note, tags, source</p>
      </div>
      <input type="file" accept=".csv,.xlsx" disabled={disabled} onChange={(e) => onFileChange(e.target.files?.[0] || null)} />
      {file ? (
        <div className="notice">
          待导入：{file.name} · {formatFileSize(file.size)}
        </div>
      ) : null}
      <div className="toolbar">
        <button className="ghost" type="button" disabled={disabled} onClick={onDownloadTemplate}>
          下载模板
        </button>
        <button className="secondary" type="button" disabled={disabled || !file} onClick={onPreview}>
          预览校验
        </button>
        <button className="secondary" type="submit" disabled={disabled || !file}>
          {disabled ? '处理中...' : '开始导入'}
        </button>
      </div>
      {preview ? (
        <div className="panel grid">
          <div className="hero-topline">
            <div>
              <span className="eyebrow">Preview</span>
              <h3>导入预览</h3>
            </div>
            <span className="badge">
              有效 {preview.validRows} / 重复 {preview.duplicateRows} / 失败 {preview.failedRows}
            </span>
          </div>
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>行</th>
                  <th>时间</th>
                  <th>类型</th>
                  <th>金额</th>
                  <th>分类</th>
                  <th>账户</th>
                  <th>备注</th>
                  <th>状态</th>
                </tr>
              </thead>
              <tbody>
                {preview.rows.map((row) => (
                  <tr key={row.line}>
                    <td>{row.line}</td>
                    <td>{row.occurredAt || '-'}</td>
                    <td>{row.type || '-'}</td>
                    <td>{row.amount || '-'}</td>
                    <td>{row.category || '-'}</td>
                    <td>{row.account || '-'}</td>
                    <td>{row.note || '-'}</td>
                    <td>{row.valid ? (row.duplicate ? row.duplicateReason || '重复风险' : '可导入') : row.error || '错误'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          {preview.truncated ? <div className="muted">只展示前 {preview.rows.length} 行，总计 {preview.totalRows} 行。</div> : null}
        </div>
      ) : null}
      <div className="muted">
        单个文件最多 {formatFileSize(preview?.maxFileBytes || 5 * 1024 * 1024)}、最多 {preview?.maxRows || 5000} 行；重复记录会在导入时自动跳过。
      </div>
    </form>
  );
}

function formatFileSize(size: number) {
  if (size < 1024) return `${size} B`;
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)} KB`;
  return `${(size / 1024 / 1024).toFixed(1)} MB`;
}
