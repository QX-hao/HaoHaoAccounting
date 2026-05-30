# Settings Feature

我的/设置模块负责 Web 端当前用户信息展示。

## 文件说明

- `page.tsx`: 加载用户信息并组织页面。
- `api.ts`: 当前用户请求。
- `components/ProfileHero.tsx`: 用户账本主卡片。
- `components/ProfileDetails.tsx`: 账号信息列表。

账号修改、第三方登录等能力接入后，应优先在这个模块内扩展。
