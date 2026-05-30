import type { CurrentUser } from '@/lib/types';

export function ProfileHero({ me }: { me: CurrentUser | null }) {
  return (
    <section className="hero-card">
      <div className="hero-content">
        <div className="hero-topline">
          <span className="hero-kicker">个人账本</span>
          <span className="pill">已登录</span>
        </div>
        <h3 className="hero-amount">{me?.name || '好好用户'}</h3>
        <div className="hero-metrics">
          <div className="hero-metric">
            <span>用户 ID</span>
            <strong>{me?.id || '-'}</strong>
          </div>
          <div className="hero-metric">
            <span>登录方式</span>
            <strong>账号密码</strong>
          </div>
        </div>
      </div>
    </section>
  );
}
