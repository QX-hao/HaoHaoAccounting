import type { SummaryTableRow } from '@/lib/types';
import { formatMoney } from '@/lib/format';

export function SummaryTables({ daily, monthly }: { daily: SummaryTableRow[]; monthly: SummaryTableRow[] }) {
  return (
    <div className="grid two">
      <SummaryTable title="日汇总" rows={daily.slice(-8)} />
      <SummaryTable title="月汇总" rows={monthly.slice(-8)} />
    </div>
  );
}

function SummaryTable({ title, rows }: { title: string; rows: SummaryTableRow[] }) {
  return (
    <section className="panel">
      <div className="hero-topline" style={{ marginBottom: 16 }}>
        <div>
          <span className="eyebrow">Summary</span>
          <h3>{title}</h3>
        </div>
      </div>
      <div className="table-wrap">
        <table>
          <thead>
            <tr>
              <th>周期</th>
              <th>收入</th>
              <th>支出</th>
              <th>结余</th>
              <th>笔数</th>
            </tr>
          </thead>
          <tbody>
            {rows.length ? null : (
              <tr>
                <td colSpan={5}>暂无汇总数据</td>
              </tr>
            )}
            {rows.map((row) => (
              <tr key={row.period}>
                <td>{row.period}</td>
                <td>{formatMoney(row.income)}</td>
                <td>{formatMoney(row.expense)}</td>
                <td>{formatMoney(row.balance)}</td>
                <td>{row.txCount}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </section>
  );
}
