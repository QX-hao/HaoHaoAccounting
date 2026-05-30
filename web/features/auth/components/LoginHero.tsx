const featureItems = [
  ['3s', '快速记账'],
  ['AI', '对话解析'],
  ['CSV', '数据导出'],
  ['Safe', '本地服务'],
];

export function LoginHero() {
  return (
    <section className="login-hero">
      <span className="eyebrow">HaoHao Accounting</span>
      <h1>三秒记一笔，月底不用补账。</h1>
      <p>好好记账专注轻量、清楚、快速完成的体验：先看结余，再快速记账，最后用统计复盘。</p>

      <div className="feature-strip">
        {featureItems.map(([value, label]) => (
          <div className="card stat-card" key={label}>
            <span className="label">{label}</span>
            <span className="value">{value}</span>
          </div>
        ))}
      </div>
    </section>
  );
}
