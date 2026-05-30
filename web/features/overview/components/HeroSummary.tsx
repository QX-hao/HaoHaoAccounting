import type { Summary } from '@/lib/types';
import { formatMoney } from '@/lib/format';

export function HeroSummary({ summary }: { summary: Summary | null }) {
  return (
    <section className="hero-card">
      <div className="hero-content">
        <div className="hero-topline">
          <span className="hero-kicker">本期结余</span>
          <span className="pill">自动同步</span>
        </div>
        <h3 className="hero-amount">{formatMoney(summary?.balance)}</h3>
        <div className="hero-metrics">
          <div className="hero-metric">
            <span>收入</span>
            <strong>{formatMoney(summary?.income)}</strong>
          </div>
          <div className="hero-metric">
            <span>支出</span>
            <strong>{formatMoney(summary?.expense)}</strong>
          </div>
        </div>
      </div>
    </section>
  );
}
