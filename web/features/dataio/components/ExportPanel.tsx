'use client';

import type { ExportFormat } from '../api';

type Props = {
  format: ExportFormat;
  onFormatChange: (value: ExportFormat) => void;
  onExport: () => void;
};

export function ExportPanel({ format, onFormatChange, onExport }: Props) {
  return (
    <div className="card grid">
      <div>
        <span className="eyebrow">Export</span>
        <h3>导出数据</h3>
      </div>
      <select value={format} onChange={(e) => onFormatChange(e.target.value as ExportFormat)}>
        <option value="csv">CSV</option>
        <option value="xlsx">Excel (XLSX)</option>
      </select>
      <button className="primary" type="button" onClick={onExport}>
        立即导出
      </button>
    </div>
  );
}
