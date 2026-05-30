import type { Summary } from '@/lib/types';
import { formatMoney } from '@/lib/format';

export function PeriodCompare({ summary }: { summary: Summary | null }) {
  return (
    <section className="card">
      <div className="hero-topline" style={{ marginBottom: 16 }}>
        <div>
          <span className="eyebrow">Compare</span>
          <h3>时间段对比</h3>
        </div>
      </div>
      <div className="grid two">
        <div className="panel">
          <span className="muted">当前周期</span>
          <h3>
            {formatMoney(summary?.periodCompare?.current?.income)} /{' '}
            {formatMoney(summary?.periodCompare?.current?.expense)}
          </h3>
        </div>
        <div className="panel">
          <span className="muted">上一周期</span>
          <h3>
            {formatMoney(summary?.periodCompare?.previous?.income)} /{' '}
            {formatMoney(summary?.periodCompare?.previous?.expense)}
          </h3>
        </div>
      </div>
    </section>
  );
}
