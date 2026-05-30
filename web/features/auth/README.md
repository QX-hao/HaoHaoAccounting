# Auth Feature

登录模块负责 Web 端登录页和登录态初始化。

## 文件说明

- `page.tsx`: 登录页状态和跳转逻辑。
- `api.ts`: 登录和当前用户校验请求。
- `components/LoginForm.tsx`: 登录表单。
- `components/LoginHero.tsx`: 登录页左侧介绍区域。

浏览器 token 的读写在 `web/shared/auth` 中维护，避免多个地方直接操作 localStorage key。
