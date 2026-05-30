# Transactions Feature

记账模块负责 Web 端流水录入、AI 辅助填表和账单列表。

## 文件说明

- `page.tsx`: 页面状态编排和提交流程。
- `api.ts`: 交易、账户、分类和 AI 解析请求。
- `components/TransactionForm.tsx`: 手动记账表单。
- `components/AIParsePanel.tsx`: AI 对话记账输入和解析预览。
- `components/TransactionList.tsx`: 账单列表。

## 维护注意

前端只负责收集用户确认后的输入。余额调整、分类账户可访问性校验和事务一致性由后端交易模块保证。
