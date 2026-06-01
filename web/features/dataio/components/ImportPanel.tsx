'use client';

import type { FormEvent } from 'react';

type Props = {
  file: File | null;
  disabled: boolean;
  onDownloadTemplate: () => void;
  onFileChange: (file: File | null) => void;
  onImport: (event: FormEvent) => void;
};

export function ImportPanel({ file, disabled, onDownloadTemplate, onFileChange, onImport }: Props) {
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
        <button className="secondary" type="submit" disabled={disabled || !file}>
          {disabled ? '处理中...' : '开始导入'}
        </button>
      </div>
      <div className="muted">
        导入前会按行校验字段；同一文件重复导入暂不会自动去重，请先在表格中确认。
      </div>
    </form>
  );
}

function formatFileSize(size: number) {
  if (size < 1024) return `${size} B`;
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)} KB`;
  return `${(size / 1024 / 1024).toFixed(1)} MB`;
}
