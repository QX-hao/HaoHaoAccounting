# Accounts Feature

账户模块负责 Web 端账户列表和新增账户表单。

## 文件说明

- `page.tsx`: 页面级状态和布局。
- `api.ts`: 账户相关后端请求。
- `components/AccountForm.tsx`: 新增账户表单。
- `components/AccountGrid.tsx`: 账户卡片列表。

交易产生的余额变化由后端交易模块维护，Web 端只展示后端返回的账户余额。
