# Reports Feature

统计模块负责 Web 端报表展示。

## 文件说明

- `page.tsx`: 加载汇总数据并组织页面。
- `api.ts`: 报表接口请求。
- `components/SummaryCards.tsx`: 收入、支出、结余卡片。
- `components/BarBreakdown.tsx`: 分类和账户横向条形图。
- `components/MonthlyTrend.tsx`: 月度收支趋势。
- `components/PeriodCompare.tsx`: 当前周期和上一周期对比。

报表只读，不在前端计算或修改账单数据源。
