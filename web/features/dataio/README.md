# Data IO Feature

导入导出模块负责 Web 端 CSV/XLSX 文件导入和导出。

## 文件说明

- `page.tsx`: 页面状态、上传和下载流程。
- `api.ts`: 导入导出接口。
- `components/ExportPanel.tsx`: 导出格式选择和下载按钮。
- `components/ImportPanel.tsx`: 文件选择和上传表单。

导入格式需要和后端 `dataio` 模块 README 中的字段保持一致。
