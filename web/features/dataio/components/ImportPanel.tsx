'use client';

import type { FormEvent } from 'react';

type Props = {
  onFileChange: (file: File | null) => void;
  onImport: (event: FormEvent) => void;
};

export function ImportPanel({ onFileChange, onImport }: Props) {
  return (
    <form className="card grid" onSubmit={onImport}>
      <div>
        <span className="eyebrow">Import</span>
        <h3>导入数据</h3>
      </div>
      <input type="file" accept=".csv,.xlsx" onChange={(e) => onFileChange(e.target.files?.[0] || null)} />
      <button className="secondary" type="submit">
        开始导入
      </button>
    </form>
  );
}
