# Categories Feature

分类模块负责 Web 端分类列表和新增分类表单。

## 文件说明

- `page.tsx`: 页面状态和布局。
- `api.ts`: 分类相关后端请求。
- `components/CategoryForm.tsx`: 新增分类表单。
- `components/CategoryGrid.tsx`: 分类卡片列表。

系统分类由后端初始化，Web 端只允许通过接口创建自定义分类。
