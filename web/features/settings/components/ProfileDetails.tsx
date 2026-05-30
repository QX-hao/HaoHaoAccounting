import type { CurrentUser } from '@/lib/types';

export function ProfileDetails({ me }: { me: CurrentUser | null }) {
  return (
    <section className="card grid">
      <div>
        <span className="eyebrow">Profile</span>
        <h3>账号信息</h3>
      </div>
      <div className="list">
        <div className="list-row">
          <strong>用户名</strong>
          <span className="muted">{me?.username || '-'}</span>
        </div>
        <div className="list-row">
          <strong>登录方式</strong>
          <span className="muted">账号密码</span>
        </div>
        <div className="list-row">
          <strong>第三方登录</strong>
          <span className="muted">后续接入</span>
        </div>
      </div>
    </section>
  );
}
