# Overview Feature

首页模块负责 Web 端登录后的概览。

## 文件说明

- `page.tsx`: 首页数据加载和页面布局。
- `api.ts`: 汇总和最近账单请求。
- `components/HeroSummary.tsx`: 本期结余主卡片。
- `components/QuickActions.tsx`: 快捷入口。
- `components/RecentTransactions.tsx`: 最近账单列表。

首页只聚合其他模块的数据，不直接修改账单、账户或分类。
